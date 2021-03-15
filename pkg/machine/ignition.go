package machine

import (
	"encoding/json"
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

// NewIgnitionFile
func NewIgnitionFile(name, key, writePath string) error {
	if len(name) < 1 {
		name = DefaultIgnitionUserName
	}
	ignVersion := Ignition{
		Version: "3.2.0",
	}

	ignPassword := Passwd{
		Users: []PasswdUser{{
			Name:              name,
			SSHAuthorizedKeys: []SSHAuthorizedKey{SSHAuthorizedKey(key)},
		}},
	}

	ignStorage := Storage{
		Directories: getDirs(name),
		Files:       getFiles(name),
		Links:       getLinks(name),
	}
	ignSystemd := Systemd{
		Units: []Unit{
			{
				Enabled: boolToPtr(true),
				Name:    "podman.socket",
			}}}

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
	return ioutil.WriteFile(writePath, b, 0644)
}

func getDirs(usrName string) []Directory {
	// Ignition has a bug/feature? where if you make a series of dirs
	// in one swoop, then the leading dirs are creates as root.
	newDirs := []string{
		"/home/" + usrName + "/.config",
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
			DirectoryEmbedded1: DirectoryEmbedded1{Mode: intToPtr(493)},
		}
		dirs[i] = newDir
	}
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
			Mode: intToPtr(484),
		},
	})

	// Add a file into linger
	files = append(files, File{
		Node: Node{
			Group: getNodeGrp(usrName),
			Path:  "/var/lib/systemd/linger/core",
			User:  getNodeUsr(usrName),
		},
		FileEmbedded1: FileEmbedded1{Mode: intToPtr(420)},
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
