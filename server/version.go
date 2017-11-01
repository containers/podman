package server

import (
	"github.com/kubernetes-incubator/cri-o/version"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

const (
	// kubeAPIVersion is the api version of kubernetes.
	// TODO: Track upstream code. For now it expects 0.1.0
	kubeAPIVersion = "0.1.0"
	// containerName is the name prepended in kubectl describe->Container ID:
	// cri-o://<CONTAINER_ID>
	containerName     = "cri-o"
	runtimeAPIVersion = "v1alpha1"
)

// Version returns the runtime name, runtime version and runtime API version
func (s *Server) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{
		Version:           kubeAPIVersion,
		RuntimeName:       containerName,
		RuntimeVersion:    version.Version,
		RuntimeApiVersion: runtimeAPIVersion,
	}, nil
}
