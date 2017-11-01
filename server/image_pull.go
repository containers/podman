package server

import (
	"encoding/base64"
	"strings"

	"github.com/containers/image/copy"
	"github.com/containers/image/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// PullImage pulls a image with authentication config.
func (s *Server) PullImage(ctx context.Context, req *pb.PullImageRequest) (*pb.PullImageResponse, error) {
	logrus.Debugf("PullImageRequest: %+v", req)
	// TODO: what else do we need here? (Signatures when the story isn't just pulling from docker://)
	image := ""
	img := req.GetImage()
	if img != nil {
		image = img.Image
	}

	var (
		images []string
		pulled string
		err    error
	)
	images, err = s.StorageImageServer().ResolveNames(image)
	if err != nil {
		return nil, err
	}
	for _, img := range images {
		var (
			username string
			password string
		)
		if req.GetAuth() != nil {
			username = req.GetAuth().Username
			password = req.GetAuth().Password
			if req.GetAuth().Auth != "" {
				username, password, err = decodeDockerAuth(req.GetAuth().Auth)
				if err != nil {
					logrus.Debugf("error decoding authentication for image %s: %v", img, err)
					continue
				}
			}
		}
		options := &copy.Options{
			SourceCtx: &types.SystemContext{},
		}
		// Specifying a username indicates the user intends to send authentication to the registry.
		if username != "" {
			options.SourceCtx = &types.SystemContext{
				DockerAuthConfig: &types.DockerAuthConfig{
					Username: username,
					Password: password,
				},
			}
		}

		var canPull bool
		canPull, err = s.StorageImageServer().CanPull(img, options)
		if err != nil && !canPull {
			logrus.Debugf("error checking image %s: %v", img, err)
			continue
		}

		// let's be smart, docker doesn't repull if image already exists.
		_, err = s.StorageImageServer().ImageStatus(s.ImageContext(), img)
		if err == nil {
			logrus.Debugf("image %s already in store, skipping pull", img)
			pulled = img
			break
		}

		_, err = s.StorageImageServer().PullImage(s.ImageContext(), img, options)
		if err != nil {
			logrus.Debugf("error pulling image %s: %v", img, err)
			continue
		}
		pulled = img
		break
	}
	if pulled == "" && err != nil {
		return nil, err
	}
	resp := &pb.PullImageResponse{
		ImageRef: pulled,
	}
	logrus.Debugf("PullImageResponse: %+v", resp)
	return resp, nil
}

func decodeDockerAuth(s string) (string, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		// if it's invalid just skip, as docker does
		return "", "", nil
	}
	user := parts[0]
	password := strings.Trim(parts[1], "\x00")
	return user, password, nil
}
