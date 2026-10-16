package cas

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mayankgts/anvil/internal/digest"
)

func newStore(t *testing.T) *DiskStore {
	t.Helper()
	s, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	return s
}

func get(t *testing.T, s *DiskStore, d digest.Digest) []byte {
	t.Helper()
	rc, err := s.Get(context.Background(), d)
	if err != nil {
		t.Fatalf("Get(%s): %v", d, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", d, err)
	}
	return data
}

func TestPutThenGet(t *testing.T) {
	s := newStore(t)
	data := []byte("hello")
	d := digest.FromBytes(data)

	if err := s.Put(context.Background(), d, bytes.NewReader(data)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got := get(t, s, d); !bytes.Equal(got, data) {
		t.Errorf("Get = %q, want %q", got, data)
	}
}

func TestGetMissing(t *testing.T) {
	s := newStore(t)
	d := digest.FromBytes([]byte("never stored"))
	if _, err := s.Get(context.Background(), d); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound", err)
	}
}

func TestEmptyBlobAlwaysPresent(t *testing.T) {
	s := newStore(t)
	if got := get(t, s, digest.Empty); len(got) != 0 {
		t.Errorf("Get(Empty) = %q, want empty", got)
	}
}

func TestPutMismatchStoresNothing(t *testing.T) {
	s := newStore(t)
	d := digest.FromBytes([]byte("hello"))

	err := s.Put(context.Background(), d, strings.NewReader("jello"))
	if !errors.Is(err, digest.ErrMismatch) {
		t.Fatalf("Put error = %v, want ErrMismatch", err)
	}
	if _, err := s.Get(context.Background(), d); !errors.Is(err, ErrNotFound) {
		t.Errorf("after failed Put, Get error = %v, want ErrNotFound", err)
	}
	entries, err := os.ReadDir(s.tmpDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("tmp dir has %d leftover files, want 0", len(entries))
	}
}

func TestPutTwice(t *testing.T) {
	s := newStore(t)
	data := []byte("hello")
	d := digest.FromBytes(data)
	for i := 0; i < 2; i++ {
		if err := s.Put(context.Background(), d, bytes.NewReader(data)); err != nil {
			t.Fatalf("Put #%d: %v", i+1, err)
		}
	}
}

func TestFindMissing(t *testing.T) {
	s := newStore(t)
	have := []byte("stored")
	dHave := digest.FromBytes(have)
	dMissing := digest.FromBytes([]byte("not stored"))
	if err := s.Put(context.Background(), dHave, bytes.NewReader(have)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	missing, err := s.FindMissing(context.Background(), []digest.Digest{dHave, dMissing, digest.Empty})
	if err != nil {
		t.Fatalf("FindMissing: %v", err)
	}
	if len(missing) != 1 || missing[0] != dMissing {
		t.Errorf("FindMissing = %v, want [%s]", missing, dMissing)
	}
}

func TestConcurrentPutSameBlob(t *testing.T) {
	s := newStore(t)
	data := bytes.Repeat([]byte("anvil"), 10000)
	d := digest.FromBytes(data)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Put(context.Background(), d, bytes.NewReader(data)); err != nil {
				t.Errorf("concurrent Put: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := get(t, s, d); !bytes.Equal(got, data) {
		t.Errorf("stored blob differs after concurrent puts")
	}
}
