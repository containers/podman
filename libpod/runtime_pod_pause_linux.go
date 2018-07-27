// +build linux

package libpod

import (
	"context"

	"github.com/containers/libpod/libpod/image"
	"github.com/opencontainers/runtime-tools/generate"
)

const (
	// IDTruncLength is the length of the pod's id that will be used to make the
	// pause container name
	IDTruncLength = 12
)

func (r *Runtime) makePauseContainer(ctx context.Context, p *Pod, imgName, imgID string) (*Container, error) {

	// Set up generator for pause container defaults
	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}

	g.SetRootReadonly(true)
	g.SetProcessArgs([]string{r.config.PauseCommand})

	containerName := p.ID()[:IDTruncLength] + "-infra"
	var options []CtrCreateOption
	options = append(options, r.WithPod(p))
	options = append(options, WithRootFSFromImage(imgID, imgName, false))
	options = append(options, WithName(containerName))
	options = append(options, withIsPause())

	return r.newContainer(ctx, g.Config, options...)
}

// createPauseContainer wrap creates a pause container for a pod.
// A pause container becomes the basis for kernel namespace sharing between
// containers in the pod.
func (r *Runtime) createPauseContainer(ctx context.Context, p *Pod) (*Container, error) {
	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	newImage, err := r.ImageRuntime().New(ctx, r.config.PauseImage, "", "", nil, nil, image.SigningOptions{}, false, false)
	if err != nil {
		return nil, err
	}

	data, err := newImage.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	imageName := newImage.Names()[0]
	imageID := data.ID

	return r.makePauseContainer(ctx, p, imageName, imageID)
}
