// Package digest identifies blobs by their SHA-256 hash and size.
package digest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

// hashLen is the length of a SHA-256 hash in hex characters (32 bytes × 2).
const hashLen = 64

// Digest identifies a blob by its SHA-256 hash and size in bytes.
type Digest struct {
	Hash string
	Size int64
}

// ErrInvalid is returned when a hash or size is malformed.
var ErrInvalid = errors.New("invalid digest")
var Empty = FromBytes(nil)

// New validates hash and size and returns a Digest.
// hash must be exactly 64 lowercase hex characters; size must be >= 0.
func New(hash string, size int64) (Digest, error) {
	if len(hash) != hashLen {
		return Digest{}, fmt.Errorf("%w: hash must be %d hex chars, got %d", ErrInvalid, hashLen, len(hash))
	}
	for i := 0; i < len(hash); i++ {
		c := hash[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return Digest{}, fmt.Errorf("%w: invalid char %q at index %d (want lowercase hex)", ErrInvalid, c, i)
		}
	}
	if size < 0 {
		return Digest{}, fmt.Errorf("%w: size must be >= 0, got %d", ErrInvalid, size)
	}
	return Digest{Hash: hash, Size: size}, nil
}

// FromBytes computes the digest of data.
func FromBytes(data []byte) Digest {
	sum := sha256.Sum256(data)
	return Digest{Hash: hex.EncodeToString(sum[:]), Size: int64(len(data))}
}

// String returns "hash/size", the form used in logs and resource names.
func (d Digest) String() string {
	return fmt.Sprintf("%s/%d", d.Hash, d.Size)
}
