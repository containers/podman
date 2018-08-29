// +build linux

package libpod

import (
	"context"

	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/cri-o/ocicni/pkg/ocicni"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
)

const (
	// IDTruncLength is the length of the pod's id that will be used to make the
	// infra container name
	IDTruncLength = 12
)

func (r *Runtime) makeInfraContainer(ctx context.Context, p *Pod, imgName, imgID string) (*Container, error) {

	// Set up generator for infra container defaults
	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}

	isRootless := rootless.IsRootless()

	g.SetRootReadonly(true)
	g.SetProcessArgs([]string{r.config.InfraCommand})

	if isRootless {
		g.RemoveMount("/dev/pts")
		devPts := spec.Mount{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"private", "nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		}
		g.AddMount(devPts)
	}

	containerName := p.ID()[:IDTruncLength] + "-infra"
	var options []CtrCreateOption
	options = append(options, r.WithPod(p))
	options = append(options, WithRootFSFromImage(imgID, imgName, false))
	options = append(options, WithName(containerName))
	options = append(options, withIsInfra())

	// Since user namespace sharing is not implemented, we only need to check if it's rootless
	portMappings := make([]ocicni.PortMapping, 0)
	networks := make([]string, 0)
	options = append(options, WithNetNS(portMappings, isRootless, networks))

	return r.newContainer(ctx, g.Config, options...)
}

// createInfraContainer wrap creates an infra container for a pod.
// An infra container becomes the basis for kernel namespace sharing between
// containers in the pod.
func (r *Runtime) createInfraContainer(ctx context.Context, p *Pod) (*Container, error) {
	if !r.valid {
		return nil, ErrRuntimeStopped
	}

	newImage, err := r.ImageRuntime().New(ctx, r.config.InfraImage, "", "", nil, nil, image.SigningOptions{}, false, false)
	if err != nil {
		return nil, err
	}

	data, err := newImage.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	imageName := newImage.Names()[0]
	imageID := data.ID

	return r.makeInfraContainer(ctx, p, imageName, imageID)
}
