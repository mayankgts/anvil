// Command anvil runs a Remote Execution API cache server.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc"

	"github.com/mayankgts/anvil/internal/ac"
	"github.com/mayankgts/anvil/internal/cas"
	"github.com/mayankgts/anvil/internal/server"
)

func main() {
	addr := flag.String("addr", "localhost:9092", "address to listen on")
	dir := flag.String("dir", "anvil-data", "directory for stored data")
	flag.Parse()

	if err := run(*addr, *dir); err != nil {
		slog.Error("anvil stopped", "err", err)
		os.Exit(1)
	}
}

func run(addr, dir string) error {
	store, err := cas.NewDiskStore(dir)
	if err != nil {
		return err
	}
	cache, err := ac.New(dir, store)
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	srv := grpc.NewServer(grpc.MaxRecvMsgSize(server.MaxRecvMsgSize))
	repb.RegisterCapabilitiesServer(srv, server.CapabilitiesServer{})
	repb.RegisterContentAddressableStorageServer(srv, &server.CASServer{Store: store})
	repb.RegisterActionCacheServer(srv, &server.ACServer{Cache: cache})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		slog.Info("shutting down")
		srv.GracefulStop()
	}()

	slog.Info("anvil listening", "addr", lis.Addr().String(), "dir", dir)
	return srv.Serve(lis)
}
