package connection

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/containers/podman/v4/pkg/machine/define"
)

// AddSSHConnectionsToPodmanSocket adds SSH connections to the podman socket if
// no ignition path is provided
func AddSSHConnectionsToPodmanSocket(uid, port int, identityPath, name, remoteUsername string, opts define.InitOptions) error {
	if len(opts.IgnitionPath) > 0 {
		fmt.Println("An ignition path was provided.  No SSH connection was added to Podman")
		return nil
	}
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
	return nil
}
