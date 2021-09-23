// +build amd64,!windows arm64,!windows

package machine

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

/*
	If this file gets too nuts, we can perhaps use existing go code
	to create ignition files.  At this point, the file is so simple
	that I chose to use structs and not import any code as I was
	concerned (unsubstantiated) about too much bloat coming in.

	https://github.com/openshift/machine-config-operator/blob/master/pkg/server/server.go
*/

// Convenience function to convert int to ptr
func intToPtr(i int) *int {
	return &i
}

// Convenience function to convert string to ptr
func strToPtr(s string) *string {
	return &s
}

// Convenience function to convert bool to ptr
func boolToPtr(b bool) *bool {
	return &b
}

func getNodeUsr(usrName string) NodeUser {
	return NodeUser{Name: &usrName}
}

func getNodeGrp(grpName string) NodeGroup {
	return NodeGroup{Name: &grpName}
}

type DynamicIgnition struct {
	Name      string
	Key       string
	VMName    string
	WritePath string
}

// NewIgnitionFile
func NewIgnitionFile(ign DynamicIgnition) error {
	if len(ign.Name) < 1 {
		ign.Name = DefaultIgnitionUserName
	}
	ignVersion := Ignition{
		Version: "3.2.0",
	}

	ignPassword := Passwd{
		Users: []PasswdUser{
			{
				Name:              ign.Name,
				SSHAuthorizedKeys: []SSHAuthorizedKey{SSHAuthorizedKey(ign.Key)},
			},
			{
				Name:              "root",
				SSHAuthorizedKeys: []SSHAuthorizedKey{SSHAuthorizedKey(ign.Key)},
			},
		},
	}

	ignStorage := Storage{
		Directories: getDirs(ign.Name),
		Files:       getFiles(ign.Name),
		Links:       getLinks(ign.Name),
	}

	// ready is a unit file that sets up the virtual serial device
	// where when the VM is done configuring, it will send an ack
	// so a listening host knows it can being interacting with it
	ready := `[Unit]
Requires=dev-virtio\\x2dports-%s.device
After=replace-moby.service
OnFailure=emergency.target
OnFailureJobMode=isolate
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/bin/sh -c '/usr/bin/echo Ready >/dev/%s'
[Install]
RequiredBy=multi-user.target
`
	deMoby := `[Unit]
Description=Remove moby-engine, alias podman
# Run once for the machine
After=systemd-machine-id-commit.service
Before=zincati.service
ConditionPathExists=!/var/lib/%N.stamp

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/bin/rpm-ostree override remove moby-engine
ExecStart=/bin/ln -fs /usr/bin/podman /usr/local/bin/docker
ExecStart=/bin/ln -fs /run/podman/podman.sock /run/docker.sock
ExecStart=/bin/touch /var/lib/%N.stamp
ExecStart=/usr/bin/rpm-ostree ex apply-live --allow-replacement

[Install]
WantedBy=multi-user.target
 `
	_ = ready
	ignSystemd := Systemd{
		Units: []Unit{
			{
				Enabled: boolToPtr(true),
				Name:    "podman.socket",
			},
			{
				Enabled:  boolToPtr(true),
				Name:     "ready.service",
				Contents: strToPtr(fmt.Sprintf(ready, "vport1p1", "vport1p1")),
			},
			{
				Name: "docker.service",
				Mask: boolToPtr(true),
			},
			{
				Enabled:  boolToPtr(true),
				Name:     "replace-moby.service",
				Contents: &deMoby,
			},
		}}
	ignConfig := Config{
		Ignition: ignVersion,
		Passwd:   ignPassword,
		Storage:  ignStorage,
		Systemd:  ignSystemd,
	}
	b, err := json.Marshal(ignConfig)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(ign.WritePath, b, 0644)
}

