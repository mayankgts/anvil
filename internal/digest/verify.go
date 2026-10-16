package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
)

// ErrMismatch is returned when streamed data does not match its declared digest.
var ErrMismatch = errors.New("digest mismatch")

// Verifier is an io.Writer that hashes data as it streams through and
// checks it against an expected digest.
type Verifier struct {
	want Digest
	h    hash.Hash
	n    int64
}

// NewVerifier returns a Verifier expecting data matching want.
func NewVerifier(want Digest) *Verifier {
	return &Verifier{want: want, h: sha256.New()}
}

// Write hashes p. It fails as soon as more bytes arrive than declared.
func (v *Verifier) Write(p []byte) (int, error) {
	if v.n+int64(len(p)) > v.want.Size {
		return 0, fmt.Errorf("%w: received more than the declared %d bytes", ErrMismatch, v.want.Size)
	}
	v.n += int64(len(p))
	v.h.Write(p) // hash.Hash.Write never returns an error
	return len(p), nil
}

// Check reports whether everything written matches the expected digest.
// Call it once, after the last Write.
func (v *Verifier) Check() error {
	if v.n != v.want.Size {
		return fmt.Errorf("%w: received %d bytes, want %d", ErrMismatch, v.n, v.want.Size)
	}
	if got := hex.EncodeToString(v.h.Sum(nil)); got != v.want.Hash {
		return fmt.Errorf("%w: got hash %s, want %s", ErrMismatch, got, v.want.Hash)
	}
	return nil
}
