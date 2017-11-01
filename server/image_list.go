package server

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// ListImages lists existing images.
func (s *Server) ListImages(ctx context.Context, req *pb.ListImagesRequest) (*pb.ListImagesResponse, error) {
	logrus.Debugf("ListImagesRequest: %+v", req)
	filter := ""
	reqFilter := req.GetFilter()
	if reqFilter != nil {
		filterImage := reqFilter.GetImage()
		if filterImage != nil {
			filter = filterImage.Image
		}
	}
	results, err := s.StorageImageServer().ListImages(s.ImageContext(), filter)
	if err != nil {
		return nil, err
	}
	response := pb.ListImagesResponse{}
	for _, result := range results {
		if result.Size != nil {
			response.Images = append(response.Images, &pb.Image{
				Id:       result.ID,
				RepoTags: result.Names,
				Size_:    *result.Size,
			})
		} else {
			response.Images = append(response.Images, &pb.Image{
				Id:       result.ID,
				RepoTags: result.Names,
			})
		}
	}
	logrus.Debugf("ListImagesResponse: %+v", response)
	return &response, nil
}
