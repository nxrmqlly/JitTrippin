package main

import (
	"context"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/nxrmqlly/jittrippin/internal/auth"
	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/daemon/api"
	"github.com/nxrmqlly/jittrippin/internal/github"
	"github.com/nxrmqlly/jittrippin/internal/store"
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/logs"
	"github.com/nxrmqlly/jittrippin/pkg/runner"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	connstr := os.Getenv("JTD_POSTGRES_CONNSTR")
	if connstr == "" {
		log.Fatalf("JTD_POSTGRES_CONNSTR is not set.")
	}

	if err := store.Migrate(ctx, connstr); err != nil {
		log.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, connstr)
	if err != nil {
		log.Fatal(err)
	}

	encKey := make([]byte, 32)
	hexKey := os.Getenv("JTD_SIGNING_SECRET")
	if hexKey == "" {
		log.Fatalf("JTD_SIGNING_SECRET must be set. (32 byte hex encoded key)")
	}
	keyb, err := hex.DecodeString(hexKey)
	if err != nil {
		log.Fatalf("JTD_SIGNING_SECRET in wrong format, must be 64 hex chars")
	}
	if len(keyb) != 32 {
		log.Fatalf("JTD_SIGNING_SECRET must be exactly 32 bytes (64 hex chars)")
	}
	encKey = keyb
	st := store.New(pool, encKey)
	as := &artifact.LocalStore{
		Root: ".jt-artifacts",
	}

	ch := &checkout.GitCheckouter{}

	rn, err := runner.NewDockerRunner()
	if err != nil {
		log.Fatal(err)
	}

	bs := logs.NewBroadcaster()

	ls := logs.NewFileStore(as, bs)

	e := engine.NewExecutor(engine.ExecutorConfig{
		MaxParallel: -1,

		Runner:        rn,
		ArtifactStore: as,
		Checkouter:    ch,
		LogsStore:     ls,
	})

	mgr, err := daemon.NewManager(ctx, daemon.ManagerConfig{
		Executor:        e,
		Store:           st,
		LogsBroadcaster: bs,
	})
	if err != nil {
		log.Fatal(err)
	}

	auth := auth.New(st, auth.NewGithub(auth.GithubConfig{
		RedirectURL:  os.Getenv("JTD_REDIRECT_URL"),
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
	}))

	gh, err := github.New(github.Config{
		AppID:      os.Getenv("GITHUB_APP_ID"),
		PrivateKey: os.Getenv("GITHUB_PRIVATE_KEY_PATH"),
		PublicURL:  os.Getenv("JTD_PUBLIC_URL"),
	}, st, mgr)

	if err != nil {
		log.Fatal(err)
	}

	router := api.New(mgr, auth, gh)

	srv := http.Server{
		Addr:    ":5500",
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
