package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/huh/v2"
	"github.com/nxrmqlly/jittrippin/internal/config"
	"github.com/urfave/cli/v3"
)

const projConfFile = ".jtrc"

func findProjectConfig() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		p := filepath.Join(dir, projConfFile)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func loadProjectConfig() (*config.Config, error) {
	cfg := &config.Config{Pipelines: struct {
		Dir string `toml:"dir"`
	}{Dir: ".jt"}}
	if p, ok := findProjectConfig(); ok {
		loaded, err := config.LoadConfig(p)
		if err != nil {
			return nil, err
		}
		if loaded.Project.Name != "" {
			cfg.Project.Name = loaded.Project.Name
		}
		if loaded.Pipelines.Dir != "" {
			cfg.Pipelines.Dir = loaded.Pipelines.Dir
		}
	} else if dir, err := os.Getwd(); err == nil {
		cfg.Project.Name = filepath.Base(dir)
	}
	return cfg, nil
}

func saveProjectConfig(cfg *config.Config) error {
	return config.Save(projConfFile, cfg)
}

func CmdInit() *cli.Command {
	return &cli.Command{
		Name:    "init",
		Aliases: []string{"i"},
		Usage:   "set up a .jtrc project config in the current directory",
		Action:  handleInit,
	}
}

func handleInit(ctx context.Context, c *cli.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg := &config.Config{}
	cfg.Project.Name = filepath.Base(cwd)
	cfg.Pipelines.Dir = ".jt"

	name := cfg.Project.Name
	if err := huh.NewInput().Title("Project name").Value(&name).Run(); err != nil {
		return err
	}
	dir := cfg.Pipelines.Dir
	if err := huh.NewInput().Title("Pipelines directory").Value(&dir).Run(); err != nil {
		return err
	}
	cfg.Project.Name = name
	cfg.Pipelines.Dir = dir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := saveProjectConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Initialized new project\n\n\t%s\n\nin %s\n", name, cwd)
	return nil

}
