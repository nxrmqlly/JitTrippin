package github

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"
	"github.com/nxrmqlly/jittrippin/internal/daemon"
	"github.com/nxrmqlly/jittrippin/internal/store"
	"golang.org/x/oauth2"
)

var ErrInstallNotFound = errors.New("github: app is not installed for repository owner")

// Integration is the Github bot
type Integration struct {
	appTS oauth2.TokenSource
	st    *store.Store
	mgr   *daemon.Manager
	srcs  *installTokenSourceCache
	pub   string
	slug  string
}

type Config struct {
	AppID      string
	PrivateKey string
	PublicURL  string
	AppSlug    string
}

func New(cfg Config, st *store.Store, mgr *daemon.Manager) (*Integration, error) {
	appTS, err := loadAppTokenSrc(cfg.AppID, cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	return &Integration{
		appTS: appTS,
		st:    st,
		mgr:   mgr,
		srcs:  newInstallTokenSourceCache(),
		pub:   cfg.PublicURL,
		slug:  cfg.AppSlug,
	}, nil
}

func (i *Integration) installForOwner(ctx context.Context, cu *Client, owner string) (int64, error) {
	installs, err := cu.ListUserInstallations(ctx)
	if err != nil {
		return 0, err
	}
	for _, inst := range installs {
		if inst.AccountLogin == owner {
			return inst.ID, nil
		}
	}
	return 0, ErrInstallNotFound
}

func (i *Integration) SubmitPush(ctx context.Context, tracked store.TrackedRepository, sha string, installID int64) ([]*daemon.Run, error) {
	ts := i.srcs.Get(installID, i.appTS)

	c, err := forInstallation(ts)
	if err != nil {
		return nil, err
	}

	tok, err := ts.Token()
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(tracked.FullName, "/", 2)
	if len(parts) != 2 {
		return nil, errors.New("github: invalid full_name " + tracked.FullName)
	}
	owner, name := parts[0], parts[1]
	pipelines, err := LoadPipelinesAtSHA(ctx, c, owner, name, sha)
	if err != nil {
		return nil, err
	}

	var runs []*daemon.Run
	for _, p := range pipelines {
		p.Checkout.URL = "https://github.com/" + tracked.FullName + ".git"
		p.Checkout.Ref = sha
		p.Checkout.Username = "x-access-token"
		p.Checkout.Password = tok.AccessToken

		run, err := i.mgr.Submit(tracked.OwnerID, p)
		if err != nil {
			return runs, err
		}
		runs = append(runs, run)

		statusCtx := "jittrippin/" + p.Name
		target := i.pub + "/runs/" + run.ID()
		if err := c.CreateCommitStatus(ctx, CreateCommitStatusConfig{
			// owner, name, sha, "pending", statusCtx, "jittrippin: run started", target
			Owner:     owner,
			Name:      name,
			SHA:       sha,
			State:     "pending",
			Context:   statusCtx,
			Desc:      "jittrippin: run started",
			TargetURL: target,
		}); err != nil {
			log.Printf("github: pending status failed: %v", err)
		}
		go i.reportCompletion(ctx, c, owner, name, sha, p.Name, run, target)
	}
	return runs, nil
}

func (i *Integration) reportCompletion(ctx context.Context, c *Client, owner, name, sha, pipelineName string, run *daemon.Run, target string) {
	run.Wait()
	state := "success"
	descr := "jittrippin: " + pipelineName + " [passed]"
	if run.Status() == daemon.StatusFailed {
		state = "failure"
		descr = "jittrippin: " + pipelineName + " [failed]"
	}
	if err := c.CreateCommitStatus(ctx, CreateCommitStatusConfig{
		Owner:     owner,
		Name:      name,
		SHA:       sha,
		State:     state,
		Context:   "jittrippin/" + pipelineName,
		Desc:      descr,
		TargetURL: target,
	}); err != nil {
		log.Printf("github: final status failed: %v", err)
	}

}

func (i *Integration) TrackRepository(ctx context.Context, userToken, ownerID, fullName, branch string) error {
	cu, err := forUser(userToken)
	if err != nil {
		return err
	}

	owner, name, _ := strings.Cut(fullName, "/")
	installID, err := i.installForOwner(ctx, cu, owner)
	if err != nil {
		return err
	}

	ts := i.srcs.Get(installID, i.appTS)
	c, err := forInstallation(ts)
	if err != nil {
		return err
	}

	repo, err := c.FetchRepository(ctx, owner, name)
	if err != nil {
		return err
	}

	return i.mgr.TrackRepository(daemon.TrackRepositoryConfig{
		OwnerID:              ownerID,
		Provider:             "github",
		ProviderInstance:     "github.com",
		ProviderRepositoryID: repo.ID,
		FullName:             repo.FullName,
		Branch:               branch,
	})
}

func (i *Integration) RecordInstall(ctx context.Context, installID int64, setupAction string) error {
	httpC := oauth2.NewClient(ctx, i.appTS)
	c, err := gh.NewClient(gh.WithHTTPClient(httpC))
	if err != nil {
		return err
	}
	inst, _, err := c.Apps.GetInstallation(ctx, installID)
	if err != nil {
		return err
	}
	rec := &store.Installation{
		ID:               strconv.FormatInt(installID, 10),
		Provider:         "github",
		ProviderInstance: "github.com",
		SetupAction:      setupAction,
	}
	if a := inst.GetAccount(); a != nil {
		rec.AccountLogin = a.GetLogin()
		rec.AccountType = a.GetType()
	}
	return i.st.UpsertInstallation(ctx, rec)
}

type InstallStatusResult struct {
	Installed  bool
	Accounts   []Installation
	InstallURL string
}

func (i *Integration) InstallStatus(ctx context.Context, userToken string) (InstallStatusResult, error) {
	cu, err := forUser(userToken)
	if err != nil {
		return InstallStatusResult{}, err
	}
	installs, err := cu.ListUserInstallations(ctx)
	if err != nil {
		return InstallStatusResult{}, err
	}
	return InstallStatusResult{
		Installed:  len(installs) > 0,
		Accounts:   installs,
		InstallURL: "https://github.com/apps/" + i.slug + "/installations/new",
	}, nil
}

func (i *Integration) ListBranches(ctx context.Context, userToken, fullName string) ([]string, error) {
	owner, name, _ := strings.Cut(fullName, "/")

	cu, err := forUser(userToken)
	if err != nil {
		return nil, err
	}
	installID, err := i.installForOwner(ctx, cu, owner)
	if err != nil {
		return nil, err
	}
	ts := i.srcs.Get(installID, i.appTS)
	c, err := forInstallation(ts)
	if err != nil {
		return nil, err
	}
	return c.ListBranches(ctx, owner, name)
}
