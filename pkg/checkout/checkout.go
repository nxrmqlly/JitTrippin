package checkout

import (
	"context"
	"fmt"
	"io"
	"net/url"
)

type Checkout struct {
	URL      string `json:"url,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Username string `json:"-"`
	Password string `json:"-"`
}

func (c Checkout) AuthURL() (string, error) {
	if c.Username == "" || c.Password == "" {
		return c.URL, nil
	}
	u, err := url.Parse(c.URL)
	if err != nil {
		return "", fmt.Errorf("cannot parse checkout url %q: %w", c.URL, err)
	}
	u.User = url.UserPassword(c.Username, c.Password)
	return u.String(), nil
}

type Checkouter interface {
	// Checkout clones cfg.URL and checks out to cfg.Ref if set
	// and streams the resulting working tree as a tar stream.
	// The caller must read till completion or close the returned ReadCloser.
	//
	// ctx controls the lifetime of all underlying processes so it must be valid until
	// the returned ReadCloser has been fully read or closed.
	Checkout(ctx context.Context, c Checkout) (io.ReadCloser, error)
}
