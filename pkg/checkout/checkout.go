package checkout

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
)

type Checkout struct {
	URL      string `json:"url,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Username string `json:"-"`
	Password string `json:"-"`
}

// AuthURL generates the git repository URL for cloning with Username and Password added
// to it.
//
//	Format:
//	https://x-access-token:tok@github.com/nxrmqlly/JitTrippin.git
func (c Checkout) AuthURL() (string, error) {
	if c.Username == "" || c.Password == "" {
		return c.URL, nil
	}
	raw := c.URL
	if !strings.Contains(raw, "://") {
		if i := strings.Index(raw, "@"); i > 0 {
			if host, path, ok := strings.Cut(raw[i+1:], ":"); ok && host != "" && path != "" {
				raw = "https://" + host + "/" + path
			}
		}
	}
	u, err := url.Parse(raw)
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
