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

// WaitAPIAndPrintInfo prints info about the machine and does a ping test on the
// API socket
func WaitAPIAndPrintInfo(forwardState APIForwardingState, name, helper, forwardSock string, noInfo, isIncompatible, rootful bool) {
	suffix := ""
	if name != DefaultMachineName {
		suffix = " " + name
	}

	if isIncompatible {
		fmt.Fprintf(os.Stderr, "\n!!! ACTION REQUIRED: INCOMPATIBLE MACHINE !!!\n")

		fmt.Fprintf(os.Stderr, "\nThis machine was created by an older podman release that is incompatible\n")
		fmt.Fprintf(os.Stderr, "with this release of podman. It has been started in a limited operational\n")
		fmt.Fprintf(os.Stderr, "mode to allow you to copy any necessary files before recreating it. This\n")
		fmt.Fprintf(os.Stderr, "can be accomplished with the following commands:\n\n")
		fmt.Fprintf(os.Stderr, "\t# Login and copy desired files (Optional)\n")
		fmt.Fprintf(os.Stderr, "\t# podman machine ssh%s tar cvPf - /path/to/files > backup.tar\n\n", suffix)
		fmt.Fprintf(os.Stderr, "\t# Recreate machine (DESTRUCTIVE!) \n")
		fmt.Fprintf(os.Stderr, "\tpodman machine stop%s\n", suffix)
		fmt.Fprintf(os.Stderr, "\tpodman machine rm -f%s\n", suffix)
		fmt.Fprintf(os.Stderr, "\tpodman machine init --now%s\n\n", suffix)
		fmt.Fprintf(os.Stderr, "\t# Copy back files (Optional)\n")
		fmt.Fprintf(os.Stderr, "\t# cat backup.tar | podman machine ssh%s tar xvPf - \n\n", suffix)
	}

	if forwardState == NoForwarding {
		return
	}

	WaitAndPingAPI(forwardSock)

	if !noInfo {
		if !rootful {
			fmt.Printf("\nThis machine is currently configured in rootless mode. If your containers\n")
			fmt.Printf("require root permissions (e.g. ports < 1024), or if you run into compatibility\n")
			fmt.Printf("issues with non-podman clients, you can switch using the following command: \n")
			fmt.Printf("\n\tpodman machine set --rootful%s\n\n", suffix)
		}

		fmt.Printf("API forwarding listening on: %s\n", forwardSock)
		if forwardState == DockerGlobal {
			fmt.Printf("Docker API clients default to this address. You do not need to set DOCKER_HOST.\n\n")
		} else {
			stillString := "still "
			switch forwardState {
			case NotInstalled:
				fmt.Printf("\nThe system helper service is not installed; the default Docker API socket\n")
				fmt.Printf("address can't be used by podman. ")
				if len(helper) > 0 {
					fmt.Printf("If you would like to install it run the\nfollowing commands:\n")
					fmt.Printf("\n\tsudo %s install\n", helper)
					fmt.Printf("\tpodman machine stop%s; podman machine start%s\n\n", suffix, suffix)
				}
			case MachineLocal:
				fmt.Printf("\nAnother process was listening on the default Docker API socket address.\n")
			case ClaimUnsupported:
				fallthrough
			default:
				stillString = ""
			}

			fmt.Printf("You can %sconnect Docker API clients by setting DOCKER_HOST using the\n", stillString)
			fmt.Printf("following command in your terminal session:\n")
			fmt.Printf("\n\texport DOCKER_HOST='unix://%s'\n\n", forwardSock)
		}
	}
}
