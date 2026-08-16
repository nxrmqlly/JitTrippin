package cli

import (
	"context"

	"github.com/urfave/cli/v3"
)

func New() *cli.Command {
	return &cli.Command{
		Name:  "jt",
		Usage: "jittrippin - the local ci/cd engine",
		Action: func(_ context.Context, c *cli.Command) error {
			cli.ShowRootCommandHelp(c)
			return nil
		},
		Commands: []*cli.Command{
			CmdInit(),
			CmdRun(),
			CmdValidate(),
			CmdAuth(),
			CmdRepos(),
			CmdPing(),
			CmdDaemon(),
			CmdIntegrations(),
		},
	}
}
