package autoupdate

import (
	"context"
	"os"
	"sort"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/libpod/events"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/systemd"
	systemdDefine "github.com/containers/podman/v4/pkg/systemd/define"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Label denotes the container/pod label key to specify auto-update policies in
// container labels.
const Label = "io.containers.autoupdate"

// Label denotes the container label key to specify authfile in
// container labels.
const AuthfileLabel = "io.containers.autoupdate.authfile"

// Policy represents an auto-update policy.
type Policy string

const (
	// PolicyDefault is the default policy denoting no auto updates.
	PolicyDefault Policy = "disabled"
	// PolicyRegistryImage is the policy to update as soon as there's a new image found.
	PolicyRegistryImage = "registry"
	// PolicyLocalImage is the policy to run auto-update based on a local image
	PolicyLocalImage = "local"
)

// Map for easy lookups of supported policies.
var supportedPolicies = map[string]Policy{
	"":         PolicyDefault,
	"disabled": PolicyDefault,
	"image":    PolicyRegistryImage,
	"registry": PolicyRegistryImage,
	"local":    PolicyLocalImage,
}

// policyMapper is used for tying a container to it's autoupdate policy
type policyMapper map[Policy][]*libpod.Container

// LookupPolicy looks up the corresponding Policy for the specified
// string. If none is found, an errors is returned including the list of
// supported policies.
//
// Note that an empty string resolved to PolicyDefault.
func LookupPolicy(s string) (Policy, error) {
	policy, exists := supportedPolicies[s]
	if exists {
		return policy, nil
	}

	// Sort the keys first as maps are non-deterministic.
	keys := []string{}
	for k := range supportedPolicies {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	return "", errors.Errorf("invalid auto-update policy %q: valid policies are %+q", s, keys)
}

// ValidateImageReference checks if the specified imageName is a fully-qualified
// image reference to the docker transport (without digest).  Such a reference
// includes a domain, name and tag (e.g., quay.io/podman/stable:latest).  The
// reference may also be prefixed with "docker://" explicitly indicating that
// it's a reference to the docker transport.
func ValidateImageReference(imageName string) error {
	// Make sure the input image is a docker.
	imageRef, err := alltransports.ParseImageName(imageName)
	if err == nil && imageRef.Transport().Name() != docker.Transport.Name() {
		return errors.Errorf("auto updates require the docker image transport but image is of transport %q", imageRef.Transport().Name())
	} else if err != nil {
		repo, err := reference.Parse(imageName)
		if err != nil {
			return errors.Wrap(err, "enforcing fully-qualified docker transport reference for auto updates")
		}
		if _, ok := repo.(reference.NamedTagged); !ok {
			return errors.Errorf("auto updates require fully-qualified image references (no tag): %q", imageName)
		}
		if _, ok := repo.(reference.Digested); ok {
			return errors.Errorf("auto updates require fully-qualified image references without digest: %q", imageName)
		}
	}
	return nil
}

// AutoUpdate looks up containers with a specified auto-update policy and acts
// accordingly.
//
// If the policy is set to PolicyRegistryImage, it checks if the image
// on the remote registry is different than the local one. If the image digests
// differ, it pulls the remote image and restarts the systemd unit running the
// container.
//
// If the policy is set to PolicyLocalImage, it checks if the image
// of a running container is different than the local one. If the image digests
// differ, it restarts the systemd unit with the new image.
//
// It returns a slice of successfully restarted systemd units and a slice of
// errors encountered during auto update.
func AutoUpdate(ctx context.Context, runtime *libpod.Runtime, options entities.AutoUpdateOptions) ([]*entities.AutoUpdateReport, []error) {
	// Create a map from `image ID -> []*Container`.
	containerMap, errs := imageContainersMap(runtime)
	if len(containerMap) == 0 {
		return nil, errs
	}

	// Create a map from `image ID -> *libimage.Image` for image lookups.
	listOptions := &libimage.ListImagesOptions{
		Filters: []string{"readonly=false"},
	}
	imagesSlice, err := runtime.LibimageRuntime().ListImages(ctx, nil, listOptions)
	if err != nil {
		return nil, []error{err}
	}
	imageMap := make(map[string]*libimage.Image)
	for i := range imagesSlice {
		imageMap[imagesSlice[i].ID()] = imagesSlice[i]
	}

	// Connect to DBUS.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		logrus.Errorf(err.Error())
		return nil, []error{err}
	}
	defer conn.Close()

	runtime.NewSystemEvent(events.AutoUpdate)

	// Update all images/container according to their auto-update policy.
	var allReports []*entities.AutoUpdateReport
	updatedRawImages := make(map[string]bool)
	for imageID, policyMapper := range containerMap {
		image, exists := imageMap[imageID]
		if !exists {
			errs = append(errs, errors.Errorf("container image ID %q not found in local storage", imageID))
			return nil, errs
		}

		for _, ctr := range policyMapper[PolicyRegistryImage] {
			report, err := autoUpdateRegistry(ctx, image, ctr, updatedRawImages, &options, conn, runtime)
			if err != nil {
				errs = append(errs, err)
			}
			if report != nil {
				allReports = append(allReports, report)
			}
		}

		for _, ctr := range policyMapper[PolicyLocalImage] {
			report, err := autoUpdateLocally(ctx, image, ctr, &options, conn, runtime)
			if err != nil {
				errs = append(errs, err)
			}
			if report != nil {
				allReports = append(allReports, report)
			}
		}
	}

	return allReports, errs
}

// autoUpdateRegistry updates the image/container according to the "registry" policy.
func autoUpdateRegistry(ctx context.Context, image *libimage.Image, ctr *libpod.Container, updatedRawImages map[string]bool, options *entities.AutoUpdateOptions, conn *dbus.Conn, runtime *libpod.Runtime) (*entities.AutoUpdateReport, error) {
	cid := ctr.ID()
	rawImageName := ctr.RawImageName()
	if rawImageName == "" {
		return nil, errors.Errorf("registry auto-updating container %q: raw-image name is empty", cid)
	}

	labels := ctr.Labels()
	unit, exists := labels[systemdDefine.EnvVariable]
	if !exists {
		return nil, errors.Errorf("auto-updating container %q: no %s label found", ctr.ID(), systemdDefine.EnvVariable)
	}

	report := &entities.AutoUpdateReport{
		ContainerID:   cid,
		ContainerName: ctr.Name(),
		ImageName:     rawImageName,
		Policy:        PolicyRegistryImage,
		SystemdUnit:   unit,
		Updated:       "failed",
	}

	if _, updated := updatedRawImages[rawImageName]; updated {
		logrus.Infof("Auto-updating container %q using registry image %q", cid, rawImageName)
		if err := restartSystemdUnit(ctx, ctr, unit, conn); err != nil {
			return report, err
		}
		report.Updated = "true"
		return report, nil
	}

	authfile := getAuthfilePath(ctr, options)
	needsUpdate, err := newerRemoteImageAvailable(ctx, image, rawImageName, authfile)
	if err != nil {
		return report, errors.Wrapf(err, "registry auto-updating container %q: image check for %q failed", cid, rawImageName)
	}

	if !needsUpdate {
		report.Updated = "false"
		return report, nil
	}

	if options.DryRun {
		report.Updated = "pending"
		return report, nil
	}

	if _, err := updateImage(ctx, runtime, rawImageName, authfile); err != nil {
		return report, errors.Wrapf(err, "registry auto-updating container %q: image update for %q failed", cid, rawImageName)
	}
	updatedRawImages[rawImageName] = true

	logrus.Infof("Auto-updating container %q using registry image %q", cid, rawImageName)
	updateErr := restartSystemdUnit(ctx, ctr, unit, conn)
	if updateErr == nil {
		report.Updated = "true"
		return report, nil
	}

	if !options.Rollback {
		return report, updateErr
	}

	// To fallback, simply retag the old image and restart the service.
	if err := image.Tag(rawImageName); err != nil {
		return report, errors.Wrap(err, "falling back to previous image")
	}
	if err := restartSystemdUnit(ctx, ctr, unit, conn); err != nil {
		return report, errors.Wrap(err, "restarting unit with old image during fallback")
	}

	report.Updated = "rolled back"
	return report, nil
}

// autoUpdateRegistry updates the image/container according to the "local" policy.
func autoUpdateLocally(ctx context.Context, image *libimage.Image, ctr *libpod.Container, options *entities.AutoUpdateOptions, conn *dbus.Conn, runtime *libpod.Runtime) (*entities.AutoUpdateReport, error) {
	cid := ctr.ID()
	rawImageName := ctr.RawImageName()
	if rawImageName == "" {
		return nil, errors.Errorf("locally auto-updating container %q: raw-image name is empty", cid)
	}

	labels := ctr.Labels()
	unit, exists := labels[systemdDefine.EnvVariable]
	if !exists {
		return nil, errors.Errorf("auto-updating container %q: no %s label found", ctr.ID(), systemdDefine.EnvVariable)
	}

	report := &entities.AutoUpdateReport{
		ContainerID:   cid,
		ContainerName: ctr.Name(),
		ImageName:     rawImageName,
		Policy:        PolicyLocalImage,
		SystemdUnit:   unit,
		Updated:       "failed",
	}

	needsUpdate, err := newerLocalImageAvailable(runtime, image, rawImageName)
	if err != nil {
		return report, errors.Wrapf(err, "locally auto-updating container %q: image check for %q failed", cid, rawImageName)
	}

	if !needsUpdate {
		report.Updated = "false"
		return report, nil
	}

	if options.DryRun {
		report.Updated = "pending"
		return report, nil
	}

	logrus.Infof("Auto-updating container %q using local image %q", cid, rawImageName)
	updateErr := restartSystemdUnit(ctx, ctr, unit, conn)
	if updateErr == nil {
		report.Updated = "true"
		return report, nil
	}

	if !options.Rollback {
		return report, updateErr
	}

	// To fallback, simply retag the old image and restart the service.
	if err := image.Tag(rawImageName); err != nil {
		return report, errors.Wrap(err, "falling back to previous image")
	}
	if err := restartSystemdUnit(ctx, ctr, unit, conn); err != nil {
		return report, errors.Wrap(err, "restarting unit with old image during fallback")
	}

	report.Updated = "rolled back"
	return report, nil
}

// restartSystemdUnit restarts the systemd unit the container is running in.
func restartSystemdUnit(ctx context.Context, ctr *libpod.Container, unit string, conn *dbus.Conn) error {
	restartChan := make(chan string)
	if _, err := conn.RestartUnitContext(ctx, unit, "replace", restartChan); err != nil {
		return errors.Wrapf(err, "auto-updating container %q: restarting systemd unit %q failed", ctr.ID(), unit)
	}

	// Wait for the restart to finish and actually check if it was
	// successful or not.
	result := <-restartChan

	switch result {
	case "done":
		logrus.Infof("Successfully restarted systemd unit %q of container %q", unit, ctr.ID())
		return nil

	default:
		return errors.Errorf("auto-updating container %q: restarting systemd unit %q failed: expected %q but received %q", ctr.ID(), unit, "done", result)
	}
}

// imageContainersMap generates a map[image ID] -> [containers using the image]
// of all containers with a valid auto-update policy.
func imageContainersMap(runtime *libpod.Runtime) (map[string]policyMapper, []error) {
	allContainers, err := runtime.GetAllContainers()
	if err != nil {
		return nil, []error{err}
	}

	errors := []error{}
	containerMap := make(map[string]policyMapper)
	for _, ctr := range allContainers {
		state, err := ctr.State()
		if err != nil {
			errors = append(errors, err)
			continue
		}
		// Only update running containers.
		if state != define.ContainerStateRunning {
			continue
		}

		// Only update containers with the specific label/policy set.
		labels := ctr.Labels()
		value, exists := labels[Label]
		if !exists {
			continue
		}

		policy, err := LookupPolicy(value)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// Skip labels not related to autoupdate
		if policy == PolicyDefault {
			continue
		} else {
			id, _ := ctr.Image()
			policyMap, exists := containerMap[id]
			if !exists {
				policyMap = make(map[Policy][]*libpod.Container)
			}
			policyMap[policy] = append(policyMap[policy], ctr)
			containerMap[id] = policyMap
			// Now we know that `ctr` is configured for auto updates.
		}
	}

	return containerMap, errors
}

// getAuthfilePath returns an authfile path, if set. The authfile label in the
// container, if set, as precedence over the one set in the options.
func getAuthfilePath(ctr *libpod.Container, options *entities.AutoUpdateOptions) string {
	labels := ctr.Labels()
	authFilePath, exists := labels[AuthfileLabel]
	if exists {
		return authFilePath
	}
	return options.Authfile
}

// newerRemoteImageAvailable returns true if there corresponding image on the remote
// registry is newer.
func newerRemoteImageAvailable(ctx context.Context, img *libimage.Image, origName string, authfile string) (bool, error) {
	remoteRef, err := docker.ParseReference("//" + origName)
	if err != nil {
		return false, err
	}
	options := &libimage.HasDifferentDigestOptions{AuthFilePath: authfile}
	return img.HasDifferentDigest(ctx, remoteRef, options)
}

// newerLocalImageAvailable returns true if the container and local image have different digests
func newerLocalImageAvailable(runtime *libpod.Runtime, img *libimage.Image, rawImageName string) (bool, error) {
	localImg, _, err := runtime.LibimageRuntime().LookupImage(rawImageName, nil)
	if err != nil {
		return false, err
	}
	return localImg.Digest().String() != img.Digest().String(), nil
}

// updateImage pulls the specified image.
func updateImage(ctx context.Context, runtime *libpod.Runtime, name, authfile string) (*libimage.Image, error) {
	pullOptions := &libimage.PullOptions{}
	pullOptions.AuthFilePath = authfile
	pullOptions.Writer = os.Stderr

	pulledImages, err := runtime.LibimageRuntime().Pull(ctx, name, config.PullPolicyAlways, pullOptions)
	if err != nil {
		return nil, err
	}
	return pulledImages[0], nil
}
