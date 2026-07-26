package checkout

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

type GitCheckouter struct{}

func tarDir(ctx context.Context, d string, pw *io.PipeWriter) {
	defer os.RemoveAll(d)

	tw := tar.NewWriter(pw)

	if err := filepath.WalkDir(d, func(path string, de fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			return err
		}

		if de.IsDir() && de.Name() == ".git" {
			return filepath.SkipDir
		}

		relpath, err := filepath.Rel(d, path)
		if err != nil {
			return err
		}

		if relpath == "." {
			return nil // skip root
		}

		info, err := de.Info()
		if err != nil {
			return err
		}

		var link string
		if de.Type()&fs.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}

		header.Name = relpath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if de.Type().IsRegular() {
			if err := func() error {
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				if _, err := io.Copy(tw, file); err != nil {
					return err
				}
				return nil
			}(); err != nil {
				return err
			}
		}

		return nil

	}); err != nil {
		pw.CloseWithError(err)
	}

	if err := tw.Close(); err != nil {
		pw.CloseWithError(err)
	}
}

func (gc *GitCheckouter) Checkout(ctx context.Context, c Checkout) (io.ReadCloser, error) {
	d, err := os.MkdirTemp("", "jittrippin_checkout_*")
	if err != nil {
		return nil, fmt.Errorf("cannot create checkout dir: %w", err)
	}

	var stderr bytes.Buffer

	clone := exec.CommandContext(ctx, "git", "clone", c.URL, d)
	clone.Stderr = &stderr
	if err := clone.Run(); err != nil {
		os.RemoveAll(d)
		return nil, fmt.Errorf("git clone failed: %s: %w", stderr.String(), err)
	}

	if c.Ref != "" {
		stderr.Reset()

		checkout := exec.CommandContext(ctx, "git", "checkout", c.Ref)
		checkout.Dir = d
		checkout.Stderr = &stderr

		if err := checkout.Run(); err != nil {
			os.RemoveAll(d)
			return nil, fmt.Errorf("git checkout to '%s': %s: %w", c.Ref, stderr.String(), err)
		}
	}

	pr, pw := io.Pipe()

	go tarDir(ctx, d, pw)

	return pr, nil
}
