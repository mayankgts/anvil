package digest

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ResourceName is a parsed ByteStream resource name.
type ResourceName struct {
	Instance string // may be empty
	UploadID string // set only for write resources
	Digest   Digest
}

// ParseReadResource parses "{instance}/blobs/{hash}/{size}[/anything]".
func ParseReadResource(name string) (ResourceName, error) {
	parts := strings.Split(name, "/")
	i := slices.Index(parts, "blobs")
	if i < 0 || len(parts) < i+3 {
		return ResourceName{}, fmt.Errorf("%w: read resource %q must contain blobs/{hash}/{size}", ErrInvalid, name)
	}
	d, err := parseHashSize(parts[i+1], parts[i+2])
	if err != nil {
		return ResourceName{}, err
	}
	return ResourceName{Instance: strings.Join(parts[:i], "/"), Digest: d}, nil
}

// ParseWriteResource parses "{instance}/uploads/{uuid}/blobs/{hash}/{size}[/anything]".
func ParseWriteResource(name string) (ResourceName, error) {
	parts := strings.Split(name, "/")
	u := slices.Index(parts, "uploads")
	if u < 0 || len(parts) < u+5 || parts[u+2] != "blobs" {
		return ResourceName{}, fmt.Errorf("%w: write resource %q must contain uploads/{uuid}/blobs/{hash}/{size}", ErrInvalid, name)
	}
	if parts[u+1] == "" {
		return ResourceName{}, fmt.Errorf("%w: write resource %q has an empty upload id", ErrInvalid, name)
	}
	d, err := parseHashSize(parts[u+3], parts[u+4])
	if err != nil {
		return ResourceName{}, err
	}
	return ResourceName{Instance: strings.Join(parts[:u], "/"), UploadID: parts[u+1], Digest: d}, nil
}

// parseHashSize converts the hash and size segments into a validated Digest.
func parseHashSize(hash, sizeStr string) (Digest, error) {
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil || strconv.FormatInt(size, 10) != sizeStr {
		return Digest{}, fmt.Errorf("%w: size %q is not a canonical decimal integer", ErrInvalid, sizeStr)
	}
	return New(hash, size)
}
