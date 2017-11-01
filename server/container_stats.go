package server

import (
	"fmt"

	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ContainerStats returns stats of the container. If the container does not
// exist, the call returns an error.
func (s *Server) ContainerStats(ctx context.Context, req *pb.ContainerStatsRequest) (*pb.ContainerStatsResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
