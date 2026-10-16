package server

import (
	"bytes"
	"context"
	"io"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mayankgts/anvil/internal/cas"
	"github.com/mayankgts/anvil/internal/digest"
)

// CASServer serves the ContentAddressableStorage service.
type CASServer struct {
	repb.UnimplementedContentAddressableStorageServer
	Store cas.Store
}

func (s *CASServer) FindMissingBlobs(ctx context.Context, req *repb.FindMissingBlobsRequest) (*repb.FindMissingBlobsResponse, error) {
	ds := make([]digest.Digest, 0, len(req.GetBlobDigests()))
	for _, p := range req.GetBlobDigests() {
		d, err := digest.FromProto(p)
		if err != nil {
			return nil, toStatus(err)
		}
		ds = append(ds, d)
	}

	missing, err := s.Store.FindMissing(ctx, ds)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &repb.FindMissingBlobsResponse{}
	for _, d := range missing {
		resp.MissingBlobDigests = append(resp.MissingBlobDigests, d.ToProto())
	}
	return resp, nil
}

func (s *CASServer) BatchUpdateBlobs(ctx context.Context, req *repb.BatchUpdateBlobsRequest) (*repb.BatchUpdateBlobsResponse, error) {
	var total int64
	for _, r := range req.GetRequests() {
		total += int64(len(r.GetData()))
	}
	if total > maxBatchSize {
		return nil, status.Errorf(codes.InvalidArgument, "batch of %d bytes exceeds limit of %d", total, maxBatchSize)
	}

	resp := &repb.BatchUpdateBlobsResponse{}
	for _, r := range req.GetRequests() {
		resp.Responses = append(resp.Responses, &repb.BatchUpdateBlobsResponse_Response{
			Digest: r.GetDigest(),
			Status: rpcStatus(s.putOne(ctx, r)).Proto(),
		})
	}
	return resp, nil
}

func (s *CASServer) putOne(ctx context.Context, r *repb.BatchUpdateBlobsRequest_Request) error {
	if r.GetCompressor() != repb.Compressor_IDENTITY {
		return status.Error(codes.InvalidArgument, "compression is not supported")
	}
	d, err := digest.FromProto(r.GetDigest())
	if err != nil {
		return err
	}
	return s.Store.Put(ctx, d, bytes.NewReader(r.GetData()))
}

func (s *CASServer) BatchReadBlobs(ctx context.Context, req *repb.BatchReadBlobsRequest) (*repb.BatchReadBlobsResponse, error) {
	// Validate everything first so the size limit is computed from trusted values.
	ds := make([]digest.Digest, 0, len(req.GetDigests()))
	var total int64
	for _, p := range req.GetDigests() {
		d, err := digest.FromProto(p)
		if err != nil {
			return nil, toStatus(err)
		}
		ds = append(ds, d)
		total += d.Size
	}
	if total > maxBatchSize {
		return nil, status.Errorf(codes.InvalidArgument, "batch of %d bytes exceeds limit of %d", total, maxBatchSize)
	}

	resp := &repb.BatchReadBlobsResponse{}
	for _, d := range ds {
		data, err := s.readOne(ctx, d)
		resp.Responses = append(resp.Responses, &repb.BatchReadBlobsResponse_Response{
			Digest: d.ToProto(),
			Data:   data,
			Status: rpcStatus(err).Proto(),
		})
	}
	return resp, nil
}

func (s *CASServer) readOne(ctx context.Context, d digest.Digest) ([]byte, error) {
	rc, err := s.Store.Get(ctx, d)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
