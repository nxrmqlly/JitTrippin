package github

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"
	"github.com/nxrmqlly/jittrippin/internal/config"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"golang.org/x/oauth2"
)

const defaultPipelineDir = ".jt"

type Client struct {
	client *gh.Client
}

func forInstallation(ts oauth2.TokenSource) (*Client, error) {
	httpC := oauth2.NewClient(context.Background(), ts)
	c, err := gh.NewClient(gh.WithHTTPClient(httpC))
	if err != nil {
		return nil, err
	}
	return &Client{client: c}, nil
}

func forUser(token string) (*Client, error) {
	c, err := gh.NewClient(gh.WithAuthToken(token))
	if err != nil {
		return nil, err
	}
	return &Client{client: c}, nil
}

type Repository struct {
	ID            string
	FullName      string
	DefaultBranch string
}

func (c *Client) FetchRepository(ctx context.Context, owner string, name string) (*Repository, error) {
	repo, _, err := c.client.Repositories.Get(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	return &Repository{
		ID:            strconv.FormatInt(repo.GetID(), 10),
		FullName:      repo.GetFullName(),
		DefaultBranch: repo.GetDefaultBranch(),
	}, nil
}

type TreeEntry struct {
	Path string
	Type string
}

func (c *Client) GetTreeAtSHA(ctx context.Context, owner, name, sha string) ([]TreeEntry, error) {
	tree, _, err := c.client.Git.GetTree(ctx, owner, name, sha, true)
	if err != nil {
		return nil, err
	}
	var out []TreeEntry
	for _, tr := range tree.Entries {
		out = append(out, TreeEntry{Path: tr.GetPath(), Type: tr.GetType()})
	}
	return out, nil
}

func (c *Client) GetContentAtSHA(ctx context.Context, owner, name, path, ref string) (string, error) {
	fc, _, _, err := c.client.Repositories.GetContents(ctx, owner, name, path, &gh.RepositoryContentGetOptions{
		Ref: ref,
	})

	if err != nil {
		return "", nil
	}
	if fc == nil {
		return "", fmt.Errorf("content %q is a directory", path)
	}
	return fc.GetContent()
}

type CreateCommitStatusConfig struct {
	Owner     string
	Name      string
	SHA       string
	State     string
	Context   string
	Desc      string
	TargetURL string
}

func (c *Client) CreateCommitStatus(ctx context.Context, cfg CreateCommitStatusConfig) error {
	_, _, err := c.client.Repositories.CreateStatus(ctx, cfg.Owner, cfg.Name, cfg.SHA, gh.RepoStatus{
		State:       new(cfg.State),
		Context:     new(cfg.Context),
		Description: new(cfg.Desc),
		TargetURL:   new(cfg.TargetURL),
	})
	return err
}

type Installation struct {
	ID           int64
	AccountLogin string
}

func (c *Client) ListUserInstallations(ctx context.Context) ([]Installation, error) {
	ins, _, err := c.client.Apps.ListInstallations(ctx, nil)
	if err != nil {
		return nil, err
	}
	var out []Installation
	for _, i := range ins {
		login := ""
		if a := i.GetAccount(); a != nil {
			login = a.GetLogin()
		}
		out = append(out, Installation{ID: i.GetID(), AccountLogin: login})
	}
	return out, nil
}

func LoadPipelinesAtSHA(ctx context.Context, c *Client, owner, name, sha string) ([]*engine.Pipeline, error) {
	dir := defaultPipelineDir

	if rc, err := c.GetContentAtSHA(ctx, owner, name, ".jtrc", sha); err != nil {
		if cfg, perr := config.Parse([]byte(rc)); perr == nil && cfg.Pipelines.Dir != "" {
			dir = cfg.Pipelines.Dir
		}
	}

	tree, err := c.GetTreeAtSHA(ctx, owner, name, sha)
	if err != nil {
		return nil, err
	}
	pre := dir + "/"
	var pipelines []*engine.Pipeline
	for _, en := range tree {
		if en.Type != "blob" || !strings.HasPrefix(en.Path, pre) {
			continue
		}
		rel := strings.TrimPrefix(en.Path, pre)
		if strings.Contains(rel, "/") || !strings.HasSuffix(rel, ".json") {
			continue
		}
		cont, err := c.GetContentAtSHA(ctx, owner, name, en.Path, sha)
		if err != nil {
			log.Printf("github: skip pipeline %q: %v", en.Path, err)
			continue
		}
		p, err := engine.ProcessJSON(cont)
		if err != nil {
			log.Printf("github: skip pipeline %q: %v", en.Path, err)
			continue
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, nil
}
