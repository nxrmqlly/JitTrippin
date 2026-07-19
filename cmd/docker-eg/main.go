package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nxrmqlly/jittrippin/pkg/runner"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

func main() {
	const pipelinePath = "_example/concurrency_pipeline.json"

	fmt.Printf("Reading pipeline: %s\n", pipelinePath)

	raw, err := os.ReadFile(pipelinePath)
	if err != nil {
		log.Fatal(err)
	}

	p1, err := engine.ProcessJSON(string(raw))
	if err != nil {
		log.Fatal(err)
	}

	p2, err := engine.ProcessJSON(string(raw))
	if err != nil {
		log.Fatal(err)
	}

	p3, err := engine.ProcessJSON(string(raw))
	if err != nil {
		log.Fatal(err)
	}

	exec := engine.NewSharedExecutor(&runner.DockerRunner{}, -1)

	fmt.Printf("Submitting pipeline %q\n", p1.Name)
	pe1, err := exec.Submit(context.Background(), p1, os.Stdout, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Submitting pipeline %q\n", p2.Name)
	pe2, err := exec.Submit(context.Background(), p2, os.Stdout, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Submitting pipeline %q\n", p2.Name)
	pe3, err := exec.Submit(context.Background(), p3, os.Stdout, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Both pipelines submitted, waiting...")

	if err := pe1.Wait(); err != nil {
		log.Printf("Pipeline 1 failed: %v", err)
	} else {
		fmt.Println("Pipeline 1 completed successfully")
	}

	if err := pe2.Wait(); err != nil {
		log.Printf("Pipeline 2 failed: %v", err)
	} else {
		fmt.Println("Pipeline 2 completed successfully")
	}

	if err := pe3.Wait(); err != nil {
		log.Printf("Pipeline 2 failed: %v", err)
	} else {
		fmt.Println("Pipeline 2 completed successfully")
	}

	exec.Shutdown()
	fmt.Println("Executor shut down")
}
