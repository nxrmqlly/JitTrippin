package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"
)

func CmdConnect() *cli.Command {
	return &cli.Command{
		Name:   "connect",
		Usage:  "track a GitHub repository",
		Action: handleConnect,
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "repository"},
			&cli.StringArg{Name: "branch"},
		},
		Flags: []cli.Flag{},
	}
}

func handleConnect(ctx context.Context, c *cli.Command) error {
	fullName := c.StringArg("repository")
	branch := c.StringArg("branch")

	if fullName == "" || branch == "" {
		return errors.New("please specify repository and branch")
	}

	sess, err := loadSession()
	if err != nil {
		return err
	}
	if sess == nil || sess.Token == "" {
		return errors.New("not logged in: run 'jt login' first")
	}
	base, err := daemonURL(c)
	if err != nil {
		return err
	}
	client := newDaemonClient(base)
	client.token = sess.Token

	if err := client.ConnectGithub(fullName, branch); err != nil {
		return err
	}
	fmt.Printf("Tracking %s (%s) successful.", fullName, branch)
	return nil
}
