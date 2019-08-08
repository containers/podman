package buildah

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"time"

	"github.com/containers/buildah/docker"
	"github.com/containers/image/manifest"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/stringid"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// unmarshalConvertedConfig obtains the config blob of img valid for the wantedManifestMIMEType format
// (either as it exists, or converting the image if necessary), and unmarshals it into dest.
// NOTE: The MIME type is of the _manifest_, not of the _config_ that is returned.
func unmarshalConvertedConfig(ctx context.Context, dest interface{}, img types.Image, wantedManifestMIMEType string) error {
	_, actualManifestMIMEType, err := img.Manifest(ctx)
	if err != nil {
		return errors.Wrapf(err, "error getting manifest MIME type for %q", transports.ImageName(img.Reference()))
	}
	if wantedManifestMIMEType != actualManifestMIMEType {
		img, err = img.UpdatedImage(ctx, types.ManifestUpdateOptions{
			ManifestMIMEType: wantedManifestMIMEType,
			InformationOnly: types.ManifestUpdateInformation{ // Strictly speaking, every value in here is invalid. But…
				Destination:  nil, // Destination is technically required, but actually necessary only for conversion _to_ v2s1.  Leave it nil, we will crash if that ever changes.
				LayerInfos:   nil, // LayerInfos is necessary for size information in v2s2/OCI manifests, but the code can work with nil, and we are not reading the converted manifest at all.
				LayerDiffIDs: nil, // LayerDiffIDs are actually embedded in the converted manifest, but the code can work with nil, and the values are not needed until pushing the finished image, at which time containerImageRef.NewImageSource builds the values from scratch.
			},
		})
		if err != nil {
			return errors.Wrapf(err, "error converting image %q to %s", transports.ImageName(img.Reference()), wantedManifestMIMEType)
		}
	}
	config, err := img.ConfigBlob(ctx)
	if err != nil {
		return errors.Wrapf(err, "error reading %s config from %q", wantedManifestMIMEType, transports.ImageName(img.Reference()))
	}
	if err := json.Unmarshal(config, dest); err != nil {
		return errors.Wrapf(err, "error parsing %s configuration %q from %q", wantedManifestMIMEType, string(config), transports.ImageName(img.Reference()))
	}
	return nil
}

func (b *Builder) initConfig(ctx context.Context, img types.Image) error {
	if img != nil { // A pre-existing image, as opposed to a "FROM scratch" new one.
		rawManifest, manifestMIMEType, err := img.Manifest(ctx)
		if err != nil {
			return errors.Wrapf(err, "error reading image manifest for %q", transports.ImageName(img.Reference()))
		}
		rawConfig, err := img.ConfigBlob(ctx)
		if err != nil {
			return errors.Wrapf(err, "error reading image configuration for %q", transports.ImageName(img.Reference()))
		}
		b.Manifest = rawManifest
		b.Config = rawConfig

		dimage := docker.V2Image{}
		if err := unmarshalConvertedConfig(ctx, &dimage, img, manifest.DockerV2Schema2MediaType); err != nil {
			return err
		}
		b.Docker = dimage

		oimage := ociv1.Image{}
		if err := unmarshalConvertedConfig(ctx, &oimage, img, ociv1.MediaTypeImageManifest); err != nil {
			return err
		}
		b.OCIv1 = oimage

		if manifestMIMEType == ociv1.MediaTypeImageManifest {
			// Attempt to recover format-specific data from the manifest.
			v1Manifest := ociv1.Manifest{}
			if err := json.Unmarshal(b.Manifest, &v1Manifest); err != nil {
				return errors.Wrapf(err, "error parsing OCI manifest %q", string(b.Manifest))
			}
			b.ImageAnnotations = v1Manifest.Annotations
		}
	}

	b.fixupConfig()
	return nil
}

func (b *Builder) fixupConfig() {
	if b.Docker.Config != nil {
		// Prefer image-level settings over those from the container it was built from.
		b.Docker.ContainerConfig = *b.Docker.Config
	}
	b.Docker.Config = &b.Docker.ContainerConfig
	b.Docker.DockerVersion = ""
	now := time.Now().UTC()
	if b.Docker.Created.IsZero() {
		b.Docker.Created = now
	}
	if b.OCIv1.Created == nil || b.OCIv1.Created.IsZero() {
		b.OCIv1.Created = &now
	}
	if b.OS() == "" {
		b.SetOS(runtime.GOOS)
	}
	if b.Architecture() == "" {
		b.SetArchitecture(runtime.GOARCH)
	}
	if b.Format == Dockerv2ImageManifest && b.Hostname() == "" {
		b.SetHostname(stringid.TruncateID(stringid.GenerateRandomID()))
	}
}

