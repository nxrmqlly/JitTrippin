// THIS FILE IS PURE AISLOP.
// THIS FILE IS PURE AISLOP.
// THIS FILE IS PURE AISLOP.
// THIS FILE IS PURE AISLOP.
// THIS FILE IS PURE AISLOP.
// THIS FILE IS PURE AISLOP.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nxrmqlly/jittrippin/internal/runner"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

func main() {
	const pipelinePath = "_example/concurrency_pipeline.json"

	fmt.Printf("🔍 Reading pipeline: %s\n", pipelinePath)

	raw, err := os.ReadFile(pipelinePath)
	if err != nil {
		log.Fatal(err)
	}

	p, err := engine.ProcessJSON(string(raw))
	if err != nil {
		log.Fatal(err)
	}

	e := engine.LocalExecutor{
		Runner:      &runner.JobRunner{},
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		MaxParallel: 6,
	}

	fmt.Printf("🚀 Running pipeline %q\n\n", p.Name)

	if err := e.Run(context.Background(), p, os.Stdout, os.Stderr); err != nil {
		log.Fatalf("❌ Pipeline failed: %v", err)
	}

	fmt.Println("\n🏁 Pipeline completed successfully")
}
