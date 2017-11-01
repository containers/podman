package server

import (
	"fmt"

	"github.com/containers/storage"
	"github.com/kubernetes-incubator/cri-o/libkpod/sandbox"
	"github.com/kubernetes-incubator/cri-o/oci"
	pkgstorage "github.com/kubernetes-incubator/cri-o/pkg/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// RemovePodSandbox deletes the sandbox. If there are any running containers in the
// sandbox, they should be force deleted.
func (s *Server) RemovePodSandbox(ctx context.Context, req *pb.RemovePodSandboxRequest) (*pb.RemovePodSandboxResponse, error) {
	logrus.Debugf("RemovePodSandboxRequest %+v", req)
	sb, err := s.getPodSandboxFromRequest(req.PodSandboxId)
	if err != nil {
		if err == sandbox.ErrIDEmpty {
			return nil, err
		}

		// If the sandbox isn't found we just return an empty response to adhere
		// the the CRI interface which expects to not error out in not found
		// cases.

		resp := &pb.RemovePodSandboxResponse{}
		logrus.Warnf("could not get sandbox %s, it's probably been removed already: %v", req.PodSandboxId, err)
		return resp, nil
	}

	podInfraContainer := sb.InfraContainer()
	containers := sb.Containers().List()
	containers = append(containers, podInfraContainer)

	// Delete all the containers in the sandbox
	for _, c := range containers {
		if !sb.Stopped() {
			cState := s.Runtime().ContainerStatus(c)
			if cState.Status == oci.ContainerStateCreated || cState.Status == oci.ContainerStateRunning {
				if err := s.Runtime().StopContainer(ctx, c, 10); err != nil {
					// Assume container is already stopped
					logrus.Warnf("failed to stop container %s: %v", c.Name(), err)
				}
			}
		}

		if err := s.Runtime().DeleteContainer(c); err != nil {
			return nil, fmt.Errorf("failed to delete container %s in pod sandbox %s: %v", c.Name(), sb.ID(), err)
		}

		if c.ID() == podInfraContainer.ID() {
			continue
		}

		if err := s.StorageRuntimeServer().StopContainer(c.ID()); err != nil && err != storage.ErrContainerUnknown {
			// assume container already umounted
			logrus.Warnf("failed to stop container %s in pod sandbox %s: %v", c.Name(), sb.ID(), err)
		}
		if err := s.StorageRuntimeServer().DeleteContainer(c.ID()); err != nil && err != storage.ErrContainerUnknown {
			return nil, fmt.Errorf("failed to delete container %s in pod sandbox %s: %v", c.Name(), sb.ID(), err)
		}

		s.ReleaseContainerName(c.Name())
		s.removeContainer(c)
		if err := s.CtrIDIndex().Delete(c.ID()); err != nil {
			return nil, fmt.Errorf("failed to delete container %s in pod sandbox %s from index: %v", c.Name(), sb.ID(), err)
		}
	}

	s.removeInfraContainer(podInfraContainer)

	// Remove the files related to the sandbox
	if err := s.StorageRuntimeServer().StopContainer(sb.ID()); err != nil && errors.Cause(err) != storage.ErrContainerUnknown {
		logrus.Warnf("failed to stop sandbox container in pod sandbox %s: %v", sb.ID(), err)
	}
	if err := s.StorageRuntimeServer().RemovePodSandbox(sb.ID()); err != nil && err != pkgstorage.ErrInvalidSandboxID {
		return nil, fmt.Errorf("failed to remove pod sandbox %s: %v", sb.ID(), err)
	}

	s.ReleaseContainerName(podInfraContainer.Name())
	if err := s.CtrIDIndex().Delete(podInfraContainer.ID()); err != nil {
		return nil, fmt.Errorf("failed to delete infra container %s in pod sandbox %s from index: %v", podInfraContainer.ID(), sb.ID(), err)
	}

	s.ReleasePodName(sb.Name())
	s.removeSandbox(sb.ID())
	if err := s.PodIDIndex().Delete(sb.ID()); err != nil {
		return nil, fmt.Errorf("failed to delete pod sandbox %s from index: %v", sb.ID(), err)
	}

	resp := &pb.RemovePodSandboxResponse{}
	logrus.Debugf("RemovePodSandboxResponse %+v", resp)
	return resp, nil
}
