package server

import (
	"context"
	"io"

	bspb "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mayankgts/anvil/internal/cas"
	"github.com/mayankgts/anvil/internal/digest"
)

// readChunkSize is how many bytes each Read response carries.
const readChunkSize = 64 << 10 // 64 KiB

// ByteStreamServer streams blobs of any size to and from the CAS.
type ByteStreamServer struct {
	bspb.UnimplementedByteStreamServer
	Store cas.Store
}

func (s *ByteStreamServer) Read(req *bspb.ReadRequest, stream bspb.ByteStream_ReadServer) error {
	res, err := digest.ParseReadResource(req.GetResourceName())
	if err != nil {
		return toStatus(err)
	}
	size, offset, limit := res.Digest.Size, req.GetReadOffset(), req.GetReadLimit()
	if offset < 0 || offset > size {
		return status.Errorf(codes.OutOfRange, "read offset %d is outside a blob of %d bytes", offset, size)
	}
	if limit < 0 {
		return status.Errorf(codes.InvalidArgument, "negative read limit %d", limit)
	}

	rc, err := s.Store.Get(stream.Context(), res.Digest)
	if err != nil {
		return toStatus(err)
	}
	defer rc.Close()

	if err := skip(rc, offset); err != nil {
		return toStatus(err)
	}
	var r io.Reader = rc
	if limit > 0 {
		r = io.LimitReader(rc, limit)
	}

	for {
		// A fresh buffer per chunk: gRPC may still hold the previous message after Send returns.
		buf := make([]byte, readChunkSize)
		n, err := r.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&bspb.ReadResponse{Data: buf[:n]}); sendErr != nil {
				return sendErr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return toStatus(err)
		}
	}
}

// skip advances r by n bytes, seeking when possible.
func skip(r io.Reader, n int64) error {
	if n == 0 {
		return nil
	}
	if sk, ok := r.(io.Seeker); ok {
		_, err := sk.Seek(n, io.SeekStart)
		return err
	}
	_, err := io.CopyN(io.Discard, r, n)
	return err
}

func (s *ByteStreamServer) Write(stream bspb.ByteStream_WriteServer) error {
	first, err := stream.Recv()
	if err == io.EOF {
		return status.Error(codes.InvalidArgument, "empty write stream")
	}
	if err != nil {
		return err
	}

	res, err := digest.ParseWriteResource(first.GetResourceName())
	if err != nil {
		return toStatus(err)
	}
	d, ctx := res.Digest, stream.Context()

	// Already stored: finish immediately without reading the rest.
	missing, err := s.Store.FindMissing(ctx, []digest.Digest{d})
	if err != nil {
		return toStatus(err)
	}
	if len(missing) == 0 {
		return stream.SendAndClose(&bspb.WriteResponse{CommittedSize: d.Size})
	}

	r := &writeReader{stream: stream, resource: first.GetResourceName()}
	if err := r.accept(first); err != nil {
		return err
	}
	if err := s.Store.Put(ctx, d, r); err != nil {
		return toStatus(err)
	}
	return stream.SendAndClose(&bspb.WriteResponse{CommittedSize: d.Size})
}

func (s *ByteStreamServer) QueryWriteStatus(ctx context.Context, req *bspb.QueryWriteStatusRequest) (*bspb.QueryWriteStatusResponse, error) {
	res, err := digest.ParseWriteResource(req.GetResourceName())
	if err != nil {
		return nil, toStatus(err)
	}
	missing, err := s.Store.FindMissing(ctx, []digest.Digest{res.Digest})
	if err != nil {
		return nil, toStatus(err)
	}
	if len(missing) == 0 {
		return &bspb.QueryWriteStatusResponse{CommittedSize: res.Digest.Size, Complete: true}, nil
	}
	// Partial uploads are not kept, so an interrupted client restarts from zero.
	return &bspb.QueryWriteStatusResponse{}, nil
}

// writeReader turns a stream of WriteRequests into an io.Reader,
// checking offsets and the finish_write flag as messages arrive.
type writeReader struct {
	stream   bspb.ByteStream_WriteServer
	resource string
	pending  []byte // data received but not yet read
	offset   int64  // total bytes received so far
	finished bool
}

func (r *writeReader) Read(p []byte) (int, error) {
	for len(r.pending) == 0 {
		if r.finished {
			return 0, io.EOF
		}
		req, err := r.stream.Recv()
		if err == io.EOF {
			return 0, status.Error(codes.InvalidArgument, "stream ended before finish_write")
		}
		if err != nil {
			return 0, err
		}
		if err := r.accept(req); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

// accept validates one WriteRequest and queues its data.
func (r *writeReader) accept(req *bspb.WriteRequest) error {
	if name := req.GetResourceName(); name != "" && name != r.resource {
		return status.Errorf(codes.InvalidArgument, "resource name changed mid-stream to %q", name)
	}
	if req.GetWriteOffset() != r.offset {
		return status.Errorf(codes.InvalidArgument, "write offset %d, expected %d", req.GetWriteOffset(), r.offset)
	}
	r.pending = req.GetData()
	r.offset += int64(len(r.pending))
	r.finished = req.GetFinishWrite()
	return nil
}
