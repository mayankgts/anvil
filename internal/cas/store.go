package cas

import (
	"context"
	"errors"
	"io"

	"github.com/mayankgts/anvil/internal/digest"
)

// ErrNotFound is returned when a requested blob is not in the store.
var ErrNotFound = errors.New("blob not found")

// Store holds blobs keyed by digest. Implementations must be safe for
// concurrent use. Digests passed in must already be validated.
type Store interface {
	// FindMissing returns the subset of ds that the store does not have.
	FindMissing(ctx context.Context, ds []digest.Digest) ([]digest.Digest, error)
	// Get opens a blob for reading. The caller must close it.
	Get(ctx context.Context, d digest.Digest) (io.ReadCloser, error)
	// Put stores the data read from r, which must match d.
	Put(ctx context.Context, d digest.Digest, r io.Reader) error
}
