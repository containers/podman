package varlinkapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/google/shlex"
	"github.com/pkg/errors"
)

func GetSystemContext(authfile string) (*types.SystemContext, error) {
	if authfile != "" {
		if _, err := os.Stat(authfile); err != nil {
			return nil, errors.Wrapf(err, "error checking authfile path %s", authfile)
		}
	}
	return image.GetSystemContext("", authfile, false), nil
}

func substituteCommand(cmd string) (string, error) {
	var (
		newCommand string
	)

	// Replace cmd with "/proc/self/exe" if "podman" or "docker" is being
	// used. If "/usr/bin/docker" is provided, we also sub in podman.
	// Otherwise, leave the command unchanged.
	if cmd == "podman" || filepath.Base(cmd) == "docker" {
		newCommand = "/proc/self/exe"
	} else {
		newCommand = cmd
	}

	// If cmd is an absolute or relative path, check if the file exists.
	// Throw an error if it doesn't exist.
	if strings.Contains(newCommand, "/") || strings.HasPrefix(newCommand, ".") {
		res, err := filepath.Abs(newCommand)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(res); !os.IsNotExist(err) {
			return res, nil
		} else if err != nil {
			return "", err
		}
	}

	return newCommand, nil
}

// GenerateCommand takes a label (string) and converts it to an executable command
func GenerateCommand(command, imageName, name, globalOpts string) ([]string, error) {
	var (
		newCommand []string
	)
	if name == "" {
		name = imageName
	}

	cmd, err := shlex.Split(command)
	if err != nil {
		return nil, err
	}

	prog, err := substituteCommand(cmd[0])
	if err != nil {
		return nil, err
	}
	newCommand = append(newCommand, prog)

	for _, arg := range cmd[1:] {
		var newArg string
		switch arg {
		case "IMAGE":
			newArg = imageName
		case "$IMAGE":
			newArg = imageName
		case "IMAGE=IMAGE":
			newArg = fmt.Sprintf("IMAGE=%s", imageName)
		case "IMAGE=$IMAGE":
			newArg = fmt.Sprintf("IMAGE=%s", imageName)
		case "NAME":
			newArg = name
		case "NAME=NAME":
			newArg = fmt.Sprintf("NAME=%s", name)
		case "NAME=$NAME":
			newArg = fmt.Sprintf("NAME=%s", name)
		case "$NAME":
			newArg = name
		case "$GLOBAL_OPTS":
			newArg = globalOpts
		default:
			newArg = arg
		}
		newCommand = append(newCommand, newArg)
	}
	return newCommand, nil
}

// GenerateRunEnvironment merges the current environment variables with optional
// environment variables provided by the user
func GenerateRunEnvironment(name, imageName string, opts map[string]string) []string {
	newEnv := os.Environ()
	newEnv = append(newEnv, fmt.Sprintf("NAME=%s", name))
	newEnv = append(newEnv, fmt.Sprintf("IMAGE=%s", imageName))

	if opts["opt1"] != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT1=%s", opts["opt1"]))
	}
	if opts["opt2"] != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT2=%s", opts["opt2"]))
	}
	if opts["opt3"] != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT3=%s", opts["opt3"]))
	}
	return newEnv
}
