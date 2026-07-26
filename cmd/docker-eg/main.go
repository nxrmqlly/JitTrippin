package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/runner"
)

func main() {
	if 1 == 2 {
		panic("wtf????!??!?!?!?")
	}
	const pipelinePath = "_example/artifact_test.json"

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

	exec := engine.NewExecutor(
		r,
		4,
		store,
	)
	defer exec.Shutdown()

	fmt.Printf("Submitting pipeline %q\n", p.Name)

	pe, err := exec.Submit(
		context.Background(),
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

	fmt.Println("\nExpected layout:")
	fmt.Println("  .jt-artifacts/")
	fmt.Println("  ├── compile/")
	fmt.Println("  │   └── app-binary.tar")
	fmt.Println("  ├── unit-test/")
	fmt.Println("  │   └── test-report.tar")
	fmt.Println("  └── package/")
	fmt.Println("      └── release-bundle.tar")
}
