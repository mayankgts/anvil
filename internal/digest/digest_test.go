package digest

import (
	"errors"
	"strings"
	"testing"
)

// sha256("hello")
const helloHash = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

// sha256("") — the empty blob, which every CAS must handle.
const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		size    int64
		wantErr bool
	}{
		{"valid", helloHash, 5, false},
		{"zero size", emptyHash, 0, false},
		{"uppercase", strings.ToUpper(helloHash), 5, true},
		{"too short", helloHash[:63], 5, true},
		{"too long", helloHash + "a", 5, true},
		{"non-hex char", "g" + helloHash[1:], 5, true},
		{"empty hash", "", 0, true},
		{"negative size", helloHash, -1, true},
		{"path traversal", "../../" + helloHash[6:], 5, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, err := New(tc.hash, tc.size)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("New(%q, %d) error = %v, want ErrInvalid", tc.hash, tc.size, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("New(%q, %d) unexpected error: %v", tc.hash, tc.size, err)
			}
			if d.Hash != tc.hash || d.Size != tc.size {
				t.Errorf("New(%q, %d) = %+v, want hash and size unchanged", tc.hash, tc.size, d)
			}
		})
	}
}

func TestFromBytes(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want Digest
	}{
		{"hello", []byte("hello"), Digest{Hash: helloHash, Size: 5}},
		{"empty", []byte{}, Digest{Hash: emptyHash, Size: 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FromBytes(tc.data); got != tc.want {
				t.Errorf("FromBytes(%q) = %+v, want %+v", tc.data, got, tc.want)
			}
		})
	}
}

func TestString(t *testing.T) {
	d := Digest{Hash: helloHash, Size: 5}
	want := helloHash + "/5"
	if got := d.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
