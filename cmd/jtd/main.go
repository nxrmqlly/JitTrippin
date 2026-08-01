package main

import (
	"net/http"

	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/daemon/api"
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/runner"
)

func main() {

	s := &artifact.LocalStore{
		Root: ".jt-pipelines",
		Extension: ".tar",
	}

	c := &checkout.GitCheckouter{}

	r, err := runner.NewDockerRunner()
	if err != nil {
		panic(err)
	}

	e := engine.NewExecutor(engine.ExecutorConfig{
		MaxParallel: 0,

		Runner:        r,
		ArtifactStore: s,
		Checkouter:    c,
	})

	mgr, err := daemon.NewManager(e)
	if err != nil {
		panic(err)
	}
	router := api.NewRouter(mgr)

	srv := http.Server{
		Addr:    ":5500",
		Handler: router,
	}

	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}
