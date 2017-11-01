package server

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// RemoveImage removes the image.
func (s *Server) RemoveImage(ctx context.Context, req *pb.RemoveImageRequest) (*pb.RemoveImageResponse, error) {
	logrus.Debugf("RemoveImageRequest: %+v", req)
	image := ""
	img := req.GetImage()
	if img != nil {
		image = img.Image
	}
	if image == "" {
		return nil, fmt.Errorf("no image specified")
	}
	var (
		images  []string
		err     error
		deleted bool
	)
	images, err = s.StorageImageServer().ResolveNames(image)
	if err != nil {
		// This means we got an image ID
		if strings.Contains(err.Error(), "cannot specify 64-byte hexadecimal strings") {
			images = append(images, image)
		} else {
			return nil, err
		}
	}
	for _, img := range images {
		err = s.StorageImageServer().UntagImage(s.ImageContext(), img)
		if err != nil {
			logrus.Debugf("error deleting image %s: %v", img, err)
			continue
		}
		deleted = true
		break
	}
	if !deleted && err != nil {
		return nil, err
	}
	resp := &pb.RemoveImageResponse{}
	logrus.Debugf("RemoveImageResponse: %+v", resp)
	return resp, nil
}
