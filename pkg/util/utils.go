package util

import (
	"fmt"
	"strings"

	"github.com/containers/image/types"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"
)

// Helper function to determine the username/password passed
// in the creds string.  It could be either or both.
func parseCreds(creds string) (string, string) {
	if creds == "" {
		return "", ""
	}
	up := strings.SplitN(creds, ":", 2)
	if len(up) == 1 {
		return up[0], ""
	}
	return up[0], up[1]
}

// ParseRegistryCreds takes a credentials string in the form USERNAME:PASSWORD
// and returns a DockerAuthConfig
func ParseRegistryCreds(creds string) (*types.DockerAuthConfig, error) {
	username, password := parseCreds(creds)
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scanln(&username)
	}
	if password == "" {
		fmt.Print("Password: ")
		termPassword, err := terminal.ReadPassword(0)
		if err != nil {
			return nil, errors.Wrapf(err, "could not read password from terminal")
		}
		password = string(termPassword)
	}

	return &types.DockerAuthConfig{
		Username: username,
		Password: password,
	}, nil
}

// StringInSlice determines if a string is in a string slice, returns bool
func StringInSlice(s string, sl []string) bool {
	for _, i := range sl {
		if i == s {
			return true
		}
	}
	return false
}

// GetImageConfig converts the --change flag values in the format "CMD=/bin/bash USER=example"
// to a type v1.ImageConfig
func GetImageConfig(changes []string) (v1.ImageConfig, error) {
	// USER=value | EXPOSE=value | ENV=value | ENTRYPOINT=value |
	// CMD=value | VOLUME=value | WORKDIR=value | LABEL=key=value | STOPSIGNAL=value

	var (
		user       string
		env        []string
		entrypoint []string
		cmd        []string
		workingDir string
		stopSignal string
	)

	exposedPorts := make(map[string]struct{})
	volumes := make(map[string]struct{})
	labels := make(map[string]string)

	for _, ch := range changes {
		pair := strings.Split(ch, "=")
		if len(pair) == 1 {
			return v1.ImageConfig{}, errors.Errorf("no value given for instruction %q", ch)
		}
		switch pair[0] {
		case "USER":
			user = pair[1]
		case "EXPOSE":
			var st struct{}
			exposedPorts[pair[1]] = st
		case "ENV":
			env = append(env, pair[1])
		case "ENTRYPOINT":
			entrypoint = append(entrypoint, pair[1])
		case "CMD":
			cmd = append(cmd, pair[1])
		case "VOLUME":
			var st struct{}
			volumes[pair[1]] = st
		case "WORKDIR":
			workingDir = pair[1]
		case "LABEL":
			if len(pair) == 3 {
				labels[pair[1]] = pair[2]
			} else {
				labels[pair[1]] = ""
			}
		case "STOPSIGNAL":
			stopSignal = pair[1]
		}
	}

	return v1.ImageConfig{
		User:         user,
		ExposedPorts: exposedPorts,
		Env:          env,
		Entrypoint:   entrypoint,
		Cmd:          cmd,
		Volumes:      volumes,
		WorkingDir:   workingDir,
		Labels:       labels,
		StopSignal:   stopSignal,
	}, nil
}
