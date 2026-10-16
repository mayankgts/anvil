package cas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mayankgts/anvil/internal/digest"
)

// DiskStore stores each blob as a file named by its hash.
type DiskStore struct {
	blobDir string
	tmpDir  string
}

// Compile-time check that DiskStore implements Store.
var _ Store = (*DiskStore)(nil)

// NewDiskStore creates (if needed) and opens a store rooted at root.
func NewDiskStore(root string) (*DiskStore, error) {
	s := &DiskStore{
		blobDir: filepath.Join(root, "cas"),
		tmpDir:  filepath.Join(root, "tmp"),
	}
	for _, dir := range []string{s.blobDir, s.tmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return s, nil
}

// path returns where blob d lives, e.g. cas/2c/2cf24d...
func (s *DiskStore) path(d digest.Digest) string {
	return filepath.Join(s.blobDir, d.Hash[:2], d.Hash)
}

// has reports whether a complete copy of d is stored.
func (s *DiskStore) has(d digest.Digest) (bool, error) {
	if d == digest.Empty {
		return true, nil
	}
	fi, err := os.Stat(s.path(d))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return fi.Size() == d.Size, nil
}

func (s *DiskStore) FindMissing(ctx context.Context, ds []digest.Digest) ([]digest.Digest, error) {
	var missing []digest.Digest
	for _, d := range ds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ok, err := s.has(d)
		if err != nil {
			return nil, err
		}
		if !ok {
			missing = append(missing, d)
		}
	}
	return missing, nil
}

func (s *DiskStore) Get(ctx context.Context, d digest.Digest) (io.ReadCloser, error) {
	if d == digest.Empty {
		return io.NopCloser(strings.NewReader("")), nil
	}
	f, err := os.Open(s.path(d))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, d)
	}
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if fi.Size() != d.Size {
		f.Close()
		return nil, fmt.Errorf("%w: %s (stored size %d)", ErrNotFound, d, fi.Size())
	}
	return f, nil
}

func (s *DiskStore) Put(ctx context.Context, d digest.Digest, r io.Reader) error {
	ok, err := s.has(d)
	if err != nil {
		return err
	}
	if ok {
		return nil // already stored; content is identical by definition
	}

	tmp, err := os.CreateTemp(s.tmpDir, "upload-*")
	if err != nil {
		return err
	}
	// Runs on every return. After a successful rename the temp name no longer
	// exists, so Remove is a harmless no-op; on any failure it deletes the partial file.
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	v := digest.NewVerifier(d)
	if _, err := io.Copy(io.MultiWriter(v, tmp), r); err != nil {
		return fmt.Errorf("write %s: %w", d, err)
	}
	if err := v.Check(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	final := s.path(d)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		// A concurrent upload of the same blob may have finished first.
		if ok, _ := s.has(d); ok {
			return nil
		}
		return fmt.Errorf("commit %s: %w", d, err)
	}
	return nil
}
