package github

import (
	"context"
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v90/github"
	"github.com/nxrmqlly/jittrippin/internal/config"
	"github.com/nxrmqlly/jittrippin/pkg/engine"
	"golang.org/x/oauth2"
)

const defaultPipelineDir = ".jt"

type Installation struct {
	ID           int64
	AccountLogin string
	AccountType  string
}

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
		return "", err
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

func (c *Client) ListUserInstallations(ctx context.Context) ([]Installation, error) {
	ins, _, err := c.client.Apps.ListUserInstallations(ctx, nil)
	if err != nil {
		return nil, err
	}
	var out []Installation
	for _, i := range ins {
		login := ""
		typ := ""
		if a := i.GetAccount(); a != nil {
			login = a.GetLogin()
			typ = a.GetType()
		}
		out = append(out, Installation{ID: i.GetID(), AccountLogin: login, AccountType: typ})
	}
	return out, nil
}

func LoadPipelinesAtSHA(ctx context.Context, c *Client, owner, name, sha string) ([]*engine.Pipeline, error) {
	dir := defaultPipelineDir

	if rc, err := c.GetContentAtSHA(ctx, owner, name, ".jtrc", sha); err == nil {
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

func (c *Client) GetInstallation(ctx context.Context, id int64) (*Installation, error) {
	inst, _, err := c.client.Apps.GetInstallation(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &Installation{ID: inst.GetID()}
	if a := inst.GetAccount(); a != nil {
		out.AccountLogin = a.GetLogin()
		out.AccountType = a.GetType()
	}
	return out, nil
}

func (c *Client) ListBranches(ctx context.Context, owner, name string) ([]string, error) {
	var branches []string
	opts := &gh.BranchListOptions{ListOptions: gh.ListOptions{
		PerPage: 100,
	}}
	for {
		bs, resp, err := c.client.Repositories.ListBranches(ctx, owner, name, opts)
		if err != nil {
			return nil, err
		}
		for _, b := range bs {
			branches = append(branches, b.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return branches, nil
}

func (c *Client) ListInstallationRepos(ctx context.Context) ([]Repository, error) {
	opts := &gh.ListOptions{PerPage: 100}
	var out []Repository
	for {
		repos, resp, err := c.client.Apps.ListRepos(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, r := range repos.Repositories {
			out = append(out, Repository{
				ID:            strconv.FormatInt(r.GetID(), 10),
				FullName:      r.GetFullName(),
				DefaultBranch: r.GetDefaultBranch(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func (c *Client) ResolveTagSHA(ctx context.Context, owner, name, tag string) (string, error) {
	ref, _, err := c.client.Git.GetRef(ctx, owner, name, "tags/"+tag)
	if err != nil {
		return "", err
	}
	sha := ref.GetObject().GetSHA()
	if ref.GetObject().GetType() == "tag" { // annotated tag: peel to commit
		tagObj, _, err := c.client.Git.GetTag(ctx, owner, name, sha)
		if err != nil {
			return "", err
		}
		sha = tagObj.GetObject().GetSHA()
	}
	return sha, nil
}
func (c *Client) ListReleaseAssets(ctx context.Context, owner, name string, releaseID int64) ([]*gh.ReleaseAsset, error) {
	var out []*gh.ReleaseAsset
	opts := &gh.ListOptions{PerPage: 100}
	for {
		assets, resp, err := c.client.Repositories.ListReleaseAssets(ctx, owner, name, releaseID, opts)
		if err != nil {
			return nil, err
		}
		out = append(out, assets...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

func (c *Client) DeleteReleaseAsset(ctx context.Context, owner, name string, assetID int64) error {
	_, err := c.client.Repositories.DeleteReleaseAsset(ctx, owner, name, assetID)
	return err
}

func (c *Client) UploadReleaseAsset(ctx context.Context, owner, name string, releaseID int64, assetName string, file *os.File) (*gh.ReleaseAsset, error) {
	opts := &gh.UploadOptions{Name: assetName}
	if mt := mime.TypeByExtension(filepath.Ext(assetName)); mt != "" {
		opts.MediaType = mt
	}
	asset, _, err := c.client.Repositories.UploadReleaseAsset(ctx, owner, name, releaseID, opts, file)
	if err != nil {
		return nil, err
	}
	return asset, nil
}
