package server

import (
	"fmt"

	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ImageFsInfo returns information of the filesystem that is used to store images.
func (s *Server) ImageFsInfo(ctx context.Context, req *pb.ImageFsInfoRequest) (*pb.ImageFsInfoResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
