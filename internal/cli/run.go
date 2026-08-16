package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
		Usage:  "run pipelines from the project (remotely on the daemon by default)",
		Action: handleRun,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "pipeline",
				Aliases: []string{"p"},
				Usage:   "only run the pipeline with this name",
			},
			&cli.BoolFlag{
				Name:  "local",
				Usage: "run locally with docker instead of on the daemon",
			},
			&cli.StringFlag{
				Name:  "daemon",
				Usage: "daemon base URL (default: from user config)",
			},
			&cli.IntFlag{
				Name:  "parallel",
				Value: -1,
				Usage: "maximum concurrent jobs (-1 = automatic, local only)",
			},
			&cli.StringFlag{
				Name:    "artifact-dir",
				Aliases: []string{"a"},
				Value:   ".jt-artifacts/",
				Usage:   "artifact storage directory (local only)",
			},
			&cli.BoolFlag{
				Name:    "quiet",
				Aliases: []string{"q"},
				Usage:   "only show the final checklist, not live logs",
			},
		},
	}
}

// loadPipeline loads a single pipeline from a file. Used by `jt validate`.
func loadPipeline(path string) (*engine.Pipeline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return engine.ProcessJSON(string(raw))
}

func handleRun(ctx context.Context, c *cli.Command) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	cfg, err := loadProjectConfig()
	if err != nil {
		return err
	}

	pipelines, err := resolvePipelines(cfg.Pipelines.Dir, c.String("pipeline"))
	if err != nil {
		return err
	}

	if !c.Bool("local") && !hasSession() {
		fmt.Println("Not logged in; running locally. Run 'jt auth login' to run on the daemon.")
	}
	if c.Bool("local") || !hasSession() {
		return runLocal(ctx, c, pipelines)
	}

	fillCheckout(pipelines)
	return runRemote(ctx, c, pipelines)
}

func resolvePipelines(dir, filter string) ([]*engine.Pipeline, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("pipelines directory %q not found (run 'jt init'?)", dir)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}

	var pipelines []*engine.Pipeline
	var names []string
	for _, f := range files {
		p, err := engine.ProcessJSONFile(f)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", f, err)
		}
		names = append(names, p.Name)
		if filter != "" && p.Name != filter && filepath.Base(f) != filter && filepath.Base(f) != filter+".json" {
			continue
		}
		pipelines = append(pipelines, p)
	}

	if len(pipelines) == 0 {
		if len(names) == 0 {
			return nil, fmt.Errorf("no pipelines found in %q", dir)
		}
		return nil, fmt.Errorf("pipeline %q not found; available: %s", filter, strings.Join(names, ", "))
	}

	return pipelines, nil
}

// fillCheckout best-effort fills the checkout URL and ref from the current git
// repository so the daemon can clone the right code.
func fillCheckout(pipelines []*engine.Pipeline) {
	url := gitOutput("git", "config", "--get", "remote.origin.url")
	ref := gitOutput("git", "rev-parse", "--abbrev-ref", "HEAD")

	for _, p := range pipelines {
		if p.Checkout.URL == "" {
			p.Checkout.URL = url
		}
		if p.Checkout.Ref == "" {
			p.Checkout.Ref = ref
		}
	}

	if url == "" {
		for _, p := range pipelines {
			if p.Checkout.URL == "" {
				fmt.Fprintln(os.Stderr, "warning: no git remote origin detected; the daemon checkout may fail")
				break
			}
		}
	}
}

