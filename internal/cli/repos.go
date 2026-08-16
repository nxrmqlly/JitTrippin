package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"charm.land/huh/v2"
	"github.com/nxrmqlly/jittrippin/helpers"
	"github.com/urfave/cli/v3"
)

func CmdRepos() *cli.Command {
	return &cli.Command{
		Name:        "repos",
		Description: "track GitHub repositories",
		Commands: []*cli.Command{
			CmdReposAdd(),
		},
	}
}

func CmdReposAdd() *cli.Command {
	return &cli.Command{
		Name:        "add",
		Description: "track a repository",
		Action:      handleReposAdd,
	}
}

func handleReposAdd(ctx context.Context, c *cli.Command) error {
	client, err := ensureSession(c)
	if err != nil {
		return err
	}

	st, err := client.InstallStatus()
	if err != nil {
		var ae apiErr
		if errors.As(err, &ae) && ae.Status == http.StatusForbidden {
			return errors.New("GitHub account not connected: run 'jt auth login' and pick the github provider")
		}
		return err
	}
	if !st.Installed {
		fmt.Printf("JitTrippin is not installed for your GitHub account.\nOpen this URL to install it:\n\n    %s\n\n", st.InstallURL)
		_ = openBrowser(st.InstallURL)
		deadline := time.Now().Add(loginTimeout)
		for {
			cur, err := client.InstallStatus()
			if err == nil && cur.Installed {
				fmt.Println("    OK JitTrippin installed")
				break
			}
			if time.Now().After(deadline) {
				return errors.New("timed out waiting for installation")
			}
			fmt.Println("    waiting for install...")
			time.Sleep(2 * time.Second)
		}
	}
	repos, err := client.InstallableRepos()
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return errors.New("no installable repositories found")
	}
	opts := make([]huh.Option[string], 0, len(repos))
	for _, r := range repos {
		opts = append(opts, huh.NewOption(r.FullName, r.FullName))
	}
	var fullname string
	if err := huh.NewSelect[string]().Title("Repository").
		Options(opts...).
		Value(&fullname).Run(); err != nil {
		return err
	}
	defaultBranch := ""
	for _, r := range repos {
		if r.FullName == fullname {
			defaultBranch = r.DefaultBranch
			break
		}
	}
	branches, err := client.ListBranches(fullname)
	if err != nil {
		return err
	}
	branch := defaultBranch
	if branch == "" || !helpers.ContainsS(branches, branch) {
		if len(branches) == 1 {
			branch = branches[0]
		} else if len(branches) > 1 {
			bopts := make([]huh.Option[string], 0, len(branches))
			for _, b := range branches {
				bopts = append(bopts, huh.NewOption(b, b))
			}
			if err := huh.NewSelect[string]().Title("Branch").
				Options(bopts...).
				Value(&branch).Run(); err != nil {
				return err
			}
		}
	}
	if err := client.ConnectGithub(fullname, branch); err != nil {
		return err
	}
	fmt.Printf("OK tracking %s (%s)\n", fullname, branch)
	return nil
}
