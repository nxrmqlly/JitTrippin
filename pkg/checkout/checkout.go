package checkout

import (
	"context"
	"io"
)

type Checkout struct {
	URL string `json:"url,omitempty"`
	Ref string `json:"ref,omitempty"`
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