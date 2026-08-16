package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"charm.land/huh/v2"
	"github.com/urfave/cli/v3"
)

func CmdIntegrations() *cli.Command {
	return &cli.Command{
		Name:    "integrations",
		Aliases: []string{"intg"},
		Usage:   "manage service integrations on the daemons",
		Commands: []*cli.Command{
			CmdIntegrationsList(),
			CmdIntegrationsAdd(),
			CmdIntegrationsRemove(),
		},
	}
}

func CmdIntegrationsList() *cli.Command {
	return &cli.Command{
		Name:   "list",
		Usage:  "list connected integrations",
		Action: handleIntegrationsList,
	}
}

func handleIntegrationsList(ctx context.Context, c *cli.Command) error {
	client, err := ensureSession(c)
	if err != nil {
		return err
	}
	integrations, err := client.MyIntegrations()
	if err != nil {
		return err
	}
	if len(integrations) == 0 {
		fmt.Println("No integrations connected. Run 'jt integrations add'.")
		return nil
	}
	for _, in := range integrations {
		fmt.Printf("    - %s", in.Provider)
		if len(in.Installations) > 0 {
			logins := make([]string, 0, len(in.Installations))
			for _, a := range in.Installations {
				logins = append(logins, a.AccountLogin)
			}
			fmt.Printf(" (installed on: %s)", strings.Join(logins, ", "))
		}
		fmt.Println()
	}
	return nil
}

func waitForInstall(client *daemonClient) error {
	deadline := time.Now().Add(loginTimeout)
	for {
		st, err := client.InstallStatus()
		if err == nil && st.Installed {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("timed out waiting for installation")
		}
		fmt.Println("    waiting for install...")
		time.Sleep(2 * time.Second)
	}
}

func CmdIntegrationsAdd() *cli.Command {
	return &cli.Command{
		Name:   "add",
		Usage:  "connect a service integration",
		Action: handleIntegrationsAdd,
	}
}

func handleIntegrationsAdd(ctx context.Context, c *cli.Command) error {
	client, err := ensureSession(c)
	if err != nil {
		return err
	}
	providers, err := client.IntegrationProviders()
	if err != nil {
		return err
	}
	if len(providers) == 0 {
		return errors.New("daemon supports no integrations")
	}
	provider := providers[0]
	if len(providers) > 1 {
		if err := huh.NewSelect[string]().Title("Integration").
			Options(huh.NewOptions(providers...)...).
			Value(&provider).Run(); err != nil {
			return err
		}
	}
	me, err := client.Me()
	if err != nil {
		return err
	}
	hasIdentity := false
	for _, id := range me.Identities {
		if id.Provider == provider {
			hasIdentity = true
			break
		}
	}
	if !hasIdentity {
		return fmt.Errorf("%s account not connected: run 'jt auth login' first", provider)
	}
	st, err := client.InstallStatus()
	if err != nil {
		var ae apiErr
		if errors.As(err, &ae) && ae.Status == http.StatusForbidden {
			return fmt.Errorf("%s account not connected: run 'jt auth login' first", provider)
		}
		return err
	}
	if !st.Installed {
		fmt.Printf("JitTrippin is not installed for your account.\nOpen this URL to install it:\n\n    %s\n\n", st.InstallURL)
		_ = openBrowser(st.InstallURL)
		if err := waitForInstall(client); err != nil {
			return err
		}
	}
	accounts, err := client.LinkIntegration(provider)
	if err != nil {
		return err
	}
	if len(accounts) == 0 {
		return errors.New("no installations found to link")
	}
	fmt.Print("OK connected: ")
	for _, a := range accounts {
		fmt.Printf("%s ", a.Login)
	}
	fmt.Println()
	return nil
}

func CmdIntegrationsRemove() *cli.Command {
	return &cli.Command{
		Name:   "remove",
		Usage:  "disconnect an integration",
		Action: handleIntegrationsRemove,
	}
}

func handleIntegrationsRemove(ctx context.Context, c *cli.Command) error {
	client, err := ensureSession(c)
	if err != nil {
		return err
	}
	integrations, err := client.MyIntegrations()
	if err != nil {
		return err
	}
	if len(integrations) == 0 {
		return errors.New("no integrations connected")
	}
	opts := make([]huh.Option[string], 0, len(integrations))
	for _, in := range integrations {
		opts = append(opts, huh.NewOption(in.Provider, in.Provider))
	}
	var provider string
	if err := huh.NewSelect[string]().Title("Integration").
		Options(opts...).
		Value(&provider).Run(); err != nil {
		return err
	}
	repos, err := client.TrackedRepos()
	if err != nil {
		return err
	}

	ok := false
	if err := huh.NewConfirm().Title(fmt.Sprintf(
		"This removes the %s integration and %d tracked repositories", provider, len(repos),
	)).Value(&ok).Run(); err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted")
	}

	if err := client.RemoveIntegration(provider); err != nil {
		return err
	}
	fmt.Printf("Disconnected %s.\n", provider)
	return nil
}
