package cli

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func CmdValidate() *cli.Command {
	return &cli.Command{
		Name:   "validate",
		Usage:  "validates a pipeline",
		Action: handleValidate,
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name: "pipeline-file",
			},
		},
		Flags: []cli.Flag{},
	}
}

func handleValidate(ctx context.Context, c *cli.Command) error {
	pipelineFile := c.StringArg("pipeline-file")

	p, err := loadPipeline(pipelineFile)
	if err != nil {
		return err
	}

	if err := p.Validate(); err != nil {
		return err
	}

	fmt.Println("pipeline validation passed")
	return nil

}
