package abi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/domain/entities"
	envLib "github.com/containers/podman/v2/pkg/env"
	"github.com/containers/podman/v2/utils"
	"github.com/google/shlex"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (ic *ContainerEngine) ContainerRunlabel(ctx context.Context, label string, imageRef string, args []string, options entities.ContainerRunlabelOptions) error {
	// First, get the image and pull it if needed.
	img, err := ic.runlabelImage(ctx, label, imageRef, options)
	if err != nil {
		return err
	}
	// Extract the runlabel from the image.
	runlabel, err := img.GetLabel(ctx, label)
	if err != nil {
		return err
	}
	if runlabel == "" {
		return errors.Errorf("cannot find the value of label: %s in image: %s", label, imageRef)
	}

	cmd, env, err := generateRunlabelCommand(runlabel, img, args, options)
	if err != nil {
		return err
	}

	if options.Display {
		fmt.Printf("command: %s\n", strings.Join(append([]string{os.Args[0]}, cmd[1:]...), " "))
		return nil
	}

	stdErr := os.Stderr
	stdOut := os.Stdout
	stdIn := os.Stdin
	if options.Quiet {
		stdErr = nil
		stdOut = nil
		stdIn = nil
	}

	// If container already exists && --replace given -- Nuke it
	if options.Replace {
		for i, entry := range cmd {
			if entry == "--name" {
				name := cmd[i+1]
				ctr, err := ic.Libpod.LookupContainer(name)
				if err != nil {
					if errors.Cause(err) != define.ErrNoSuchCtr {
						logrus.Debugf("Error occurred searching for container %s: %s", name, err.Error())
						return err
					}
				} else {
					logrus.Debugf("Runlabel --replace option given. Container %s will be deleted. The new container will be named %s", ctr.ID(), name)
					if err := ic.Libpod.RemoveContainer(ctx, ctr, true, false); err != nil {
						return err
					}
				}
				break
			}
		}
	}

	return utils.ExecCmdWithStdStreams(stdIn, stdOut, stdErr, env, cmd[0], cmd[1:]...)
}

// runlabelImage returns an image based on the specified image AND options.
func (ic *ContainerEngine) runlabelImage(ctx context.Context, label string, imageRef string, options entities.ContainerRunlabelOptions) (*image.Image, error) {
	// First, look up the image locally. If we get an error and requested
	// to pull, fallthrough and pull it.
	img, err := ic.Libpod.ImageRuntime().NewFromLocal(imageRef)
	switch {
	case err == nil:
		return img, nil
	case !options.Pull:
		return nil, err
	default:
		// Fallthrough and pull!
	}

	pullOptions := entities.ImagePullOptions{
		Quiet:           options.Quiet,
		CertDir:         options.CertDir,
		SkipTLSVerify:   options.SkipTLSVerify,
		SignaturePolicy: options.SignaturePolicy,
		Authfile:        options.Authfile,
	}
	if _, err := pull(ctx, ic.Libpod.ImageRuntime(), imageRef, pullOptions, &label); err != nil {
		return nil, err
	}
	return ic.Libpod.ImageRuntime().NewFromLocal(imageRef)
}

// generateRunlabelCommand generates the to-be-executed command as a string
// slice along with a base environment.
func generateRunlabelCommand(runlabel string, img *image.Image, args []string, options entities.ContainerRunlabelOptions) ([]string, []string, error) {
	var (
		err             error
		name, imageName string
		globalOpts      string
		cmd             []string
	)

	// TODO: How do we get global opts as done in v1?

	// Extract the imageName (or ID).
	imgNames := img.Names()
	if len(imgNames) == 0 {
		imageName = img.ID()
	} else {
		imageName = imgNames[0]
	}

	// Use the user-specified name or extract one from the image.
	if options.Name != "" {
		name = options.Name
	} else {
		name, err = image.GetImageBaseName(imageName)
		if err != nil {
			return nil, nil, err
		}
	}

	// Append the user-specified arguments to the runlabel (command).
	if len(args) > 0 {
		runlabel = fmt.Sprintf("%s %s", runlabel, strings.Join(args, " "))
	}

	cmd, err = generateCommand(runlabel, imageName, name, globalOpts)
	if err != nil {
		return nil, nil, err
	}

	env := generateRunEnvironment(options)
	env = append(env, "PODMAN_RUNLABEL_NESTED=1")
	envmap, err := envLib.ParseSlice(env)
	if err != nil {
		return nil, nil, err
	}

	envmapper := func(k string) string {
		switch k {
		case "OPT1":
			return envmap["OPT1"]
		case "OPT2":
			return envmap["OPT2"]
		case "OPT3":
			return envmap["OPT3"]
		case "PWD":
			// I would prefer to use os.getenv but it appears PWD is not in the os env list.
			d, err := os.Getwd()
			if err != nil {
				logrus.Error("unable to determine current working directory")
				return ""
			}
			return d
		}
		return ""
	}
	newS := os.Expand(strings.Join(cmd, " "), envmapper)
	cmd, err = shlex.Split(newS)
	if err != nil {
		return nil, nil, err
	}
	return cmd, env, nil
}

// generateCommand takes a label (string) and converts it to an executable command
func generateCommand(command, imageName, name, globalOpts string) ([]string, error) {
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
	newCommand := []string{prog}
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
func generateRunEnvironment(options entities.ContainerRunlabelOptions) []string {
	newEnv := os.Environ()
	if options.Optional1 != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT1=%s", options.Optional1))
	}
	if options.Optional2 != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT2=%s", options.Optional2))
	}
	if options.Optional3 != "" {
		newEnv = append(newEnv, fmt.Sprintf("OPT3=%s", options.Optional3))
	}
	return newEnv
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
