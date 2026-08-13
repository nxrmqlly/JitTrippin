package github

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/jferrl/go-githubauth"
	"golang.org/x/oauth2"
)

func loadAppTokenSrc(id string, pathOrPEM string) (oauth2.TokenSource, error) {
	raw := []byte(pathOrPEM)
	if data, err := os.ReadFile(pathOrPEM); err == nil {
		raw = data
	}
	appID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("GITHUB_APP_ID is invalid: %w", err)
	}
	return githubauth.NewApplicationTokenSource(appID, raw)
}

type installTokenSourceCache struct {
	mu sync.Mutex
	m  map[int64]oauth2.TokenSource
}

func newInstallTokenSourceCache() *installTokenSourceCache {
	return &installTokenSourceCache{m: make(map[int64]oauth2.TokenSource)}
}

func (c *installTokenSourceCache) Get(installID int64, appTS oauth2.TokenSource) oauth2.TokenSource {
	c.mu.Lock()
	defer c.mu.Unlock()

	ts, ok := c.m[installID]
	if !ok {
		ts = githubauth.NewInstallationTokenSource(installID, appTS)
		c.m[installID] = ts
	}
	return ts
}


