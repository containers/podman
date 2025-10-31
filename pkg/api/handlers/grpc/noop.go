//go:build !remote

package grpc

import (
	"context"

	"github.com/containers/podman/v6/libpod"
	"github.com/containers/podman/v6/pkg/api/grpcpb"
)

type noopServer struct {
	grpcpb.UnimplementedNoopServer
	runtime *libpod.Runtime
}

func (noopServer) Noop(_ context.Context, req *grpcpb.NoopRequest) (*grpcpb.NoopResponse, error) {
	resp := &grpcpb.NoopResponse{
		Ignored: req.GetIgnored(),
	}
	return resp, nil
}

func NewNoopServer(runtime *libpod.Runtime) grpcpb.NoopServer {
	return &noopServer{runtime: runtime}
}
