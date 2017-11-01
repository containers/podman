package server

import (
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// Status returns the status of the runtime
func (s *Server) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {

	// Deal with Runtime conditions
	runtimeReady, err := s.Runtime().RuntimeReady()
	if err != nil {
		return nil, err
	}
	networkReady, err := s.Runtime().NetworkReady()
	if err != nil {
		return nil, err
	}

	// Use vendored strings
	runtimeReadyConditionString := pb.RuntimeReady
	networkReadyConditionString := pb.NetworkReady

	resp := &pb.StatusResponse{
		Status: &pb.RuntimeStatus{
			Conditions: []*pb.RuntimeCondition{
				{
					Type:   runtimeReadyConditionString,
					Status: runtimeReady,
				},
				{
					Type:   networkReadyConditionString,
					Status: networkReady,
				},
			},
		},
	}

	return resp, nil
}
