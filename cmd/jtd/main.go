package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/daemon/api"
	"github.com/nxrmqlly/jittrippin/internal/store"
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
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

	st := store.New(pool)

	as := &artifact.LocalStore{
		Root:      ".jt-artifacts",
		Extension: ".tar",
	}

	ch := &checkout.GitCheckouter{}

	rn, err := runner.NewDockerRunner()
	if err != nil {
		log.Fatal(err)
	}

	e := engine.NewExecutor(engine.ExecutorConfig{
		MaxParallel: -1,

		Runner:        rn,
		ArtifactStore: as,
		Checkouter:    ch,
	})

	mgr, err := daemon.NewManager(ctx, e, st)
	if err != nil {
		log.Fatal(err)
	}
	router := api.NewRouter(mgr)

	srv := http.Server{
		Addr:    ":5500",
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
