package server

import (
	"context"

	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"github.com/bazelbuild/remote-apis/build/bazel/semver"
)

const (
	// maxBatchSize is the largest total payload accepted by batch calls.
	maxBatchSize = 4 << 20 // 4 MiB

	// MaxRecvMsgSize leaves headroom over maxBatchSize for protobuf framing.
	MaxRecvMsgSize = maxBatchSize + (1 << 20)
)

// CapabilitiesServer tells clients what this server supports.
type CapabilitiesServer struct {
	repb.UnimplementedCapabilitiesServer
}

func (CapabilitiesServer) GetCapabilities(ctx context.Context, _ *repb.GetCapabilitiesRequest) (*repb.ServerCapabilities, error) {
	return &repb.ServerCapabilities{
		CacheCapabilities: &repb.CacheCapabilities{
			DigestFunctions:               []repb.DigestFunction_Value{repb.DigestFunction_SHA256},
			ActionCacheUpdateCapabilities: &repb.ActionCacheUpdateCapabilities{UpdateEnabled: true},
			MaxBatchTotalSizeBytes:        maxBatchSize,
			SymlinkAbsolutePathStrategy:   repb.SymlinkAbsolutePathStrategy_DISALLOWED,
		},
		LowApiVersion:  &semver.SemVer{Major: 2},
		HighApiVersion: &semver.SemVer{Major: 2},
	}, nil
}
