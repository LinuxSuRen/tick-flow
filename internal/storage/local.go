// Package storage abstracts the binary payload store. P0 ships the local
// disk implementation; an S3 implementation joins in P2 (issue #15).
package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Local is the local filesystem storage rooted at a base directory.
type Local struct {
	Base string
}

// NewLocal builds a local storage, creating the base directory when
// missing.
func NewLocal(base string) (*Local, error) {
	if base == "" {
		return nil, errors.New("storage base directory is required")
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, err
	}
	return &Local{Base: base}, nil
}

// Save writes the payload to base/path. The path is server-generated
// (tenant/month/uuid); defense in depth rejects any traversal attempt
// anyway.
func (l *Local) Save(_ context.Context, path string, r io.Reader) (int64, error) {
	full, err := l.resolve(path)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, err
	}
	f, err := os.Create(full)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}

// Open streams a stored payload.
func (l *Local) Open(_ context.Context, path string) (io.ReadCloser, error) {
	full, err := l.resolve(path)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (l *Local) resolve(path string) (string, error) {
	clean := filepath.Clean("/" + path)
	full := filepath.Join(l.Base, clean)
	root := filepath.Clean(l.Base)
	if !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		return "", errors.New("path escapes the storage root")
	}
	return full, nil
}
