package libpod

import (
	"context"
	"strings"

	is "github.com/containers/image/storage"
	"github.com/containers/libpod/libpod/image"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah"
	"github.com/projectatomic/buildah/util"
	"github.com/sirupsen/logrus"
)

// ContainerCommitOptions is a struct used to commit a container to an image
// It uses buildah's CommitOptions as a base. Long-term we might wish to
// add these to the buildah struct once buildah is more integrated with
//libpod
type ContainerCommitOptions struct {
	buildah.CommitOptions
	Pause   bool
	Author  string
	Message string
	Changes []string
}

// ChangeCmds is the list of valid Changes commands to passed to the Commit call
var ChangeCmds = []string{"CMD", "ENTRYPOINT", "ENV", "EXPOSE", "LABEL", "ONBUILD", "STOPSIGNAL", "USER", "VOLUME", "WORKDIR"}

// Commit commits the changes between a container and its image, creating a new
// image
func (c *Container) Commit(ctx context.Context, destImage string, options ContainerCommitOptions) (*image.Image, error) {
	var (
		isEnvCleared, isLabelCleared, isExposeCleared, isVolumeCleared bool
	)

	if c.config.Rootfs != "" {
		return nil, errors.Errorf("cannot commit a container that uses an exploded rootfs")
	}

	if !c.batched {
		c.lock.Lock()
		defer c.lock.Unlock()

		if err := c.syncContainer(); err != nil {
			return nil, err
		}
	}

	if c.state.State == ContainerStateRunning && options.Pause {
		if err := c.runtime.ociRuntime.pauseContainer(c); err != nil {
			return nil, errors.Wrapf(err, "error pausing container %q", c.ID())
		}
		defer func() {
			if err := c.runtime.ociRuntime.unpauseContainer(c); err != nil {
				logrus.Errorf("error unpausing container %q: %v", c.ID(), err)
			}
		}()
	}

	sc := image.GetSystemContext(options.SignaturePolicyPath, "", false)
	builderOptions := buildah.ImportOptions{
		Container:           c.ID(),
		SignaturePolicyPath: options.SignaturePolicyPath,
	}
	commitOptions := buildah.CommitOptions{
		SignaturePolicyPath:   options.SignaturePolicyPath,
		ReportWriter:          options.ReportWriter,
		SystemContext:         sc,
		PreferredManifestType: options.PreferredManifestType,
	}
	importBuilder, err := buildah.ImportBuilder(ctx, c.runtime.store, builderOptions)
	if err != nil {
		return nil, err
	}

	if options.Author != "" {
		importBuilder.SetMaintainer(options.Author)
	}
	if options.Message != "" {
		importBuilder.SetComment(options.Message)
	}

	// We need to take meta we find in the current container and
	// add it to the resulting image.

	// Entrypoint - always set this first or cmd will get wiped out
	importBuilder.SetEntrypoint(c.config.Entrypoint)

	// Cmd
	importBuilder.SetCmd(c.config.Command)

	// Env
	// TODO - this includes all the default environment vars as well
	// Should we store the ENV we actually want in the spec separately?
	if c.config.Spec.Process != nil {
		for _, e := range c.config.Spec.Process.Env {
			splitEnv := strings.Split(e, "=")
			importBuilder.SetEnv(splitEnv[0], splitEnv[1])
		}
	}
	// Expose ports
	for _, p := range c.config.PortMappings {
		importBuilder.SetPort(string(p.ContainerPort))
	}
	// Labels
	for k, v := range c.Labels() {
		importBuilder.SetLabel(k, v)
	}
	// No stop signal
	// User
	importBuilder.SetUser(c.User())
	// Volumes
	for _, v := range c.config.UserVolumes {
		if v != "" {
			importBuilder.AddVolume(v)
		}
	}
	// Workdir
	importBuilder.SetWorkDir(c.Spec().Process.Cwd)

	// Process user changes
	for _, change := range options.Changes {
		splitChange := strings.Split(change, "=")
		switch strings.ToUpper(splitChange[0]) {
		case "CMD":
			importBuilder.SetCmd(splitChange[1:])
		case "ENTRYPOINT":
			importBuilder.SetEntrypoint(splitChange[1:])
		case "ENV":
			if !isEnvCleared { // Multiple values are valid, only clear once.
				importBuilder.ClearEnv()
				isEnvCleared = true
			}
			importBuilder.SetEnv(splitChange[1], splitChange[2])
		case "EXPOSE":
			if !isExposeCleared { // Multiple values are valid, only clear once
				importBuilder.ClearPorts()
				isExposeCleared = true
			}
			importBuilder.SetPort(splitChange[1])
		case "LABEL":
			if !isLabelCleared { // multiple values are valid, only clear once
				importBuilder.ClearLabels()
				isLabelCleared = true
			}
			importBuilder.SetLabel(splitChange[1], splitChange[2])
		case "ONBUILD":
			importBuilder.SetOnBuild(splitChange[1])
		case "STOPSIGNAL":
			// No Set StopSignal
		case "USER":
			importBuilder.SetUser(splitChange[1])
		case "VOLUME":
			if !isVolumeCleared { // multiple values are valid, only clear once
				importBuilder.ClearVolumes()
				isVolumeCleared = true
			}
			importBuilder.AddVolume(splitChange[1])
		case "WORKDIR":
			importBuilder.SetWorkDir(splitChange[1])
		}
	}
	candidates, err := util.ResolveName(destImage, "", sc, c.runtime.store)
	if err != nil {
		return nil, errors.Wrapf(err, "error resolving name %q", destImage)
	}
	if len(candidates) == 0 {
		return nil, errors.Errorf("error parsing target image name %q", destImage)
	}
	imageRef, err := is.Transport.ParseStoreReference(c.runtime.store, candidates[0])
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing target image name %q", destImage)
	}
	id, err := importBuilder.Commit(ctx, imageRef, commitOptions)
	if err != nil {
		return nil, err
	}
	return c.runtime.imageRuntime.NewFromLocal(id)
}
