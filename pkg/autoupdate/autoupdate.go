package autoupdate

import (
	"context"
	"fmt"
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

// policyMappers assembles update tasks by policy
type policyMapper map[Policy][]*task

// updater includes shared state for auto-updating one or more containers.
type updater struct {
	conn                *dbus.Conn
	idToImage           map[string]*libimage.Image
	imageToPolicyMapper map[string]policyMapper
	options             *entities.AutoUpdateOptions
	updatedRawImages    map[string]bool
	runtime             *libpod.Runtime
}

// task includes data and state for updating a container
type task struct {
	container *libpod.Container // Container to update
	policy    Policy            // Update policy
	image     *libimage.Image   // Original image before the update
	unit      string            // Name of the systemd unit
}

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

	return "", fmt.Errorf("invalid auto-update policy %q: valid policies are %+q", s, keys)
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
		return fmt.Errorf("auto updates require the docker image transport but image is of transport %q", imageRef.Transport().Name())
	} else if err != nil {
		repo, err := reference.Parse(imageName)
		if err != nil {
			return fmt.Errorf("enforcing fully-qualified docker transport reference for auto updates: %w", err)
		}
		if _, ok := repo.(reference.NamedTagged); !ok {
			return fmt.Errorf("auto updates require fully-qualified image references (no tag): %q", imageName)
		}
		if _, ok := repo.(reference.Digested); ok {
			return fmt.Errorf("auto updates require fully-qualified image references without digest: %q", imageName)
		}
	}
	return nil
}