func getDirs(usrName string) []Directory {
	// Ignition has a bug/feature? where if you make a series of dirs
	// in one swoop, then the leading dirs are creates as root.
	newDirs := []string{
		"/home/" + usrName + "/.config",
		"/home/" + usrName + "/.config/containers",
		"/home/" + usrName + "/.config/systemd",
		"/home/" + usrName + "/.config/systemd/user",
		"/home/" + usrName + "/.config/systemd/user/default.target.wants",
	}
	var (
		dirs = make([]Directory, len(newDirs))
	)
	for i, d := range newDirs {
		newDir := Directory{
			Node: Node{
				Group: getNodeGrp(usrName),
				Path:  d,
				User:  getNodeUsr(usrName),
			},
			DirectoryEmbedded1: DirectoryEmbedded1{Mode: intToPtr(0755)},
		}
		dirs[i] = newDir
	}

	// Issue #11489: make sure that we can inject a custom registries.conf
	// file on the system level to force a single search registry.
	// The remote client does not yet support prompting for short-name
	// resolution, so we enforce a single search registry (i.e., docker.io)
	// as a workaround.
	dirs = append(dirs, Directory{
		Node: Node{
			Group: getNodeGrp("root"),
			Path:  "/etc/containers/registries.conf.d",
			User:  getNodeUsr("root"),
		},
		DirectoryEmbedded1: DirectoryEmbedded1{Mode: intToPtr(0755)},
	})

	return dirs
}

func getFiles(usrName string) []File {
	var (
		files []File
	)
	// Add a fake systemd service to get the user socket rolling
	files = append(files, File{
		Node: Node{
			Group: getNodeGrp(usrName),
			Path:  "/home/" + usrName + "/.config/systemd/user/linger-example.service",
			User:  getNodeUsr(usrName),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: strToPtr("data:,%5BUnit%5D%0ADescription%3DA%20systemd%20user%20unit%20demo%0AAfter%3Dnetwork-online.target%0AWants%3Dnetwork-online.target%20podman.socket%0A%5BService%5D%0AExecStart%3D%2Fusr%2Fbin%2Fsleep%20infinity%0A"),
			},
			Mode: intToPtr(0744),
		},
	})

	// Set containers.conf up for core user to use cni networks
	// by default
	files = append(files, File{
		Node: Node{
			Group: getNodeGrp(usrName),
			Path:  "/home/" + usrName + "/.config/containers/containers.conf",
			User:  getNodeUsr(usrName),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: strToPtr("data:,%5Bcontainers%5D%0D%0Anetns%3D%22bridge%22%0D%0Arootless_networking%3D%22cni%22"),
			},
			Mode: intToPtr(0744),
		},
	})
	// Add a file into linger
	files = append(files, File{
		Node: Node{
			Group: getNodeGrp(usrName),
			Path:  "/var/lib/systemd/linger/core",
			User:  getNodeUsr(usrName),
		},
		FileEmbedded1: FileEmbedded1{Mode: intToPtr(0644)},
	})

	// Set machine_enabled to true to indicate we're in a VM
	files = append(files, File{
		Node: Node{
			Group: getNodeGrp("root"),
			Path:  "/etc/containers/containers.conf",
			User:  getNodeUsr("root"),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: strToPtr("data:,%5Bengine%5D%0Amachine_enabled%3Dtrue%0A"),
			},
			Mode: intToPtr(0644),
		},
	})

	// Issue #11489: make sure that we can inject a custom registries.conf
	// file on the system level to force a single search registry.
	// The remote client does not yet support prompting for short-name
	// resolution, so we enforce a single search registry (i.e., docker.io)
	// as a workaround.
	files = append(files, File{
		Node: Node{
			Group: getNodeGrp("root"),
			Path:  "/etc/containers/registries.conf.d/999-podman-machine.conf",
			User:  getNodeUsr("root"),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: strToPtr("data:,unqualified-search-registries%3D%5B%22docker.io%22%5D"),
			},
			Mode: intToPtr(0644),
		},
	})

	return files
}

func getLinks(usrName string) []Link {
	return []Link{{
		Node: Node{
			Group: getNodeGrp(usrName),
			Path:  "/home/" + usrName + "/.config/systemd/user/default.target.wants/linger-example.service",
			User:  getNodeUsr(usrName),
		},
		LinkEmbedded1: LinkEmbedded1{
			Hard:   boolToPtr(false),
			Target: "/home/" + usrName + "/.config/systemd/user/linger-example.service",
		},
	}}
}
