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
	"os"

	"github.com/nxrmqlly/jittrippin/internal/runner"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

func main() {
	ctx := context.Background()
	pipelinePath := "example_pipeline.json"

	fmt.Printf("🔍 Reading pipeline file: %s...\n", pipelinePath)

	// 1. The Engine Engines (parse the pipeline config)
	pipeline, err := engine.ProcessJSONFile(pipelinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Engine failed to parse JSON file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🚀 Starting Pipeline: %q\n", pipeline.Name)
	fmt.Printf("📋 Description: %s\n\n", pipeline.Description)

	// 2. Loop through our jobs and run them using our runner
	for _, job := range pipeline.Jobs {
		fmt.Printf("=========================================\n")
		fmt.Printf("🎬 Executing Job: %s (Image: %s)\n", job.Name, job.Image)
		fmt.Printf("=========================================\n")

		// The Runner Runs (execute the job step-by-step and stream logs to terminal)
		err := runner.RunJob(ctx, job, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\n❌ Job %q failed: %v\n", job.Name, err)
			os.Exit(1)
		}

		fmt.Printf("\n🎉 Job %q finished successfully!\n", job.Name)
	}

	fmt.Println("\n🏁 All pipeline tasks completed successfully.")
}
