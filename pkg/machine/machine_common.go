//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

// getDevNullFiles returns pointers to Read-only and Write-only DevNull files
func GetDevNullFiles() (*os.File, *os.File, error) {
	dnr, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0755)
	if err != nil {
		return nil, nil, err
	}

	dnw, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0755)
	if err != nil {
		if e := dnr.Close(); e != nil {
			err = e
		}
		return nil, nil, err
	}

	return dnr, dnw, nil
}

// AddSSHConnectionsToPodmanSocket adds SSH connections to the podman socket if
// no ignition path is provided
func AddSSHConnectionsToPodmanSocket(uid, port int, identityPath, name, remoteUsername string, opts InitOptions) error {
	if len(opts.IgnitionPath) < 1 {
		uri := SSHRemoteConnection.MakeSSHURL(LocalhostIP, fmt.Sprintf("/run/user/%d/podman/podman.sock", uid), strconv.Itoa(port), remoteUsername)
		uriRoot := SSHRemoteConnection.MakeSSHURL(LocalhostIP, "/run/podman/podman.sock", strconv.Itoa(port), "root")

		uris := []url.URL{uri, uriRoot}
		names := []string{name, name + "-root"}

		// The first connection defined when connections is empty will become the default
		// regardless of IsDefault, so order according to rootful
		if opts.Rootful {
			uris[0], names[0], uris[1], names[1] = uris[1], names[1], uris[0], names[0]
		}

		for i := 0; i < 2; i++ {
			if err := AddConnection(&uris[i], names[i], identityPath, opts.IsDefault && i == 0); err != nil {
				return err
			}
		}
	} else {
		fmt.Println("An ignition path was provided.  No SSH connection was added to Podman")
	}
	return nil
}
