package server

import (
	"fmt"

	"github.com/containers/storage"
	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/symlink"
	"github.com/kubernetes-incubator/cri-o/libkpod/sandbox"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
func (s *Server) StopPodSandbox(ctx context.Context, req *pb.StopPodSandboxRequest) (*pb.StopPodSandboxResponse, error) {
	logrus.Debugf("StopPodSandboxRequest %+v", req)
	sb, err := s.getPodSandboxFromRequest(req.PodSandboxId)
	if err != nil {
		if err == sandbox.ErrIDEmpty {
			return nil, err
		}

		// If the sandbox isn't found we just return an empty response to adhere
		// the the CRI interface which expects to not error out in not found
		// cases.

		resp := &pb.StopPodSandboxResponse{}
		logrus.Warnf("could not get sandbox %s, it's probably been stopped already: %v", req.PodSandboxId, err)
		logrus.Debugf("StopPodSandboxResponse %s: %+v", req.PodSandboxId, resp)
		return resp, nil
	}

	if sb.Stopped() {
		resp := &pb.StopPodSandboxResponse{}
		logrus.Debugf("StopPodSandboxResponse %s: %+v", sb.ID(), resp)
		return resp, nil
	}

	// Clean up sandbox networking and close its network namespace.
	hostNetwork := sb.NetNsPath() == ""
	s.networkStop(hostNetwork, sb)
	if err := sb.NetNsRemove(); err != nil {
		return nil, err
	}

	podInfraContainer := sb.InfraContainer()
	containers := sb.Containers().List()
	containers = append(containers, podInfraContainer)

	for _, c := range containers {
		cStatus := s.Runtime().ContainerStatus(c)
		if cStatus.Status != oci.ContainerStateStopped {
			if err := s.Runtime().StopContainer(ctx, c, 10); err != nil {
				return nil, fmt.Errorf("failed to stop container %s in pod sandbox %s: %v", c.Name(), sb.ID(), err)
			}
			if c.ID() == podInfraContainer.ID() {
				continue
			}
			if err := s.StorageRuntimeServer().StopContainer(c.ID()); err != nil && errors.Cause(err) != storage.ErrContainerUnknown {
				// assume container already umounted
				logrus.Warnf("failed to stop container %s in pod sandbox %s: %v", c.Name(), sb.ID(), err)
			}
		}
		s.ContainerStateToDisk(c)
	}

	if err := label.ReleaseLabel(sb.ProcessLabel()); err != nil {
		return nil, err
	}

	// unmount the shm for the pod
	if sb.ShmPath() != "/dev/shm" {
		// we got namespaces in the form of
		// /var/run/containers/storage/overlay-containers/CID/userdata/shm
		// but /var/run on most system is symlinked to /run so we first resolve
		// the symlink and then try and see if it's mounted
		fp, err := symlink.FollowSymlinkInScope(sb.ShmPath(), "/")
		if err != nil {
			return nil, err
		}
		if mounted, err := mount.Mounted(fp); err == nil && mounted {
			if err := unix.Unmount(fp, unix.MNT_DETACH); err != nil {
				return nil, err
			}
		}
	}
	if err := s.StorageRuntimeServer().StopContainer(sb.ID()); err != nil && errors.Cause(err) != storage.ErrContainerUnknown {
		logrus.Warnf("failed to stop sandbox container in pod sandbox %s: %v", sb.ID(), err)
	}

	sb.SetStopped()
	resp := &pb.StopPodSandboxResponse{}
	logrus.Debugf("StopPodSandboxResponse %s: %+v", sb.ID(), resp)
	return resp, nil
}

// StopAllPodSandboxes removes all pod sandboxes
func (s *Server) StopAllPodSandboxes() {
	logrus.Debugf("StopAllPodSandboxes")
	for _, sb := range s.ContainerServer.ListSandboxes() {
		pod := &pb.StopPodSandboxRequest{
			PodSandboxId: sb.ID(),
		}
		if _, err := s.StopPodSandbox(nil, pod); err != nil {
			logrus.Warnf("could not StopPodSandbox %s: %v", sb.ID(), err)
		}
	}
}
