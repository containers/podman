//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/containers/storage/pkg/ioutils"
)

// GetDevNullFiles returns pointers to Read-only and Write-only DevNull files
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

// WaitAPIAndPrintInfo prints info about the machine and does a ping test on the
// API socket
func WaitAPIAndPrintInfo(forwardState APIForwardingState, name, helper, forwardSock string, noInfo, isIncompatible, rootful bool) {
	suffix := ""
	var fmtString string

	if name != DefaultMachineName {
		suffix = " " + name
	}

	if isIncompatible {
		fmtString = `
!!! ACTION REQUIRED: INCOMPATIBLE MACHINE !!!

This machine was created by an older podman release that is incompatible
with this release of podman. It has been started in a limited operational
mode to allow you to copy any necessary files before recreating it. This
can be accomplished with the following commands:

        # Login and copy desired files (Optional)
        # podman machine ssh%[1]s tar cvPf - /path/to/files > backup.tar

        # Recreate machine (DESTRUCTIVE!)
        podman machine stop%[1]s
        podman machine rm -f%[1]s
        podman machine init --now%[1]s

        # Copy back files (Optional)
        # cat backup.tar | podman machine ssh%[1]s tar xvPf -

`

		fmt.Fprintf(os.Stderr, fmtString, suffix)
	}

	if forwardState == NoForwarding {
		return
	}

	WaitAndPingAPI(forwardSock)

	if !noInfo {
		if !rootful {
			fmtString = `
This machine is currently configured in rootless mode. If your containers
require root permissions (e.g. ports < 1024), or if you run into compatibility
issues with non-podman clients, you can switch using the following command:

        podman machine set --rootful%s

`

			fmt.Printf(fmtString, suffix)
		}

		fmt.Printf("API forwarding listening on: %s\n", forwardSock)
		if forwardState == DockerGlobal {
			fmt.Printf("Docker API clients default to this address. You do not need to set DOCKER_HOST.\n\n")
		} else {
			stillString := "still "
			switch forwardState {
			case NotInstalled:

				fmtString = `
The system helper service is not installed; the default Docker API socket
address can't be used by podman. `

				if len(helper) < 1 {
					fmt.Print(fmtString)
				} else {
					fmtString += `If you would like to install it, run the following commands:

        sudo %s install
        podman machine stop %[1]s; podman machine start %[1]s

                `
					fmt.Printf(fmtString, helper, suffix)
				}
			case MachineLocal:
				fmt.Printf("\nAnother process was listening on the default Docker API socket address.\n")
			case ClaimUnsupported:
				fallthrough
			default:
				stillString = ""
			}

			fmtString = `You can %sconnect Docker API clients by setting DOCKER_HOST using the
following command in your terminal session:

        export DOCKER_HOST='unix://%s'

`

			fmt.Printf(fmtString, stillString, forwardSock)
		}
	}
}

// SetRootful modifies the machine's default connection to be either rootful or
// rootless
func SetRootful(rootful bool, name, rootfulName string) error {
	changeCon, err := AnyConnectionDefault(name, rootfulName)
	if err != nil {
		return err
	}

	if changeCon {
		newDefault := name
		if rootful {
			newDefault += "-root"
		}
		err := ChangeDefault(newDefault)
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteConfig writes the machine's JSON config file
func WriteConfig(configPath string, v VM) error {
	opts := &ioutils.AtomicFileWriterOptions{ExplicitCommit: true}
	w, err := ioutils.NewAtomicFileWriterWithOpts(configPath, 0644, opts)
	if err != nil {
		return err
	}
	defer w.Close()

	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")

	if err := enc.Encode(v); err != nil {
		return err
	}

	// Commit the changes to disk if no errors
	return w.Commit()
}
