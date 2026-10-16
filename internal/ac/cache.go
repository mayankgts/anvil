// Package ac implements the action cache: action digest → ActionResult.
package ac

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/protobuf/proto"

	"github.com/mayankgts/anvil/internal/cas"
	"github.com/mayankgts/anvil/internal/digest"
)

// ErrNotFound is returned on a cache miss, including entries whose
// outputs are no longer in the CAS.
var ErrNotFound = errors.New("action result not found")

// Cache stores ActionResults on disk and validates them against a CAS.
type Cache struct {
	dir    string
	tmpDir string
	cas    cas.Store
}

// New creates (if needed) and opens an action cache rooted at root.
func New(root string, store cas.Store) (*Cache, error) {
	c := &Cache{
		dir:    filepath.Join(root, "ac"),
		tmpDir: filepath.Join(root, "tmp"),
		cas:    store,
	}
	for _, dir := range []string{c.dir, c.tmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return c, nil
}

func (c *Cache) path(key digest.Digest) string {
	return filepath.Join(c.dir, key.Hash[:2], key.Hash)
}

// Get returns the result for key, but only if every output it references
// is still present in the CAS.
func (c *Cache) Get(ctx context.Context, key digest.Digest) (*repb.ActionResult, error) {
	data, err := os.ReadFile(c.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err != nil {
		return nil, err
	}

	res := &repb.ActionResult{}
	if err := proto.Unmarshal(data, res); err != nil {
		return nil, fmt.Errorf("%w: %s has a corrupt entry: %v", ErrNotFound, key, err)
	}

	refs, err := outputDigests(res)
	if err != nil {
		return nil, fmt.Errorf("%w: %s has an invalid output digest: %v", ErrNotFound, key, err)
	}
	missing, err := c.cas.FindMissing(ctx, refs)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s references %d missing blobs", ErrNotFound, key, len(missing))
	}
	return res, nil
}

// Put stores res under key, replacing any existing entry.
func (c *Cache) Put(ctx context.Context, key digest.Digest, res *repb.ActionResult) error {
	data, err := proto.Marshal(res)
	if err != nil {
		return fmt.Errorf("marshal result for %s: %w", key, err)
	}

	tmp, err := os.CreateTemp(c.tmpDir, "ac-*")
	if err != nil {
		return err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	final := c.path(key)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), final)
}

// outputDigests collects every CAS blob that res points to.
func outputDigests(res *repb.ActionResult) ([]digest.Digest, error) {
	var out []digest.Digest
	add := func(p *repb.Digest) error {
		d, err := digest.FromProto(p)
		if err != nil {
			return err
		}
		out = append(out, d)
		return nil
	}

	for _, f := range res.GetOutputFiles() {
		if err := add(f.GetDigest()); err != nil {
			return nil, err
		}
	}
	for _, d := range res.GetOutputDirectories() {
		if err := add(d.GetTreeDigest()); err != nil {
			return nil, err
		}
	}
	// stdout/stderr are optional: unset means there was no output to store.
	for _, p := range []*repb.Digest{res.GetStdoutDigest(), res.GetStderrDigest()} {
		if p != nil {
			if err := add(p); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