// Annotations returns a set of key-value pairs from the image's manifest.
func (b *Builder) Annotations() map[string]string {
	return copyStringStringMap(b.ImageAnnotations)
}

// SetAnnotation adds or overwrites a key's value from the image's manifest.
// Note: this setting is not present in the Docker v2 image format, so it is
// discarded when writing images using Docker v2 formats.
func (b *Builder) SetAnnotation(key, value string) {
	if b.ImageAnnotations == nil {
		b.ImageAnnotations = map[string]string{}
	}
	b.ImageAnnotations[key] = value
}

// UnsetAnnotation removes a key and its value from the image's manifest, if
// it's present.
func (b *Builder) UnsetAnnotation(key string) {
	delete(b.ImageAnnotations, key)
}

// ClearAnnotations removes all keys and their values from the image's
// manifest.
func (b *Builder) ClearAnnotations() {
	b.ImageAnnotations = map[string]string{}
}

// CreatedBy returns a description of how this image was built.
func (b *Builder) CreatedBy() string {
	return b.ImageCreatedBy
}

// SetCreatedBy sets the description of how this image was built.
func (b *Builder) SetCreatedBy(how string) {
	b.ImageCreatedBy = how
}

// OS returns a name of the OS on which the container, or a container built
// using an image built from this container, is intended to be run.
func (b *Builder) OS() string {
	return b.OCIv1.OS
}

// SetOS sets the name of the OS on which the container, or a container built
// using an image built from this container, is intended to be run.
func (b *Builder) SetOS(os string) {
	b.OCIv1.OS = os
	b.Docker.OS = os
}

// Architecture returns a name of the architecture on which the container, or a
// container built using an image built from this container, is intended to be
// run.
func (b *Builder) Architecture() string {
	return b.OCIv1.Architecture
}

// SetArchitecture sets the name of the architecture on which the container, or
// a container built using an image built from this container, is intended to
// be run.
func (b *Builder) SetArchitecture(arch string) {
	b.OCIv1.Architecture = arch
	b.Docker.Architecture = arch
}

// Maintainer returns contact information for the person who built the image.
func (b *Builder) Maintainer() string {
	return b.OCIv1.Author
}

// SetMaintainer sets contact information for the person who built the image.
func (b *Builder) SetMaintainer(who string) {
	b.OCIv1.Author = who
	b.Docker.Author = who
}

// User returns information about the user as whom the container, or a
// container built using an image built from this container, should be run.
func (b *Builder) User() string {
	return b.OCIv1.Config.User
}

// SetUser sets information about the user as whom the container, or a
// container built using an image built from this container, should be run.
// Acceptable forms are a user name or ID, optionally followed by a colon and a
// group name or ID.
func (b *Builder) SetUser(spec string) {
	b.OCIv1.Config.User = spec
	b.Docker.Config.User = spec
}

// OnBuild returns the OnBuild value from the container.
func (b *Builder) OnBuild() []string {
	return copyStringSlice(b.Docker.Config.OnBuild)
}

// ClearOnBuild removes all values from the OnBuild structure
func (b *Builder) ClearOnBuild() {
	b.Docker.Config.OnBuild = []string{}
}

// SetOnBuild sets a trigger instruction to be executed when the image is used
// as the base of another image.
// Note: this setting is not present in the OCIv1 image format, so it is
// discarded when writing images using OCIv1 formats.
func (b *Builder) SetOnBuild(onBuild string) {
	if onBuild != "" && b.Format != Dockerv2ImageManifest {
		logrus.Errorf("ONBUILD is not supported for OCI image format, %s will be ignored. Must use `docker` format", onBuild)
	}
	b.Docker.Config.OnBuild = append(b.Docker.Config.OnBuild, onBuild)
}

// WorkDir returns the default working directory for running commands in the
// container, or in a container built using an image built from this container.
func (b *Builder) WorkDir() string {
	return b.OCIv1.Config.WorkingDir
}

// SetWorkDir sets the location of the default working directory for running
// commands in the container, or in a container built using an image built from
// this container.
func (b *Builder) SetWorkDir(there string) {
	b.OCIv1.Config.WorkingDir = there
	b.Docker.Config.WorkingDir = there
}

// Shell returns the default shell for running commands in the
// container, or in a container built using an image built from this container.
func (b *Builder) Shell() []string {
	return copyStringSlice(b.Docker.Config.Shell)
}

