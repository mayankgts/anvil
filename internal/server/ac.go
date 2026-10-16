package server

import (
	"context"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mayankgts/anvil/internal/ac"
	"github.com/mayankgts/anvil/internal/digest"
)

// ACServer serves the ActionCache service.
type ACServer struct {
	repb.UnimplementedActionCacheServer
	Cache *ac.Cache
}

func (s *ACServer) GetActionResult(ctx context.Context, req *repb.GetActionResultRequest) (*repb.ActionResult, error) {
	key, err := digest.FromProto(req.GetActionDigest())
	if err != nil {
		return nil, toStatus(err)
	}
	res, err := s.Cache.Get(ctx, key)
	if err != nil {
		return nil, toStatus(err)
	}
	return res, nil
}

func (s *ACServer) UpdateActionResult(ctx context.Context, req *repb.UpdateActionResultRequest) (*repb.ActionResult, error) {
	key, err := digest.FromProto(req.GetActionDigest())
	if err != nil {
		return nil, toStatus(err)
	}
	res := req.GetActionResult()
	if res == nil {
		return nil, status.Error(codes.InvalidArgument, "missing action_result")
	}
	if err := s.Cache.Put(ctx, key, res); err != nil {
		return nil, toStatus(err)
	}
	return res, nil
}
