package ac

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/protobuf/proto"

	"github.com/mayankgts/anvil/internal/cas"
	"github.com/mayankgts/anvil/internal/digest"
)

var actionKey = digest.FromBytes([]byte("fake serialized action"))

func setup(t *testing.T) (*Cache, *cas.DiskStore) {
	t.Helper()
	root := t.TempDir()
	store, err := cas.NewDiskStore(root)
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	c, err := New(root, store)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, store
}

func putBlob(t *testing.T, store cas.Store, data string) digest.Digest {
	t.Helper()
	d := digest.FromBytes([]byte(data))
	if err := store.Put(context.Background(), d, bytes.NewReader([]byte(data))); err != nil {
		t.Fatalf("Put blob: %v", err)
	}
	return d
}

func TestGetMiss(t *testing.T) {
	c, _ := setup(t)
	if _, err := c.Get(context.Background(), actionKey); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound", err)
	}
}

func TestPutThenGet(t *testing.T) {
	c, store := setup(t)
	out := putBlob(t, store, "compiled object code")
	res := &repb.ActionResult{
		ExitCode:    0,
		OutputFiles: []*repb.OutputFile{{Path: "out/foo.o", Digest: out.ToProto()}},
	}

	if err := c.Put(context.Background(), actionKey, res); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := c.Get(context.Background(), actionKey)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !proto.Equal(got, res) {
		t.Errorf("Get = %v, want %v", got, res)
	}
}

func TestGetWithMissingOutputIsMiss(t *testing.T) {
	c, _ := setup(t)
	never := digest.FromBytes([]byte("never uploaded"))
	res := &repb.ActionResult{
		OutputFiles: []*repb.OutputFile{{Path: "out/foo.o", Digest: never.ToProto()}},
	}

	if err := c.Put(context.Background(), actionKey, res); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := c.Get(context.Background(), actionKey); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound for result with missing output", err)
	}
}

func TestPutOverwrites(t *testing.T) {
	c, _ := setup(t)
	for _, code := range []int32{0, 1} {
		if err := c.Put(context.Background(), actionKey, &repb.ActionResult{ExitCode: code}); err != nil {
			t.Fatalf("Put(exit %d): %v", code, err)
		}
	}
	got, err := c.Get(context.Background(), actionKey)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.GetExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1 (latest Put wins)", got.GetExitCode())
	}
}

func TestCorruptEntryIsMiss(t *testing.T) {
	c, _ := setup(t)
	p := c.path(actionKey)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("\xff\xff\xff"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), actionKey); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound for corrupt entry", err)
	}
}
