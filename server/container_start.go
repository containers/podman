package server

import (
	"fmt"

	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// StartContainer starts the container.
func (s *Server) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*pb.StartContainerResponse, error) {
	logrus.Debugf("StartContainerRequest %+v", req)
	c, err := s.GetContainerFromRequest(req.ContainerId)
	if err != nil {
		return nil, err
	}
	state := s.Runtime().ContainerStatus(c)
	if state.Status != oci.ContainerStateCreated {
		return nil, fmt.Errorf("container %s is not in created state: %s", c.ID(), state.Status)
	}

	defer func() {
		// if the call to StartContainer fails below we still want to fill
		// some fields of a container status. In particular, we're going to
		// adjust container started/finished time and set an error to be
		// returned in the Reason field for container status call.
		if err != nil {
			s.Runtime().SetStartFailed(c, err)
		}
		s.ContainerStateToDisk(c)
	}()

	err = s.Runtime().StartContainer(c)
	if err != nil {
		return nil, fmt.Errorf("failed to start container %s: %v", c.ID(), err)
	}

	resp := &pb.StartContainerResponse{}
	logrus.Debugf("StartContainerResponse %+v", resp)
	return resp, nil
}