func (u *updater) assembleImageMap(ctx context.Context) error {
	// Create a map from `image ID -> *libimage.Image` for image lookups.
	listOptions := &libimage.ListImagesOptions{
		Filters: []string{"readonly=false"},
	}
	imagesSlice, err := u.runtime.LibimageRuntime().ListImages(ctx, nil, listOptions)
	if err != nil {
		return err
	}
	imageMap := make(map[string]*libimage.Image)
	for i := range imagesSlice {
		imageMap[imagesSlice[i].ID()] = imagesSlice[i]
	}

	u.idToImage = imageMap
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
	auto := updater{
		options:          &options,
		runtime:          runtime,
		updatedRawImages: make(map[string]bool),
	}

	// Assemble a map `image ID -> *libimage.Image` that we can consult
	// later on for lookups.
	if err := auto.assembleImageMap(ctx); err != nil {
		return nil, []error{err}
	}

	// Create a map from `image ID -> []*Container`.
	if errs := auto.imageContainersMap(); len(errs) > 0 {
		return nil, errs
	}

	// Nothing to do.
	if len(auto.imageToPolicyMapper) == 0 {
		return nil, nil
	}

	// Connect to DBUS.
	conn, err := systemd.ConnectToDBUS()
	if err != nil {
		logrus.Errorf(err.Error())
		return nil, []error{err}
	}
	defer conn.Close()
	auto.conn = conn

	runtime.NewSystemEvent(events.AutoUpdate)

	// Update all images/container according to their auto-update policy.
	var allReports []*entities.AutoUpdateReport
	var errs []error
	for imageID, policyMapper := range auto.imageToPolicyMapper {
		if _, exists := auto.idToImage[imageID]; !exists {
			errs = append(errs, fmt.Errorf("container image ID %q not found in local storage", imageID))
			return nil, errs
		}

		for _, task := range policyMapper[PolicyRegistryImage] {
			report, err := auto.updateRegistry(ctx, task)
			if err != nil {
				errs = append(errs, err)
			}
			if report != nil {
				allReports = append(allReports, report)
			}
		}

		for _, task := range policyMapper[PolicyLocalImage] {
			report, err := auto.updateLocally(ctx, task)
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

// updateRegistry updates the image/container according to the "registry" policy.
func (u *updater) updateRegistry(ctx context.Context, task *task) (*entities.AutoUpdateReport, error) {
	cid := task.container.ID()
	rawImageName := task.container.RawImageName()
	if rawImageName == "" {
		return nil, fmt.Errorf("registry auto-updating container %q: raw-image name is empty", cid)
	}

	if task.unit == "" {
		return nil, fmt.Errorf("auto-updating container %q: no %s label found", task.container.ID(), systemdDefine.EnvVariable)
	}

	report := &entities.AutoUpdateReport{
		ContainerID:   cid,
		ContainerName: task.container.Name(),
		ImageName:     rawImageName,
		Policy:        PolicyRegistryImage,
		SystemdUnit:   task.unit,
		Updated:       "failed",
	}

	if _, updated := u.updatedRawImages[rawImageName]; updated {
		logrus.Infof("Auto-updating container %q using registry image %q", cid, rawImageName)
		if err := u.restartSystemdUnit(ctx, task.container, task.unit); err != nil {
			return report, err
		}
		report.Updated = "true"
		return report, nil
	}

	authfile := getAuthfilePath(task.container, u.options)
	needsUpdate, err := newerRemoteImageAvailable(ctx, task.image, rawImageName, authfile)
	if err != nil {
		return report, fmt.Errorf("registry auto-updating container %q: image check for %q failed: %w", cid, rawImageName, err)
	}

	if !needsUpdate {
		report.Updated = "false"
		return report, nil
	}

	if u.options.DryRun {
		report.Updated = "pending"
		return report, nil
	}

	if _, err := updateImage(ctx, u.runtime, rawImageName, authfile); err != nil {
		return report, fmt.Errorf("registry auto-updating container %q: image update for %q failed: %w", cid, rawImageName, err)
	}
	u.updatedRawImages[rawImageName] = true

	logrus.Infof("Auto-updating container %q using registry image %q", cid, rawImageName)
	updateErr := u.restartSystemdUnit(ctx, task.container, task.unit)
	if updateErr == nil {
		report.Updated = "true"
		return report, nil
	}

	if !u.options.Rollback {
		return report, updateErr
	}

	// To fallback, simply retag the old image and restart the service.
	if err := task.image.Tag(rawImageName); err != nil {
		return report, fmt.Errorf("falling back to previous image: %w", err)
	}
	if err := u.restartSystemdUnit(ctx, task.container, task.unit); err != nil {
		return report, fmt.Errorf("restarting unit with old image during fallback: %w", err)
	}

	report.Updated = "rolled back"
	return report, nil
}

// updateRegistry updates the image/container according to the "local" policy.
func (u *updater) updateLocally(ctx context.Context, task *task) (*entities.AutoUpdateReport, error) {
	cid := task.container.ID()
	rawImageName := task.container.RawImageName()
	if rawImageName == "" {
		return nil, fmt.Errorf("locally auto-updating container %q: raw-image name is empty", cid)
	}

	if task.unit == "" {
		return nil, fmt.Errorf("auto-updating container %q: no %s label found", task.container.ID(), systemdDefine.EnvVariable)
	}

	report := &entities.AutoUpdateReport{
		ContainerID:   cid,
		ContainerName: task.container.Name(),
		ImageName:     rawImageName,
		Policy:        PolicyLocalImage,
		SystemdUnit:   task.unit,
		Updated:       "failed",
	}

	needsUpdate, err := newerLocalImageAvailable(u.runtime, task.image, rawImageName)
	if err != nil {
		return report, fmt.Errorf("locally auto-updating container %q: image check for %q failed: %w", cid, rawImageName, err)
	}

	if !needsUpdate {
		report.Updated = "false"
		return report, nil
	}

	if u.options.DryRun {
		report.Updated = "pending"
		return report, nil
	}

	logrus.Infof("Auto-updating container %q using local image %q", cid, rawImageName)
	updateErr := u.restartSystemdUnit(ctx, task.container, task.unit)
	if updateErr == nil {
		report.Updated = "true"
		return report, nil
	}

	if !u.options.Rollback {
		return report, updateErr
	}

	// To fallback, simply retag the old image and restart the service.
	if err := task.image.Tag(rawImageName); err != nil {
		return report, fmt.Errorf("falling back to previous image: %w", err)
	}
	if err := u.restartSystemdUnit(ctx, task.container, task.unit); err != nil {
		return report, fmt.Errorf("restarting unit with old image during fallback: %w", err)
	}

	report.Updated = "rolled back"
	return report, nil
}

// restartSystemdUnit restarts the systemd unit the container is running in.
func (u *updater) restartSystemdUnit(ctx context.Context, ctr *libpod.Container, unit string) error {
	restartChan := make(chan string)
	if _, err := u.conn.RestartUnitContext(ctx, unit, "replace", restartChan); err != nil {
		return fmt.Errorf("auto-updating container %q: restarting systemd unit %q failed: %w", ctr.ID(), unit, err)
	}

	// Wait for the restart to finish and actually check if it was
	// successful or not.
	result := <-restartChan

	switch result {
	case "done":
		logrus.Infof("Successfully restarted systemd unit %q of container %q", unit, ctr.ID())
		return nil

	default:
		return fmt.Errorf("auto-updating container %q: restarting systemd unit %q failed: expected %q but received %q", ctr.ID(), unit, "done", result)
	}
}

// imageContainersMap generates a map[image ID] -> [containers using the image]
// of all containers with a valid auto-update policy.
func (u *updater) imageContainersMap() []error {
	allContainers, err := u.runtime.GetAllContainers()
	if err != nil {
		return []error{err}
	}

	u.imageToPolicyMapper = make(map[string]policyMapper)

	errors := []error{}
	for _, c := range allContainers {
		ctr := c
		state, err := ctr.State()
		if err != nil {
			errors = append(errors, err)
			continue
		}
		// Only update running containers.
		if state != define.ContainerStateRunning {
			continue
		}

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
		}

		id, _ := ctr.Image()
		policyMap, exists := u.imageToPolicyMapper[id]
		if !exists {
			policyMap = make(map[Policy][]*task)
		}

		image, exists := u.idToImage[id]
		if !exists {
			err := fmt.Errorf("internal error: no image found for ID %s", id)
			errors = append(errors, err)
			continue
		}

		unit, _ := labels[systemdDefine.EnvVariable]

		t := task{
			container: ctr,
			policy:    policy,
			image:     image,
			unit:      unit,
		}
		policyMap[policy] = append(policyMap[policy], &t)
		u.imageToPolicyMapper[id] = policyMap
	}

	return errors
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
