package autoupdate

import (
	"context"
	"os"
	"sort"

	"github.com/containers/common/libimage"
	"github.com/containers/common/pkg/config"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/podman/v3/libpod"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/systemd"
	systemdDefine "github.com/containers/podman/v3/pkg/systemd/define"
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

// Options include parameters for auto updates.
type Options struct {
	// Authfile to use when contacting registries.
	Authfile string
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
			return errors.Wrap(err, "error enforcing fully-qualified docker transport reference for auto updates")
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
func AutoUpdate(runtime *libpod.Runtime, options Options) ([]string, []error) {
	// Create a map from `image ID -> []*Container`.
	containerMap, errs := imageContainersMap(runtime)
	if len(containerMap) == 0 {
		return nil, errs
	}

	// Create a map from `image ID -> *libimage.Image` for image lookups.
	listOptions := &libimage.ListImagesOptions{
		Filters: []string{"readonly=false"},
	}
	imagesSlice, err := runtime.LibimageRuntime().ListImages(context.Background(), nil, listOptions)
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

	// Update images.
	containersToRestart := []*libpod.Container{}
	updatedRawImages := make(map[string]bool)
	for imageID, policyMapper := range containerMap {
		image, exists := imageMap[imageID]
		if !exists {
			errs = append(errs, errors.Errorf("container image ID %q not found in local storage", imageID))
			return nil, errs
		}
		// Now we have to check if the image of any containers must be updated.
		// Note that the image ID is NOT enough for this check as a given image
		// may have multiple tags.
		for _, registryCtr := range policyMapper[PolicyRegistryImage] {
			cid := registryCtr.ID()
			rawImageName := registryCtr.RawImageName()
			if rawImageName == "" {
				errs = append(errs, errors.Errorf("error registry auto-updating container %q: raw-image name is empty", cid))
			}
			readAuthenticationPath(registryCtr, options)
			needsUpdate, err := newerRemoteImageAvailable(runtime, image, rawImageName, options)
			if err != nil {
				errs = append(errs, errors.Wrapf(err, "error registry auto-updating container %q: image check for %q failed", cid, rawImageName))
				continue
			}

			if needsUpdate {
				logrus.Infof("Auto-updating container %q using registry image %q", cid, rawImageName)
				if _, updated := updatedRawImages[rawImageName]; !updated {
					_, err = updateImage(runtime, rawImageName, options)
					if err != nil {
						errs = append(errs, errors.Wrapf(err, "error registry auto-updating container %q: image update for %q failed", cid, rawImageName))
						continue
					}
					updatedRawImages[rawImageName] = true
				}
				containersToRestart = append(containersToRestart, registryCtr)
			}
		}

		for _, localCtr := range policyMapper[PolicyLocalImage] {
			cid := localCtr.ID()
			rawImageName := localCtr.RawImageName()
			if rawImageName == "" {
				errs = append(errs, errors.Errorf("error locally auto-updating container %q: raw-image name is empty", cid))
			}
			// This avoids restarting containers unnecessarily.
			needsUpdate, err := newerLocalImageAvailable(runtime, image, rawImageName)
			if err != nil {
				errs = append(errs, errors.Wrapf(err, "error locally auto-updating container %q: image check for %q failed", cid, rawImageName))
				continue
			}

			if needsUpdate {
				logrus.Infof("Auto-updating container %q using local image %q", cid, rawImageName)
				containersToRestart = append(containersToRestart, localCtr)
			}
		}
	}

	// Restart containers.
	updatedUnits := []string{}
	for _, ctr := range containersToRestart {
		labels := ctr.Labels()
		unit, exists := labels[systemdDefine.EnvVariable]
		if !exists {
			// Shouldn't happen but let's be sure of it.
			errs = append(errs, errors.Errorf("error auto-updating container %q: no %s label found", ctr.ID(), systemdDefine.EnvVariable))
			continue
		}
		_, err := conn.RestartUnit(unit, "replace", nil)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "error auto-updating container %q: restarting systemd unit %q failed", ctr.ID(), unit))
			continue
		}
		logrus.Infof("Successfully restarted systemd unit %q", unit)
		updatedUnits = append(updatedUnits, unit)
	}

	return updatedUnits, errs
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

// readAuthenticationPath reads a container's labels and reads authentication path into options
func readAuthenticationPath(ctr *libpod.Container, options Options) {
	labels := ctr.Labels()
	authFilePath, exists := labels[AuthfileLabel]
	if exists {
		options.Authfile = authFilePath
	}
}

// newerRemoteImageAvailable returns true if there corresponding image on the remote
// registry is newer.
func newerRemoteImageAvailable(runtime *libpod.Runtime, img *libimage.Image, origName string, options Options) (bool, error) {
	remoteRef, err := docker.ParseReference("//" + origName)
	if err != nil {
		return false, err
	}

	data, err := img.Inspect(context.Background(), false)
	if err != nil {
		return false, err
	}

	sys := runtime.SystemContext()
	sys.AuthFilePath = options.Authfile

	// We need to account for the arch that the image uses.  It seems
	// common on ARM to tweak this option to pull the correct image.  See
	// github.com/containers/podman/issues/6613.
	sys.ArchitectureChoice = data.Architecture

	remoteImg, err := remoteRef.NewImage(context.Background(), sys)
	if err != nil {
		return false, err
	}

	rawManifest, _, err := remoteImg.Manifest(context.Background())
	if err != nil {
		return false, err
	}

	remoteDigest, err := manifest.Digest(rawManifest)
	if err != nil {
		return false, err
	}

	return img.Digest().String() != remoteDigest.String(), nil
}

// newerLocalImageAvailable returns true if the container and local image have different digests
func newerLocalImageAvailable(runtime *libpod.Runtime, img *libimage.Image, rawImageName string) (bool, error) {
	localImg, _, err := runtime.LibimageRuntime().LookupImage(rawImageName, nil)
	if err != nil {
		return false, err
	}

	localDigest := localImg.Digest().String()

	ctrDigest := img.Digest().String()

	return localDigest != ctrDigest, nil
}

// updateImage pulls the specified image.
func updateImage(runtime *libpod.Runtime, name string, options Options) (*libimage.Image, error) {
	pullOptions := &libimage.PullOptions{}
	pullOptions.AuthFilePath = options.Authfile
	pullOptions.Writer = os.Stderr

	pulledImages, err := runtime.LibimageRuntime().Pull(context.Background(), name, config.PullPolicyAlways, pullOptions)
	if err != nil {
		return nil, err
	}
	return pulledImages[0], nil
}