// SetShell sets the default shell for running
// commands in the container, or in a container built using an image built from
// this container.
// Note: this setting is not present in the OCIv1 image format, so it is
// discarded when writing images using OCIv1 formats.
func (b *Builder) SetShell(shell []string) {
	if len(shell) > 0 && b.Format != Dockerv2ImageManifest {
		logrus.Errorf("SHELL is not supported for OCI image format, %s will be ignored. Must use `docker` format", shell)
	}

	b.Docker.Config.Shell = copyStringSlice(shell)
}

// Env returns a list of key-value pairs to be set when running commands in the
// container, or in a container built using an image built from this container.
func (b *Builder) Env() []string {
	return copyStringSlice(b.OCIv1.Config.Env)
}

// SetEnv adds or overwrites a value to the set of environment strings which
// should be set when running commands in the container, or in a container
// built using an image built from this container.
func (b *Builder) SetEnv(k string, v string) {
	reset := func(s *[]string) {
		n := []string{}
		for i := range *s {
			if !strings.HasPrefix((*s)[i], k+"=") {
				n = append(n, (*s)[i])
			}
		}
		n = append(n, k+"="+v)
		*s = n
	}
	reset(&b.OCIv1.Config.Env)
	reset(&b.Docker.Config.Env)
}

// UnsetEnv removes a value from the set of environment strings which should be
// set when running commands in this container, or in a container built using
// an image built from this container.
func (b *Builder) UnsetEnv(k string) {
	unset := func(s *[]string) {
		n := []string{}
		for i := range *s {
			if !strings.HasPrefix((*s)[i], k+"=") {
				n = append(n, (*s)[i])
			}
		}
		*s = n
	}
	unset(&b.OCIv1.Config.Env)
	unset(&b.Docker.Config.Env)
}

// ClearEnv removes all values from the set of environment strings which should
// be set when running commands in this container, or in a container built
// using an image built from this container.
func (b *Builder) ClearEnv() {
	b.OCIv1.Config.Env = []string{}
	b.Docker.Config.Env = []string{}
}

// Cmd returns the default command, or command parameters if an Entrypoint is
// set, to use when running a container built from an image built from this
// container.
func (b *Builder) Cmd() []string {
	return copyStringSlice(b.OCIv1.Config.Cmd)
}

// SetCmd sets the default command, or command parameters if an Entrypoint is
// set, to use when running a container built from an image built from this
// container.
func (b *Builder) SetCmd(cmd []string) {
	b.OCIv1.Config.Cmd = copyStringSlice(cmd)
	b.Docker.Config.Cmd = copyStringSlice(cmd)
}

// Entrypoint returns the command to be run for containers built from images
// built from this container.
func (b *Builder) Entrypoint() []string {
	if len(b.OCIv1.Config.Entrypoint) > 0 {
		return copyStringSlice(b.OCIv1.Config.Entrypoint)
	}
	return nil
}

// SetEntrypoint sets the command to be run for in containers built from images
// built from this container.
func (b *Builder) SetEntrypoint(ep []string) {
	b.OCIv1.Config.Entrypoint = copyStringSlice(ep)
	b.Docker.Config.Entrypoint = copyStringSlice(ep)
}

// Labels returns a set of key-value pairs from the image's runtime
// configuration.
func (b *Builder) Labels() map[string]string {
	return copyStringStringMap(b.OCIv1.Config.Labels)
}

// SetLabel adds or overwrites a key's value from the image's runtime
// configuration.
func (b *Builder) SetLabel(k string, v string) {
	if b.OCIv1.Config.Labels == nil {
		b.OCIv1.Config.Labels = map[string]string{}
	}
	b.OCIv1.Config.Labels[k] = v
	if b.Docker.Config.Labels == nil {
		b.Docker.Config.Labels = map[string]string{}
	}
	b.Docker.Config.Labels[k] = v
}

// UnsetLabel removes a key and its value from the image's runtime
// configuration, if it's present.
func (b *Builder) UnsetLabel(k string) {
	delete(b.OCIv1.Config.Labels, k)
	delete(b.Docker.Config.Labels, k)
}

// ClearLabels removes all keys and their values from the image's runtime
// configuration.
func (b *Builder) ClearLabels() {
	b.OCIv1.Config.Labels = map[string]string{}
	b.Docker.Config.Labels = map[string]string{}
}

// Ports returns the set of ports which should be exposed when a container
// based on an image built from this container is run.
func (b *Builder) Ports() []string {
	p := []string{}
	for k := range b.OCIv1.Config.ExposedPorts {
		p = append(p, k)
	}
	return p
}

