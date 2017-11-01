package server

import (
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// UpdateRuntimeConfig updates the configuration of a running container.
func (s *Server) UpdateRuntimeConfig(ctx context.Context, req *pb.UpdateRuntimeConfigRequest) (*pb.UpdateRuntimeConfigResponse, error) {
	return &pb.UpdateRuntimeConfigResponse{}, nil
}
