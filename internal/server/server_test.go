package server

import (
	"bytes"
	"context"
	"testing"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mayankgts/anvil/internal/ac"
	"github.com/mayankgts/anvil/internal/cas"
	"github.com/mayankgts/anvil/internal/digest"
)

func setup(t *testing.T) (*CASServer, *ACServer) {
	t.Helper()
	root := t.TempDir()
	store, err := cas.NewDiskStore(root)
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}
	cache, err := ac.New(root, store)
	if err != nil {
		t.Fatalf("ac.New: %v", err)
	}
	return &CASServer{Store: store}, &ACServer{Cache: cache}
}

func TestBatchUpdateThenReadAndFind(t *testing.T) {
	casSrv, _ := setup(t)
	ctx := context.Background()
	data := []byte("hello")
	d := digest.FromBytes(data).ToProto()

	up, err := casSrv.BatchUpdateBlobs(ctx, &repb.BatchUpdateBlobsRequest{
		Requests: []*repb.BatchUpdateBlobsRequest_Request{{Digest: d, Data: data}},
	})
	if err != nil {
		t.Fatalf("BatchUpdateBlobs: %v", err)
	}
	if c := codes.Code(up.GetResponses()[0].GetStatus().GetCode()); c != codes.OK {
		t.Fatalf("upload status = %v, want OK", c)
	}

	rd, err := casSrv.BatchReadBlobs(ctx, &repb.BatchReadBlobsRequest{Digests: []*repb.Digest{d}})
	if err != nil {
		t.Fatalf("BatchReadBlobs: %v", err)
	}
	if got := rd.GetResponses()[0].GetData(); !bytes.Equal(got, data) {
		t.Errorf("read data = %q, want %q", got, data)
	}

	fm, err := casSrv.FindMissingBlobs(ctx, &repb.FindMissingBlobsRequest{BlobDigests: []*repb.Digest{d}})
	if err != nil {
		t.Fatalf("FindMissingBlobs: %v", err)
	}
	if n := len(fm.GetMissingBlobDigests()); n != 0 {
		t.Errorf("FindMissingBlobs returned %d missing, want 0", n)
	}
}

func TestBatchUpdateMismatch(t *testing.T) {
	casSrv, _ := setup(t)
	d := digest.FromBytes([]byte("hello")).ToProto()

	up, err := casSrv.BatchUpdateBlobs(context.Background(), &repb.BatchUpdateBlobsRequest{
		Requests: []*repb.BatchUpdateBlobsRequest_Request{{Digest: d, Data: []byte("jello")}},
	})
	if err != nil {
		t.Fatalf("BatchUpdateBlobs: %v", err)
	}
	if c := codes.Code(up.GetResponses()[0].GetStatus().GetCode()); c != codes.InvalidArgument {
		t.Errorf("upload status = %v, want InvalidArgument", c)
	}
}

func TestFindMissingInvalidDigest(t *testing.T) {
	casSrv, _ := setup(t)
	_, err := casSrv.FindMissingBlobs(context.Background(), &repb.FindMissingBlobsRequest{
		BlobDigests: []*repb.Digest{{Hash: "../../evil", SizeBytes: 5}},
	})
	if c := status.Code(err); c != codes.InvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", c)
	}
}

func TestBatchReadNegativeSizeRejected(t *testing.T) {
	casSrv, _ := setup(t)
	d := digest.FromBytes([]byte("hello"))
	_, err := casSrv.BatchReadBlobs(context.Background(), &repb.BatchReadBlobsRequest{
		Digests: []*repb.Digest{{Hash: d.Hash, SizeBytes: -1 << 30}},
	})
	if c := status.Code(err); c != codes.InvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", c)
	}
}

func TestGetActionResultMiss(t *testing.T) {
	_, acSrv := setup(t)
	key := digest.FromBytes([]byte("some action")).ToProto()
	_, err := acSrv.GetActionResult(context.Background(), &repb.GetActionResultRequest{ActionDigest: key})
	if c := status.Code(err); c != codes.NotFound {
		t.Errorf("error code = %v, want NotFound", c)
	}
}
