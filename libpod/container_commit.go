package libpod

import (
	"context"
	"fmt"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/util"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/events"
	"github.com/containers/podman/v2/libpod/image"
	libpodutil "github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ContainerCommitOptions is a struct used to commit a container to an image
// It uses buildah's CommitOptions as a base. Long-term we might wish to
// add these to the buildah struct once buildah is more integrated with
// libpod
type ContainerCommitOptions struct {
	buildah.CommitOptions
	Pause          bool
	IncludeVolumes bool
	Author         string
	Message        string
	Changes        []string
}

// Commit commits the changes between a container and its image, creating a new
// image
func (c *Container) Commit(ctx context.Context, destImage string, options ContainerCommitOptions) (*image.Image, error) {
	var (
		imageRef types.ImageReference
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

	if c.state.State == define.ContainerStateRunning && options.Pause {
		if err := c.pause(); err != nil {
			return nil, errors.Wrapf(err, "error pausing container %q to commit", c.ID())
		}
		defer func() {
			if err := c.unpause(); err != nil {
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
			splitEnv := strings.SplitN(e, "=", 2)
			importBuilder.SetEnv(splitEnv[0], splitEnv[1])
		}
	}
	// Expose ports
	for _, p := range c.config.PortMappings {
		importBuilder.SetPort(fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
	}
	// Labels
	for k, v := range c.Labels() {
		importBuilder.SetLabel(k, v)
	}
	// No stop signal
	// User
	if c.config.User != "" {
		importBuilder.SetUser(c.config.User)
	}
	// Volumes
	if options.IncludeVolumes {
		for _, v := range c.config.UserVolumes {
			if v != "" {
				importBuilder.AddVolume(v)
			}
		}
	} else {
		// Only include anonymous named volumes added by the user by
		// default.
		for _, v := range c.config.NamedVolumes {
			include := false
			for _, userVol := range c.config.UserVolumes {
				if userVol == v.Dest {
					include = true
					break
				}
			}
			if include {
				vol, err := c.runtime.GetVolume(v.Name)
				if err != nil {
					return nil, errors.Wrapf(err, "volume %s used in container %s has been removed", v.Name, c.ID())
				}
				if vol.Anonymous() {
					importBuilder.AddVolume(v.Dest)
				}
			}
		}
	}
	// Workdir
	importBuilder.SetWorkDir(c.config.Spec.Process.Cwd)

	// Process user changes
	newImageConfig, err := libpodutil.GetImageConfig(options.Changes)
	if err != nil {
		return nil, err
	}
	if newImageConfig.User != "" {
		importBuilder.SetUser(newImageConfig.User)
	}
	// EXPOSE only appends
	for port := range newImageConfig.ExposedPorts {
		importBuilder.SetPort(port)
	}
	// ENV only appends
	for _, env := range newImageConfig.Env {
		splitEnv := strings.SplitN(env, "=", 2)
		key := splitEnv[0]
		value := ""
		if len(splitEnv) == 2 {
			value = splitEnv[1]
		}
		importBuilder.SetEnv(key, value)
	}
	if newImageConfig.Entrypoint != nil {
		importBuilder.SetEntrypoint(newImageConfig.Entrypoint)
	}
	if newImageConfig.Cmd != nil {
		importBuilder.SetCmd(newImageConfig.Cmd)
	}
	// VOLUME only appends
	for vol := range newImageConfig.Volumes {
		importBuilder.AddVolume(vol)
	}
	if newImageConfig.WorkingDir != "" {
		importBuilder.SetWorkDir(newImageConfig.WorkingDir)
	}
	for k, v := range newImageConfig.Labels {
		importBuilder.SetLabel(k, v)
	}
	if newImageConfig.StopSignal != "" {
		importBuilder.SetStopSignal(newImageConfig.StopSignal)
	}
	for _, onbuild := range newImageConfig.OnBuild {
		importBuilder.SetOnBuild(onbuild)
	}

	candidates, _, _, err := util.ResolveName(destImage, "", sc, c.runtime.store)
	if err != nil {
		return nil, errors.Wrapf(err, "error resolving name %q", destImage)
	}
	if len(candidates) > 0 {
		imageRef, err = is.Transport.ParseStoreReference(c.runtime.store, candidates[0])
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing target image name %q", destImage)
		}
	}
	id, _, _, err := importBuilder.Commit(ctx, imageRef, commitOptions)
	if err != nil {
		return nil, err
	}
	defer c.newContainerEvent(events.Commit)
	return c.runtime.imageRuntime.NewFromLocal(id)
}
