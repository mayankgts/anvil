// Package server implements the Remote Execution API gRPC services.
package server

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mayankgts/anvil/internal/ac"
	"github.com/mayankgts/anvil/internal/cas"
	"github.com/mayankgts/anvil/internal/digest"
)

// toStatus converts an internal error into a gRPC status error.
func toStatus(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err // already a gRPC status
	}
	var code codes.Code
	switch {
	case errors.Is(err, digest.ErrInvalid), errors.Is(err, digest.ErrMismatch):
		code = codes.InvalidArgument
	case errors.Is(err, cas.ErrNotFound), errors.Is(err, ac.ErrNotFound):
		code = codes.NotFound
	case errors.Is(err, context.Canceled):
		code = codes.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		code = codes.DeadlineExceeded
	default:
		code = codes.Internal
	}
	return status.Error(code, err.Error())
}

// rpcStatus returns the status to embed in a per-blob batch response.
func rpcStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}
	return status.Convert(toStatus(err))
}