// SetPort adds or overwrites an exported port in the set of ports which should
// be exposed when a container based on an image built from this container is
// run.
func (b *Builder) SetPort(p string) {
	if b.OCIv1.Config.ExposedPorts == nil {
		b.OCIv1.Config.ExposedPorts = map[string]struct{}{}
	}
	b.OCIv1.Config.ExposedPorts[p] = struct{}{}
	if b.Docker.Config.ExposedPorts == nil {
		b.Docker.Config.ExposedPorts = make(docker.PortSet)
	}
	b.Docker.Config.ExposedPorts[docker.Port(p)] = struct{}{}
}

// UnsetPort removes an exposed port from the set of ports which should be
// exposed when a container based on an image built from this container is run.
func (b *Builder) UnsetPort(p string) {
	delete(b.OCIv1.Config.ExposedPorts, p)
	delete(b.Docker.Config.ExposedPorts, docker.Port(p))
}

// ClearPorts empties the set of ports which should be exposed when a container
// based on an image built from this container is run.
func (b *Builder) ClearPorts() {
	b.OCIv1.Config.ExposedPorts = map[string]struct{}{}
	b.Docker.Config.ExposedPorts = docker.PortSet{}
}

// Volumes returns a list of filesystem locations which should be mounted from
// outside of the container when a container built from an image built from
// this container is run.
func (b *Builder) Volumes() []string {
	v := []string{}
	for k := range b.OCIv1.Config.Volumes {
		v = append(v, k)
	}
	if len(v) > 0 {
		return v
	}
	return nil
}

// CheckVolume returns True if the location exists in the image's list of locations
// which should be mounted from outside of the container when a container
// based on an image built from this container is run

func (b *Builder) CheckVolume(v string) bool {
	_, OCIv1Volume := b.OCIv1.Config.Volumes[v]
	_, DockerVolume := b.Docker.Config.Volumes[v]
	return OCIv1Volume || DockerVolume
}

// AddVolume adds a location to the image's list of locations which should be
// mounted from outside of the container when a container based on an image
// built from this container is run.
func (b *Builder) AddVolume(v string) {
	if b.OCIv1.Config.Volumes == nil {
		b.OCIv1.Config.Volumes = map[string]struct{}{}
	}
	b.OCIv1.Config.Volumes[v] = struct{}{}
	if b.Docker.Config.Volumes == nil {
		b.Docker.Config.Volumes = map[string]struct{}{}
	}
	b.Docker.Config.Volumes[v] = struct{}{}
}

// RemoveVolume removes a location from the list of locations which should be
// mounted from outside of the container when a container based on an image
// built from this container is run.
func (b *Builder) RemoveVolume(v string) {
	delete(b.OCIv1.Config.Volumes, v)
	delete(b.Docker.Config.Volumes, v)
}

// ClearVolumes removes all locations from the image's list of locations which
// should be mounted from outside of the container when a container based on an
// image built from this container is run.
func (b *Builder) ClearVolumes() {
	b.OCIv1.Config.Volumes = map[string]struct{}{}
	b.Docker.Config.Volumes = map[string]struct{}{}
}

// Hostname returns the hostname which will be set in the container and in
// containers built using images built from the container.
func (b *Builder) Hostname() string {
	return b.Docker.Config.Hostname
}

// SetHostname sets the hostname which will be set in the container and in
// containers built using images built from the container.
// Note: this setting is not present in the OCIv1 image format, so it is
// discarded when writing images using OCIv1 formats.
func (b *Builder) SetHostname(name string) {
	b.Docker.Config.Hostname = name
}

// Domainname returns the domainname which will be set in the container and in
// containers built using images built from the container.
func (b *Builder) Domainname() string {
	return b.Docker.Config.Domainname
}

// SetDomainname sets the domainname which will be set in the container and in
// containers built using images built from the container.
// Note: this setting is not present in the OCIv1 image format, so it is
// discarded when writing images using OCIv1 formats.
func (b *Builder) SetDomainname(name string) {
	if name != "" && b.Format != Dockerv2ImageManifest {
		logrus.Errorf("DOMAINNAME is not supported for OCI image format, domainname %s will be ignored. Must use `docker` format", name)
	}
	b.Docker.Config.Domainname = name
}

// SetDefaultMountsFilePath sets the mounts file path for testing purposes
func (b *Builder) SetDefaultMountsFilePath(path string) {
	b.DefaultMountsFilePath = path
}

