package server

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"

	bspb "google.golang.org/genproto/googleapis/bytestream"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/mayankgts/anvil/internal/cas"
	"github.com/mayankgts/anvil/internal/digest"
)

// newByteStreamClient starts an in-memory gRPC server and returns a client for it.
func newByteStreamClient(t *testing.T) bspb.ByteStreamClient {
	t.Helper()
	store, err := cas.NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewDiskStore: %v", err)
	}

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	bspb.RegisterByteStreamServer(srv, &ByteStreamServer{Store: store})
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return bspb.NewByteStreamClient(conn)
}

func readName(d digest.Digest) string  { return "blobs/" + d.String() }
func writeName(d digest.Digest) string { return "uploads/test-upload/blobs/" + d.String() }

// chunks builds a correct sequence of WriteRequests for parts.
func chunks(name string, parts ...string) []*bspb.WriteRequest {
	var reqs []*bspb.WriteRequest
	var off int64
	for i, p := range parts {
		reqs = append(reqs, &bspb.WriteRequest{
			ResourceName: name,
			WriteOffset:  off,
			Data:         []byte(p),
			FinishWrite:  i == len(parts)-1,
		})
		off += int64(len(p))
	}
	return reqs
}

func send(t *testing.T, c bspb.ByteStreamClient, reqs []*bspb.WriteRequest) (*bspb.WriteResponse, error) {
	t.Helper()
	stream, err := c.Write(context.Background())
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	for _, r := range reqs {
		if err := stream.Send(r); err != nil {
			break // server ended the stream early; CloseAndRecv reports why
		}
	}
	return stream.CloseAndRecv()
}

func download(c bspb.ByteStreamClient, req *bspb.ReadRequest) ([]byte, error) {
	stream, err := c.Read(context.Background(), req)
	if err != nil {
		return nil, err
	}
	var out []byte
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, resp.GetData()...)
	}
}

func TestWriteThenRead(t *testing.T) {
	c := newByteStreamClient(t)
	data := []byte("hello world")
	d := digest.FromBytes(data)

	resp, err := send(t, c, chunks(writeName(d), "hello", " ", "world"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.GetCommittedSize() != d.Size {
		t.Errorf("CommittedSize = %d, want %d", resp.GetCommittedSize(), d.Size)
	}

	got, err := download(c, &bspb.ReadRequest{ResourceName: readName(d)})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("download = %q, want %q", got, data)
	}
}

func TestReadOffsetAndLimit(t *testing.T) {
	c := newByteStreamClient(t)
	d := digest.FromBytes([]byte("hello world"))
	if _, err := send(t, c, chunks(writeName(d), "hello world")); err != nil {
		t.Fatalf("upload: %v", err)
	}

	got, err := download(c, &bspb.ReadRequest{ResourceName: readName(d), ReadOffset: 6, ReadLimit: 3})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(got) != "wor" {
		t.Errorf("download = %q, want %q", got, "wor")
	}
}

func TestReadErrors(t *testing.T) {
	c := newByteStreamClient(t)
	d := digest.FromBytes([]byte("hello"))
	if _, err := send(t, c, chunks(writeName(d), "hello")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	never := digest.FromBytes([]byte("never uploaded"))

	tests := []struct {
		name string
		req  *bspb.ReadRequest
		want codes.Code
	}{
		{"missing blob", &bspb.ReadRequest{ResourceName: readName(never)}, codes.NotFound},
		{"offset past end", &bspb.ReadRequest{ResourceName: readName(d), ReadOffset: 100}, codes.OutOfRange},
		{"negative offset", &bspb.ReadRequest{ResourceName: readName(d), ReadOffset: -1}, codes.OutOfRange},
		{"bad resource name", &bspb.ReadRequest{ResourceName: "nonsense"}, codes.InvalidArgument},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := download(c, tc.req)
			if got := status.Code(err); got != tc.want {
				t.Errorf("error code = %v, want %v (err: %v)", got, tc.want, err)
			}
		})
	}
}

func TestWriteMismatchRejected(t *testing.T) {
	c := newByteStreamClient(t)
	d := digest.FromBytes([]byte("hello"))
	_, err := send(t, c, chunks(writeName(d), "jello"))
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", got)
	}
}

func TestWriteBadOffsetRejected(t *testing.T) {
	c := newByteStreamClient(t)
	d := digest.FromBytes([]byte("hello"))
	reqs := chunks(writeName(d), "he", "llo")
	reqs[1].WriteOffset = 7 // gap: bytes 2–6 never sent

	_, err := send(t, c, reqs)
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", got)
	}
}

func TestWriteExistingBlobReturnsEarly(t *testing.T) {
	c := newByteStreamClient(t)
	d := digest.FromBytes([]byte("hello"))
	for i := 0; i < 2; i++ {
		resp, err := send(t, c, chunks(writeName(d), "hel", "lo"))
		if err != nil {
			t.Fatalf("upload #%d: %v", i+1, err)
		}
		if resp.GetCommittedSize() != d.Size {
			t.Errorf("upload #%d CommittedSize = %d, want %d", i+1, resp.GetCommittedSize(), d.Size)
		}
	}
}

func TestQueryWriteStatus(t *testing.T) {
	c := newByteStreamClient(t)
	d := digest.FromBytes([]byte("hello"))
	query := func() *bspb.QueryWriteStatusResponse {
		resp, err := c.QueryWriteStatus(context.Background(), &bspb.QueryWriteStatusRequest{ResourceName: writeName(d)})
		if err != nil {
			t.Fatalf("QueryWriteStatus: %v", err)
		}
		return resp
	}

	if query().GetComplete() {
		t.Errorf("Complete = true before upload, want false")
	}
	if _, err := send(t, c, chunks(writeName(d), "hello")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if !query().GetComplete() {
		t.Errorf("Complete = false after upload, want true")
	}
}
