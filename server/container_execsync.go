package server

import (
	"fmt"

	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ExecSync runs a command in a container synchronously.
func (s *Server) ExecSync(ctx context.Context, req *pb.ExecSyncRequest) (*pb.ExecSyncResponse, error) {
	logrus.Debugf("ExecSyncRequest %+v", req)
	c, err := s.GetContainerFromRequest(req.ContainerId)
	if err != nil {
		return nil, err
	}

	if err = s.Runtime().UpdateStatus(c); err != nil {
		return nil, err
	}

	cState := s.Runtime().ContainerStatus(c)
	if !(cState.Status == oci.ContainerStateRunning || cState.Status == oci.ContainerStateCreated) {
		return nil, fmt.Errorf("container is not created or running")
	}

	cmd := req.Cmd
	if cmd == nil {
		return nil, fmt.Errorf("exec command cannot be empty")
	}

	execResp, err := s.Runtime().ExecSync(c, cmd, req.Timeout)
	if err != nil {
		return nil, err
	}
	resp := &pb.ExecSyncResponse{
		Stdout:   execResp.Stdout,
		Stderr:   execResp.Stderr,
		ExitCode: execResp.ExitCode,
	}

	logrus.Debugf("ExecSyncResponse: %+v", resp)
	return resp, nil
}
