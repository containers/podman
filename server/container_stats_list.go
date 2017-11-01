package server

import (
	"fmt"

	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ListContainerStats returns stats of all running containers.
func (s *Server) ListContainerStats(ctx context.Context, req *pb.ListContainerStatsRequest) (*pb.ListContainerStatsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
