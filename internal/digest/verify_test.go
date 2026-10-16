package digest

import (
	"errors"
	"testing"
)

func TestVerifier(t *testing.T) {
	hello := Digest{Hash: helloHash, Size: 5}
	empty := Digest{Hash: emptyHash, Size: 0}

	tests := []struct {
		name    string
		want    Digest
		chunks  []string
		wantErr bool
	}{
		{"one write", hello, []string{"hello"}, false},
		{"chunked", hello, []string{"he", "l", "lo"}, false},
		{"empty blob", empty, nil, false},
		{"wrong content same size", hello, []string{"jello"}, true},
		{"too short", hello, []string{"hell"}, true},
		{"too long", hello, []string{"hello", "!"}, true},
		{"data for empty blob", empty, []string{"x"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := NewVerifier(tc.want)
			var err error
			for _, c := range tc.chunks {
				if _, err = v.Write([]byte(c)); err != nil {
					break
				}
			}
			if err == nil {
				err = v.Check()
			}

			if tc.wantErr {
				if !errors.Is(err, ErrMismatch) {
					t.Fatalf("error = %v, want ErrMismatch", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
