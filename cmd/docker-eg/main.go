package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/runner"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	const pipelinePath = "_example/checkout_artifact.json"

	fmt.Printf("Reading pipeline: %s\n", pipelinePath)

	raw, err := os.ReadFile(pipelinePath)
	if err != nil {
		log.Fatal(err)
	}

	p, err := engine.ProcessJSON(string(raw))
	if err != nil {
		log.Fatal(err)
	}

	r, err := runner.NewDockerRunner()
	if err != nil {
		log.Fatal(err)
	}

	store := &artifact.LocalStore{
		Root: ".jt-artifacts",
	}

	exec := engine.NewExecutor(engine.ExecutorConfig{
		MaxParallel:   4,
		Runner:        r,
		ArtifactStore: store,
		Checkouter:    &checkout.GitCheckouter{},
	})
	defer exec.Shutdown()

	fmt.Printf("Submitting pipeline %q\n", p.Name)

	pe, err := exec.Submit(
		ctx,
		p,
		os.Stdout,
		os.Stderr,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Waiting for pipeline...")

	if err := pe.Wait(); err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	fmt.Println("\nPipeline completed successfully!")
	fmt.Println("Artifacts saved under .jt-artifacts/")
}
