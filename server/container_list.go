package server

import (
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/fields"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// filterContainer returns whether passed container matches filtering criteria
func filterContainer(c *pb.Container, filter *pb.ContainerFilter) bool {
	if filter != nil {
		if filter.State != nil {
			if c.State != filter.State.State {
				return false
			}
		}
		if filter.LabelSelector != nil {
			sel := fields.SelectorFromSet(filter.LabelSelector)
			if !sel.Matches(fields.Set(c.Labels)) {
				return false
			}
		}
	}
	return true
}

// ListContainers lists all containers by filters.
func (s *Server) ListContainers(ctx context.Context, req *pb.ListContainersRequest) (*pb.ListContainersResponse, error) {
	logrus.Debugf("ListContainersRequest %+v", req)
	var ctrs []*pb.Container
	filter := req.Filter
	ctrList, err := s.ContainerServer.ListContainers()
	if err != nil {
		return nil, err
	}

	// Filter using container id and pod id first.
	if filter.Id != "" {
		id, err := s.CtrIDIndex().Get(filter.Id)
		if err != nil {
			// If we don't find a container ID with a filter, it should not
			// be considered an error.  Log a warning and return an empty struct
			logrus.Warn("unable to find container ID %s", filter.Id)
			return &pb.ListContainersResponse{}, nil
		}
		c := s.ContainerServer.GetContainer(id)
		if c != nil {
			if filter.PodSandboxId != "" {
				if c.Sandbox() == filter.PodSandboxId {
					ctrList = []*oci.Container{c}
				} else {
					ctrList = []*oci.Container{}
				}

			} else {
				ctrList = []*oci.Container{c}
			}
		}
	} else {
		if filter.PodSandboxId != "" {
			pod := s.ContainerServer.GetSandbox(filter.PodSandboxId)
			if pod == nil {
				ctrList = []*oci.Container{}
			} else {
				ctrList = pod.Containers().List()
			}
		}
	}

	for _, ctr := range ctrList {
		podSandboxID := ctr.Sandbox()
		cState := s.Runtime().ContainerStatus(ctr)
		created := cState.Created.UnixNano()
		rState := pb.ContainerState_CONTAINER_UNKNOWN
		cID := ctr.ID()
		img := &pb.ImageSpec{
			Image: ctr.Image(),
		}
		c := &pb.Container{
			Id:           cID,
			PodSandboxId: podSandboxID,
			CreatedAt:    created,
			Labels:       ctr.Labels(),
			Metadata:     ctr.Metadata(),
			Annotations:  ctr.Annotations(),
			Image:        img,
		}

		switch cState.Status {
		case oci.ContainerStateCreated:
			rState = pb.ContainerState_CONTAINER_CREATED
		case oci.ContainerStateRunning:
			rState = pb.ContainerState_CONTAINER_RUNNING
		case oci.ContainerStateStopped:
			rState = pb.ContainerState_CONTAINER_EXITED
		}
		c.State = rState

		// Filter by other criteria such as state and labels.
		if filterContainer(c, req.Filter) {
			ctrs = append(ctrs, c)
		}
	}

	resp := &pb.ListContainersResponse{
		Containers: ctrs,
	}
	logrus.Debugf("ListContainersResponse: %+v", resp)
	return resp, nil
}
