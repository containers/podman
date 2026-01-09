//go:build amd64 || arm64

package os

import (
	"os"
	"os/exec"
)

// OSTree deals with operations on ostree based os's
type OSTree struct{}

// Apply takes an input that describes an image based on transports
// defined by bootc.  Omission of a transport assumes the image
// is pulled from an OCI registry.  We simply pass the user
// input to bootc without any manipulation.
func (dist *OSTree) Apply(image string, _ ApplyOptions) error {
	cli := []string{"bootc", "switch", image}
	cmd := exec.Command("sudo", cli...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
