package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/huh/v2"
	"github.com/urfave/cli/v3"
)

func CmdPing() *cli.Command {
	return &cli.Command{
		Name:        "ping",
		Description: "check that the daemon is reachable",
		Flags:       []cli.Flag{&cli.StringFlag{Name: "daemon", Usage: "daemon base URL (default: from user config)"}},
		Action:      handlePing,
	}
}

func handlePing(ctx context.Context, c *cli.Command) error {
	base, err := daemonURL(c)
	if err != nil {
		return err
	}
	client := newDaemonClient(base)
	start := time.Now()
	if err := checkHealth(ctx, client); err != nil {
		return fmt.Errorf("daemon %s is unreachable: %w", base, err)
	}
	fmt.Printf("ok %s (%s)\n", base, time.Since(start).Round(time.Millisecond))
	return nil
}

func CmdDaemon() *cli.Command {
	return &cli.Command{
		Name:        "daemon",
		Description: "show or set the daemon URL",
		Flags:       []cli.Flag{&cli.StringFlag{Name: "set", Usage: "set a new daemon URL"}},
		Action:      handleDaemon,
	}
}

func handleDaemon(ctx context.Context, c *cli.Command) error {
	if set := c.String("set"); set != "" {
		set = strings.TrimRight(set, "/")
		if err := checkHealth(ctx, newDaemonClient(set)); err != nil {
			return fmt.Errorf("cannot switch to %s: %w", set, err)
		}
		if hasSession() {
			ok := false
			if err := huh.NewConfirm().Title("Switching daemon will log you out").
				Value(&ok).Run(); err != nil {
				return err
			}
			if !ok {
				return errors.New("aborted")
			}
			if err := clearSession(); err != nil {
				return err
			}
		}
		cfg, err := loadCfg()
		if err != nil {
			return err
		}
		cfg.Daemon = set
		if err := saveCfg(cfg); err != nil {
			return err
		}
		fmt.Printf("Daemon set to %s\n", set)
		return nil
	}
	base, err := daemonURL(c)
	if err != nil {
		return err
	}
	client := newDaemonClient(base)
	if err := checkHealth(ctx, client); err != nil {
		fmt.Printf("Daemon: %s (unreachable: %v)\n", base, err)
		return nil
	}
	fmt.Printf("Daemon: %s (ok)\n", base)
	return nil

}