func gitOutput(args ...string) string {
	if len(args) == 0 {
		return ""
	}
	out, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func runLocal(ctx context.Context, c *cli.Command, pipelines []*engine.Pipeline) error {
	parallel := c.Int("parallel")
	artifactDir := c.String("artifact-dir")
	quiet := c.Bool("quiet")

	var stdout, stderr io.Writer
	if quiet {
		stdout, stderr = io.Discard, io.Discard
	} else {
		stdout, stderr = os.Stdout, os.Stderr
	}

	for _, p := range pipelines {
		fmt.Printf("Running pipeline %s locally\n", p.Name)
		if err := runOneLocal(ctx, p, parallel, artifactDir, stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}

func runOneLocal(ctx context.Context, p *engine.Pipeline, parallel int, artifactDir string, stdout, stderr io.Writer) error {
	ls := logs.NewWritersStore(stdout, stderr)

	r, err := runner.NewDockerRunner()
	if err != nil {
		return err
	}

	store := &artifact.LocalStore{
		Root: artifactDir,
	}

	exec := engine.NewExecutor(engine.ExecutorConfig{
		MaxParallel:   parallel,
		Runner:        r,
		ArtifactStore: store,
		Checkouter:    &checkout.GitCheckouter{},
		LogsStore:     ls,
	})
	defer exec.Shutdown()

	pe, err := exec.Submit(ctx, helpers.MustUUIDV7(), p)
	if err != nil {
		return err
	}

	if err := pe.Wait(); err != nil {
		return fmt.Errorf("pipeline %q failed: %v", p.Name, err)
	}
	return nil
}

type liveJob struct {
	name string

	mu   sync.Mutex
	tail []string
}

func (j *liveJob) push(data string) {
	line := strings.TrimSpace(data)
	if line == "" {
		return
	}
	if len(line) > 120 {
		line = line[:120] + "..."
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.tail = append(j.tail, line)
	if len(j.tail) > 3 {
		j.tail = j.tail[len(j.tail)-3:]
	}
}

func (j *liveJob) Tail() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.tail...)
}

func (j *liveJob) stream(client *daemonClient, runID string) {
	ch, stop, err := client.StreamLogs(runID, j.name)
	if err != nil {
		return
	}
	defer stop()
	for ll := range ch {
		j.push(ll.Data)
	}
}

type liveRun struct {
	p        *engine.Pipeline
	id       string
	jobs     []*liveJob
	status   string
	err      *string
	errCount int
}

func (r *liveRun) terminal() bool {
	return r.status != "running"
}

func stdoutIsTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func runRemote(ctx context.Context, c *cli.Command, pipelines []*engine.Pipeline) error {
	client, err := ensureSession(c)
	if err != nil {
		return err
	}

	runs := make([]*liveRun, 0, len(pipelines))
	for _, p := range pipelines {
		id, err := client.SubmitRun(p)
		if err != nil {
			return fmt.Errorf("submit pipeline %q: %w", p.Name, err)
		}
		fmt.Printf("%s: %s/runs/%s\n", p.Name, client.baseURL, id)
		lr := &liveRun{p: p, id: id}
		for _, job := range p.Jobs {
			lj := &liveJob{name: job.Name}
			lr.jobs = append(lr.jobs, lj)
			go lj.stream(client, id)
		}
		runs = append(runs, lr)
	}
	fmt.Println()

	live := !c.Bool("quiet") && stdoutIsTerminal()

	var lastLines int
	clearBlock := func() {
		for range lastLines {
			fmt.Print("\x1b[1A\x1b[2K")
		}
		lastLines = 0
	}

	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	failed := false
	for {
		allDone := true
		for _, lr := range runs {
			res, err := client.RunGet(lr.id)
			if err != nil {
				lr.errCount++
				if lr.errCount >= 3 {
					lr.status = "error"
					failed = true
				}
			} else {
				lr.errCount = 0
				lr.status = res.Status
				if res.Error != nil {
					lr.err = res.Error
				}
			}
			if !lr.terminal() {
				allDone = false
			} else if lr.status == "failed" || lr.status == "error" {
				failed = true
			}
		}

		if live {
			clearBlock()
			lastLines = renderLive(runs)
		}

		if allDone || ctx.Err() != nil {
			break
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
		}
	}

	if live {
		clearBlock()
	}
	renderChecklist(runs, client.baseURL)

	if ctx.Err() != nil {
		for _, lr := range runs {
			if !lr.terminal() {
				_ = client.CancelRun(lr.id)
			}
		}
		fmt.Println("\nCancelled.")
		return ctx.Err()
	}

	if failed {
		return fmt.Errorf("one or more pipelines failed")
	}
	return nil
}

func renderLive(runs []*liveRun) int {
	lines := 0
	for _, lr := range runs {
		status := lr.status
		if status == "" {
			status = "running"
		}
		fmt.Printf("%s (%s)\n", lr.p.Name, status)
		lines++
		for _, lj := range lr.jobs {
			tail := lj.Tail()
			if len(tail) == 0 {
				fmt.Printf("  %s: (no output yet)\n", lj.name)
				lines++
				continue
			}
			fmt.Printf("  %s:\n", lj.name)
			lines++
			for _, l := range tail {
				fmt.Printf("    %s\n", l)
				lines++
			}
		}
	}
	return lines
}

func renderChecklist(runs []*liveRun, baseURL string) {
	for _, lr := range runs {
		switch lr.status {
		case "succeeded":
			fmt.Printf("✓ %s (succeeded)\n", lr.p.Name)
		case "failed", "error":
			fmt.Printf("✗ %s (failed)\n", lr.p.Name)
			if lr.err != nil && *lr.err != "" {
				fmt.Printf("  error: %s\n", *lr.err)
			}
			fmt.Printf("  logs: %s/runs/%s\n", baseURL, lr.id)
			for _, job := range lr.p.Jobs {
				fmt.Printf("    %s/runs/%s/logs/%s\n", baseURL, lr.id, job.Name)
			}
		default:
			fmt.Printf("? %s (%s)\n", lr.p.Name, lr.status)
		}
	}
}
