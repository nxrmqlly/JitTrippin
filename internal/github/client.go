package github

import (
	"context"

	gh "github.com/google/go-github/v90/github"
)

type Client struct {
	client *gh.Client
}

func NewClient() *Client {
	return &Client{
		// client: gh.NewClient(),
	}
}

type Repository struct {
	ID            string
	FullName      string
	DefaultBranch string
}

func (c *Client) FetchRepository(ctx context.Context, owner string, repo string) (*Repository, error) {
	// repo, resp, err := c.client.Repositories.Get(ctx, owner, repo)
	return nil, nil
}
