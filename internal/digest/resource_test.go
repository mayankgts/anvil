package digest

import (
	"errors"
	"testing"
)

func TestParseReadResource(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantInstance string
		wantErr      bool
	}{
		{"no instance", "blobs/" + helloHash + "/5", "", false},
		{"with instance", "main/blobs/" + helloHash + "/5", "main", false},
		{"nested instance", "team/prod/blobs/" + helloHash + "/5", "team/prod", false},
		{"trailing filename", "blobs/" + helloHash + "/5/out.txt", "", false},
		{"missing size", "blobs/" + helloHash, "", true},
		{"missing blobs keyword", "main/" + helloHash + "/5", "", true},
		{"bad hash", "blobs/xyz/5", "", true},
		{"non-numeric size", "blobs/" + helloHash + "/five", "", true},
		{"negative size", "blobs/" + helloHash + "/-1", "", true},
		{"leading zero size", "blobs/" + helloHash + "/05", "", true},
		{"empty", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseReadResource(tc.in)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("ParseReadResource(%q) error = %v, want ErrInvalid", tc.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseReadResource(%q) unexpected error: %v", tc.in, err)
			}
			want := ResourceName{Instance: tc.wantInstance, Digest: Digest{Hash: helloHash, Size: 5}}
			if got != want {
				t.Errorf("ParseReadResource(%q) = %+v, want %+v", tc.in, got, want)
			}
		})
	}
}

func TestParseWriteResource(t *testing.T) {
	const id = "3f2a9c1e-uuid"
	tests := []struct {
		name         string
		in           string
		wantInstance string
		wantErr      bool
	}{
		{"no instance", "uploads/" + id + "/blobs/" + helloHash + "/5", "", false},
		{"with instance", "main/uploads/" + id + "/blobs/" + helloHash + "/5", "main", false},
		{"trailing metadata", "uploads/" + id + "/blobs/" + helloHash + "/5/meta", "", false},
		{"missing uuid", "uploads/blobs/" + helloHash + "/5", "", true},
		{"empty uuid", "uploads//blobs/" + helloHash + "/5", "", true},
		{"missing blobs keyword", "uploads/" + id + "/" + helloHash + "/5", "", true},
		{"read-style name", "blobs/" + helloHash + "/5", "", true},
		{"bad hash", "uploads/" + id + "/blobs/xyz/5", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseWriteResource(tc.in)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("ParseWriteResource(%q) error = %v, want ErrInvalid", tc.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseWriteResource(%q) unexpected error: %v", tc.in, err)
			}
			want := ResourceName{Instance: tc.wantInstance, UploadID: id, Digest: Digest{Hash: helloHash, Size: 5}}
			if got != want {
				t.Errorf("ParseWriteResource(%q) = %+v, want %+v", tc.in, got, want)
			}
		})
	}
}
