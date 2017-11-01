package server

import (
	"fmt"
	"strings"

	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ImageStatus returns the status of the image.
func (s *Server) ImageStatus(ctx context.Context, req *pb.ImageStatusRequest) (*pb.ImageStatusResponse, error) {
	logrus.Debugf("ImageStatusRequest: %+v", req)
	image := ""
	img := req.GetImage()
	if img != nil {
		image = img.Image
	}
	if image == "" {
		return nil, fmt.Errorf("no image specified")
	}
	images, err := s.StorageImageServer().ResolveNames(image)
	if err != nil {
		// This means we got an image ID
		if strings.Contains(err.Error(), "cannot specify 64-byte hexadecimal strings") {
			images = append(images, image)
		} else {
			return nil, err
		}
	}
	// match just the first registry as that's what kube meant
	image = images[0]
	status, err := s.StorageImageServer().ImageStatus(s.ImageContext(), image)
	if err != nil {
		if errors.Cause(err) == storage.ErrImageUnknown {
			return &pb.ImageStatusResponse{}, nil
		}
		return nil, err
	}
	resp := &pb.ImageStatusResponse{
		Image: &pb.Image{
			Id:          status.ID,
			RepoTags:    status.Names,
			RepoDigests: status.Digests,
			Size_:       *status.Size,
		},
	}
	logrus.Debugf("ImageStatusResponse: %+v", resp)
	return resp, nil
}
