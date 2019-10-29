package libpod

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/util"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/libpod/image"
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

	if c.state.State == define.ContainerStateRunning && options.Pause {
		if err := c.pause(); err != nil {
			return nil, errors.Wrapf(err, "error pausing container %q", c.ID())
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
		importBuilder.SetPort(fmt.Sprintf("%d", p.ContainerPort))
	}
	// Labels
	for k, v := range c.Labels() {
		importBuilder.SetLabel(k, v)
	}
	// No stop signal
	// User
	importBuilder.SetUser(c.User())
	// Volumes
	if options.IncludeVolumes {
		for _, v := range c.config.UserVolumes {
			if v != "" {
				importBuilder.AddVolume(v)
			}
		}
	}
	// Workdir
	importBuilder.SetWorkDir(c.Spec().Process.Cwd)

	genCmd := func(cmd string) []string {
		trim := func(cmd []string) []string {
			if len(cmd) == 0 {
				return cmd
			}

			retCmd := []string{}
			for _, c := range cmd {
				if len(c) >= 2 {
					if c[0] == '"' && c[len(c)-1] == '"' {
						retCmd = append(retCmd, c[1:len(c)-1])
						continue
					}
				}
				retCmd = append(retCmd, c)
			}
			return retCmd
		}
		if strings.HasPrefix(cmd, "[") {
			cmd = strings.TrimPrefix(cmd, "[")
			cmd = strings.TrimSuffix(cmd, "]")
			return trim(strings.Split(cmd, ","))
		}
		return []string{"/bin/sh", "-c", cmd}
	}
	// Process user changes
	for _, change := range options.Changes {
		splitChange := strings.SplitN(change, "=", 2)
		if len(splitChange) != 2 {
			splitChange = strings.SplitN(change, " ", 2)
			if len(splitChange) < 2 {
				return nil, errors.Errorf("invalid change %s format", change)
			}
		}

		switch strings.ToUpper(splitChange[0]) {
		case "CMD":
			importBuilder.SetCmd(genCmd(splitChange[1]))
		case "ENTRYPOINT":
			importBuilder.SetEntrypoint(genCmd(splitChange[1]))
		case "ENV":
			change := strings.Split(splitChange[1], " ")
			name := change[0]
			val := ""
			if len(change) < 2 {
				change = strings.Split(change[0], "=")
			}
			if len(change) < 2 {
				var ok bool
				val, ok = os.LookupEnv(name)
				if !ok {
					return nil, errors.Errorf("invalid env variable %q: not defined in your environment", name)
				}
			} else {
				name = change[0]
				val = strings.Join(change[1:], " ")
			}
			if !isEnvCleared { // Multiple values are valid, only clear once.
				importBuilder.ClearEnv()
				isEnvCleared = true
			}
			importBuilder.SetEnv(name, val)
		case "EXPOSE":
			if !isExposeCleared { // Multiple values are valid, only clear once
				importBuilder.ClearPorts()
				isExposeCleared = true
			}
			importBuilder.SetPort(splitChange[1])
		case "LABEL":
			change := strings.Split(splitChange[1], " ")
			if len(change) < 2 {
				change = strings.Split(change[0], "=")
			}
			if len(change) < 2 {
				return nil, errors.Errorf("invalid label %s format, requires to NAME=VAL", splitChange[1])
			}
			if !isLabelCleared { // multiple values are valid, only clear once
				importBuilder.ClearLabels()
				isLabelCleared = true
			}
			importBuilder.SetLabel(change[0], strings.Join(change[1:], " "))
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
	candidates, _, _, err := util.ResolveName(destImage, "", sc, c.runtime.store)
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
	id, _, _, err := importBuilder.Commit(ctx, imageRef, commitOptions)
	if err != nil {
		return nil, err
	}
	defer c.newContainerEvent(events.Commit)
	return c.runtime.imageRuntime.NewFromLocal(id)
}