// Comment returns the comment which will be set in the container and in
// containers built using images built from the container
func (b *Builder) Comment() string {
	return b.Docker.Comment
}

// SetComment sets the comment which will be set in the container and in
// containers built using images built from the container.
// Note: this setting is not present in the OCIv1 image format, so it is
// discarded when writing images using OCIv1 formats.
func (b *Builder) SetComment(comment string) {
	if comment != "" && b.Format != Dockerv2ImageManifest {
		logrus.Errorf("COMMENT is not supported for OCI image format, comment %s will be ignored. Must use `docker` format", comment)
	}
	b.Docker.Comment = comment
}

// HistoryComment returns the comment which will be used in the history item
// which will describe the latest layer when we commit an image.
func (b *Builder) HistoryComment() string {
	return b.ImageHistoryComment
}

// SetHistoryComment sets the comment which will be used in the history item
// which will describe the latest layer when we commit an image.
func (b *Builder) SetHistoryComment(comment string) {
	b.ImageHistoryComment = comment
}

// StopSignal returns the signal which will be set in the container and in
// containers built using images buiilt from the container
func (b *Builder) StopSignal() string {
	return b.Docker.Config.StopSignal
}

// SetStopSignal sets the signal which will be set in the container and in
// containers built using images built from the container.
func (b *Builder) SetStopSignal(stopSignal string) {
	b.OCIv1.Config.StopSignal = stopSignal
	b.Docker.Config.StopSignal = stopSignal
}

// Healthcheck returns information that recommends how a container engine
// should check if a running container is "healthy".
func (b *Builder) Healthcheck() *docker.HealthConfig {
	if b.Docker.Config.Healthcheck == nil {
		return nil
	}
	return &docker.HealthConfig{
		Test:        copyStringSlice(b.Docker.Config.Healthcheck.Test),
		Interval:    b.Docker.Config.Healthcheck.Interval,
		Timeout:     b.Docker.Config.Healthcheck.Timeout,
		StartPeriod: b.Docker.Config.Healthcheck.StartPeriod,
		Retries:     b.Docker.Config.Healthcheck.Retries,
	}
}

// SetHealthcheck sets recommended commands to run in order to verify that a
// running container based on this image is "healthy", along with information
// specifying how often that test should be run, and how many times the test
// should fail before the container should be considered unhealthy.
// Note: this setting is not present in the OCIv1 image format, so it is
// discarded when writing images using OCIv1 formats.
func (b *Builder) SetHealthcheck(config *docker.HealthConfig) {
	b.Docker.Config.Healthcheck = nil
	if config != nil {
		b.Docker.Config.Healthcheck = &docker.HealthConfig{
			Test:        copyStringSlice(config.Test),
			Interval:    config.Interval,
			Timeout:     config.Timeout,
			StartPeriod: config.StartPeriod,
			Retries:     config.Retries,
		}
	}
}

// AddPrependedEmptyLayer adds an item to the history that we'll create when
// committing the image, after any history we inherit from the base image, but
// before the history item that we'll use to describe the new layer that we're
// adding.
func (b *Builder) AddPrependedEmptyLayer(created *time.Time, createdBy, author, comment string) {
	if created != nil {
		copiedTimestamp := *created
		created = &copiedTimestamp
	}
	b.PrependedEmptyLayers = append(b.PrependedEmptyLayers, ociv1.History{
		Created:    created,
		CreatedBy:  createdBy,
		Author:     author,
		Comment:    comment,
		EmptyLayer: true,
	})
}

// ClearPrependedEmptyLayers clears the list of history entries that we'll add
// to the committed image before the entry for the layer that we're adding.
func (b *Builder) ClearPrependedEmptyLayers() {
	b.PrependedEmptyLayers = nil
}

// AddAppendedEmptyLayer adds an item to the history that we'll create when
// committing the image, after the history item that we'll use to describe the
// new layer that we're adding.
func (b *Builder) AddAppendedEmptyLayer(created *time.Time, createdBy, author, comment string) {
	if created != nil {
		copiedTimestamp := *created
		created = &copiedTimestamp
	}
	b.AppendedEmptyLayers = append(b.AppendedEmptyLayers, ociv1.History{
		Created:    created,
		CreatedBy:  createdBy,
		Author:     author,
		Comment:    comment,
		EmptyLayer: true,
	})
}

// ClearAppendedEmptyLayers clears the list of history entries that we'll add
// to the committed image after the entry for the layer that we're adding.
func (b *Builder) ClearAppendedEmptyLayers() {
	b.AppendedEmptyLayers = nil
}
