//go:build !remote
// +build !remote

package specgen

import (
	"context"
	"time"

	"github.com/containers/common/libimage"
	"github.com/containers/podman/v4/libpod/define"
)

type cacheLibImage struct {
	image             *libimage.Image `json:"-"`
	resolvedImageName string          `json:"-"`
}

// SetImage sets the associated for the generator.
func (s *SpecGenerator) SetImage(image *libimage.Image, resolvedImageName string) {
	s.image = image
	s.resolvedImageName = resolvedImageName
}

// Image returns the associated image for the generator.
// May be nil if no image has been set yet.
func (s *SpecGenerator) GetImage() (*libimage.Image, string) {
	return s.image, s.resolvedImageName
}

func (s *SpecGenerator) InitHealthCheck(ctx context.Context) error {
	if s.HealthConfig == nil || s.Image != "" {
		return nil
	}
	// Image may already have been set in the generator.
	image, _ := s.GetImage()
	if image == nil {
		return nil
	}
	inspectData, err := image.Inspect(ctx, nil)
	if err != nil {
		return err
	}
	if inspectData == nil || inspectData.HealthCheck == nil {
		return nil
	}
	// NOTE: the health check is only set for Docker images
	// but inspect will take care of it.
	s.HealthConfig = inspectData.HealthCheck
	if s.HealthConfig.Timeout == 0 {
		hct, err := time.ParseDuration(define.DefaultHealthCheckTimeout)
		if err != nil {
			return err
		}
		s.HealthConfig.Timeout = hct
	}
	if s.HealthConfig.Interval == 0 {
		hct, err := time.ParseDuration(define.DefaultHealthCheckInterval)
		if err != nil {
			return err
		}
		s.HealthConfig.Interval = hct
	}
	return nil
}
