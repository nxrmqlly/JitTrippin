// // copied from https://github.com/docker/go-sdk README
// package main

// import (
// 	"context"
// 	"io"
// 	"os"

// 	"github.com/docker/go-sdk/container"
// 	"github.com/docker/go-sdk/container/wait"
// )

// func main() {

// 	ctr, err := container.Run(
// 		context.Background(),
// 		container.WithImage("alpine:latest"),
// 		container.WithCmd("echo", "hello world"),
// 		container.WithWaitStrategy(wait.ForLog("hello world")),
// 	)
// 	if err != nil {
// 		panic(err)
// 	}

// 	logs, err := ctr.Logs(context.Background())
// 	if err != nil {
// 		panic(err)
// 	}

// 	io.Copy(os.Stdout, logs)

// 	err = ctr.Terminate(context.Background())
// 	if err != nil {
// 		panic(err)
// 	}
// }

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nxrmqlly/jittrippin/internal/runner"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
)

func main() {
	// 1. Define a mock intermediate JSON string matching your engine's blueprint
	// rawJSON := `{
	// 	"name": "Local Test Pipeline",
	// 	"description": "Validating the engine to runner link",
	// 	"jobs": [
	// 		{
	// 			"name": "build",
	// 			"image": "alpine:latest",
	// 			"steps": [
	// 				{"name": "Update packages", "cmd": "apk update"},
	// 				{"name": "Verify dependencies", "cmd": "echo 'All clear'"}
	// 			]
	// 		}
	// 	]
	// }`

	// 2. The Engine Engines: Process the JSON raw data
	pipeline, err := engine.ProcessJSONFile("./example_pipeline.json")
	if err != nil {
		fmt.Printf("Engine failed to parse configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded Pipeline: %s - %s\n", pipeline.Name, pipeline.Description)

	// 3. The Runner Runs: Execute the jobs sequentially
	for _, job := range pipeline.Jobs {
		fmt.Printf("\n--- Triggering Runner for Job: %s ---\n", job.Name)

		// Run your minimal loop implementation
		err := runner.RunJob(context.Background(), job, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Printf("Execution failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("\nAll jobs successfully sent through the pipeline!")
}
