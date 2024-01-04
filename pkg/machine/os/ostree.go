//go:build amd64 || arm64

package os

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/containers/image/v5/transports/alltransports"
	"github.com/sirupsen/logrus"
)

// OSTree deals with operations on ostree based os's
type OSTree struct { //nolint:revive
}

// Apply takes an OCI image and does an rpm-ostree rebase on the image
// If no containers-transport is specified,
// apply will first check if the image exists locally, then default to pulling.
// Exec-ing out to rpm-ostree rebase requires sudo, so this means apply cannot
// be called within podman's user namespace if run as rootless.
// This means that we need to export images in containers-storage to oci-dirs
// We also need to do this via an exec, because if we tried to use the ABI functions,
// we would enter the user namespace, the rebase command would fail.
// The pull portion of this function essentially is a work-around for two things:
// 1. rpm-ostree requires you to specify the containers-transport when pulling.
// The pull in podman allows the behavior of os apply to match other podman commands,
// where you only pull if the image does not exist in storage already.
// 2. This works around the root/rootless issue.
// Podman machines are by default set up using a rootless connection.
// rpm-ostree needs to be run as root. If a user wants to use an image in containers-storage,
// rpm-ostree will look at the root storage, and not the user storage, which is unexpected behavior.
// Exporting to an oci-dir works around this, without nagging the user to configure the machine in rootful mode.
func (dist *OSTree) Apply(image string, opts ApplyOptions) error {
	imageWithTransport := image

	transport := alltransports.TransportFromImageName(image)

	switch {
	// no transport was specified
	case transport == nil:
		exists, err := execPodmanImageExists(image)
		if err != nil {
			return err
		}

		if exists {
			fmt.Println("Pulling from", "containers-storage"+":", imageWithTransport)
			dir, err := os.MkdirTemp("", pathSafeString(imageWithTransport))
			if err != nil {
				return err
			}
			if err := os.Chmod(dir, 0755); err != nil {
				return err
			}

			defer func() {
				if err := os.RemoveAll(dir); err != nil {
					logrus.Errorf("failed to remove temporary pull file: %v", err)
				}
			}()

			if err := execPodmanSave(dir, image); err != nil {
				return err
			}

			imageWithTransport = "oci:" + dir
		} else {
			// if image doesn't exist locally, assume that we want to pull and use docker transport
			imageWithTransport = "docker://" + image
		}
	// containers-transport specified
	case transport.Name() == "containers-storage":
		fmt.Println("Pulling from", image)
		dir, err := os.MkdirTemp("", pathSafeString(strings.TrimPrefix(image, "containers-storage"+":")))
		if err != nil {
			return err
		}

		if err := os.Chmod(dir, 0755); err != nil {
			return err
		}

		defer func() {
			if err := os.RemoveAll(dir); err != nil {
				logrus.Errorf("failed to remove temporary pull file: %v", err)
			}
		}()

		if err := execPodmanSave(dir, image); err != nil {
			return err
		}
		imageWithTransport = "oci:" + dir
	}

	ostreeCli := []string{"rpm-ostree", "--bypass-driver", "rebase", fmt.Sprintf("ostree-unverified-image:%s", imageWithTransport)}
	cmd := exec.Command("sudo", ostreeCli...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// pathSafeString creates a path-safe name for our tmpdirs
func pathSafeString(str string) string {
	alphanumOnly := regexp.MustCompile(`[^a-zA-Z0-9]+`)

	return alphanumOnly.ReplaceAllString(str, "")
}

// execPodmanSave execs out to podman save
func execPodmanSave(dir, image string) error {
	saveArgs := []string{"image", "save", "--format", "oci-dir", "-o", dir, image}

	saveCmd := exec.Command("podman", saveArgs...)
	saveCmd.Stdout = os.Stdout
	saveCmd.Stderr = os.Stderr
	return saveCmd.Run()
}

// execPodmanSave execs out to podman image exists
func execPodmanImageExists(image string) (bool, error) {
	existsArgs := []string{"image", "exists", image}

	existsCmd := exec.Command("podman", existsArgs...)

	if err := existsCmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			switch exitCode := exitError.ExitCode(); exitCode {
			case 1:
				return false, nil
			default:
				return false, errors.New("unable to access local image store")
			}
		}
	}
	return true, nil
}
