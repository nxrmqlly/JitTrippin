package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/nxrmqlly/jittrippin/helpers"
	"github.com/nxrmqlly/jittrippin/pkg/artifact"
	"github.com/nxrmqlly/jittrippin/pkg/checkout"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"github.com/nxrmqlly/jittrippin/pkg/logs"
	"github.com/nxrmqlly/jittrippin/pkg/runner"
	"github.com/urfave/cli/v3"
)

func CmdRun() *cli.Command {
	return &cli.Command{
		Name:   "run",
		Usage:  "runs a pipeline",
		Action: handleRun,
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name: "pipeline-file",
			},
		},
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "parallel",
				Aliases: []string{"p"},
				Value:   -1,
				Usage:   "maximum concurrent jobs (-1 = automatic)",
			},
			&cli.StringFlag{
				Name:    "artifact-dir",
				Aliases: []string{"a"},
				Value:   ".jt-artifacts/",
				Usage:   "artifact storage directory (default: .jt-artifacts/)",
			},
			&cli.BoolFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Value:   false,
				Usage:   "supress logs",
			},
		},
	}
}

func loadPipeline(path string) (*engine.Pipeline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return engine.ProcessJSON(string(raw))
}

func handleRun(ctx context.Context, c *cli.Command) error {
	pipelineFile := c.StringArg("pipeline-file")
	parallel := c.Int("parallel")
	artifactDir := c.String("artifact-dir")
	quiet := c.Bool("quiet")

	if pipelineFile == "" {
		return fmt.Errorf("pipeline file or directory path is required")
	}

	var stdout, stderr io.Writer

	if quiet {
		stdout = io.Discard
		stderr = io.Discard
	} else {
		stdout = os.Stdout
		stderr = os.Stderr
	}

	ls := logs.NewWritersStore(stdout, stderr)

	fmt.Printf("Reading pipeline: %s\n", pipelineFile)

	p, err := loadPipeline(pipelineFile)
	if err != nil {
		return err
	}

	r, err := runner.NewDockerRunner()
	if err != nil {
		return err
	}

	store := &artifact.LocalStore{
		Root: artifactDir,
	}

	exec := engine.NewExecutor(engine.ExecutorConfig{
		MaxParallel: parallel,

		Runner:        r,
		ArtifactStore: store,
		Checkouter:    &checkout.GitCheckouter{},
		LogsStore:     ls,
	})
	defer exec.Shutdown()

	pe, err := exec.Submit(
		ctx,
		helpers.MustUUIDV7(),
		p,
	)
	if err != nil {
		return err
	}

	if err := pe.Wait(); err != nil {
		return fmt.Errorf("Pipeline failed: %v", err)
	}

	return nil
}
