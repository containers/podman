package server

import (
	"github.com/kubernetes-incubator/cri-o/libkpod/sandbox"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/fields"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// filterSandbox returns whether passed container matches filtering criteria
func filterSandbox(p *pb.PodSandbox, filter *pb.PodSandboxFilter) bool {
	if filter != nil {
		if filter.State != nil {
			if p.State != filter.State.State {
				return false
			}
		}
		if filter.LabelSelector != nil {
			sel := fields.SelectorFromSet(filter.LabelSelector)
			if !sel.Matches(fields.Set(p.Labels)) {
				return false
			}
		}
	}
	return true
}

// ListPodSandbox returns a list of SandBoxes.
func (s *Server) ListPodSandbox(ctx context.Context, req *pb.ListPodSandboxRequest) (*pb.ListPodSandboxResponse, error) {
	logrus.Debugf("ListPodSandboxRequest %+v", req)
	var pods []*pb.PodSandbox
	var podList []*sandbox.Sandbox
	for _, sb := range s.ContainerServer.ListSandboxes() {
		podList = append(podList, sb)
	}

	filter := req.Filter
	// Filter by pod id first.
	if filter != nil {
		if filter.Id != "" {
			id, err := s.PodIDIndex().Get(filter.Id)
			if err != nil {
				// Not finding an ID in a filtered list should not be considered
				// and error; it might have been deleted when stop was done.
				// Log and return an empty struct.
				logrus.Warn("unable to find pod %s with filter", filter.Id)
				return &pb.ListPodSandboxResponse{}, nil
			}
			sb := s.getSandbox(id)
			if sb == nil {
				podList = []*sandbox.Sandbox{}
			} else {
				podList = []*sandbox.Sandbox{sb}
			}
		}
	}

	for _, sb := range podList {
		podInfraContainer := sb.InfraContainer()
		if podInfraContainer == nil {
			// this can't really happen, but if it does because of a bug
			// it's better not to panic
			continue
		}
		cState := s.Runtime().ContainerStatus(podInfraContainer)
		created := cState.Created.UnixNano()
		rStatus := pb.PodSandboxState_SANDBOX_NOTREADY
		if cState.Status == oci.ContainerStateRunning {
			rStatus = pb.PodSandboxState_SANDBOX_READY
		}

		pod := &pb.PodSandbox{
			Id:          sb.ID(),
			CreatedAt:   created,
			State:       rStatus,
			Labels:      sb.Labels(),
			Annotations: sb.Annotations(),
			Metadata:    sb.Metadata(),
		}

		// Filter by other criteria such as state and labels.
		if filterSandbox(pod, req.Filter) {
			pods = append(pods, pod)
		}
	}

	resp := &pb.ListPodSandboxResponse{
		Items: pods,
	}
	logrus.Debugf("ListPodSandboxResponse %+v", resp)
	return resp, nil
}
