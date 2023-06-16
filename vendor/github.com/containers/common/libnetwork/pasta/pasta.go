// SPDX-License-Identifier: Apache-2.0
//
// pasta.go - Start pasta(1) for user-mode connectivity
//
// Copyright (c) 2022 Red Hat GmbH
// Author: Stefano Brivio <sbrivio@redhat.com>

// This file has been imported from the podman repository
// (libpod/networking_pasta_linux.go), for the full history see there.

package pasta

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/config"
	"github.com/sirupsen/logrus"
)

const (
	BinaryName = "pasta"
)

type SetupOptions struct {
	// Config used to get pasta options and binary path via HelperBinariesDir
	Config *config.Config
	// Netns is the path to the container Netns
	Netns string
	// Ports that should be forwarded in the container
	Ports []types.PortMapping
	// ExtraOptions are pasta(1) cli options, these will be appended after the
	// pasta options from containers.conf to allow some form of overwrite.
	ExtraOptions []string
}

// Setup start the pasta process for the given netns.
// The pasta binary is looked up in the HelperBinariesDir and $PATH.
// Note that there is no need any special cleanup logic, the pasta process will
// automatically exit when the netns path is deleted.
func Setup(opts *SetupOptions) error {
	NoTCPInitPorts := true
	NoUDPInitPorts := true
	NoTCPNamespacePorts := true
	NoUDPNamespacePorts := true
	NoMapGW := true

	path, err := opts.Config.FindHelperBinary(BinaryName, true)
	if err != nil {
		return fmt.Errorf("could not find pasta, the network namespace can't be configured: %w", err)
	}

	cmdArgs := []string{}
	cmdArgs = append(cmdArgs, "--config-net")

	for _, i := range opts.Ports {
		protocols := strings.Split(i.Protocol, ",")
		for _, protocol := range protocols {
			var addr string

			if i.HostIP != "" {
				addr = fmt.Sprintf("%s/", i.HostIP)
			}

			switch protocol {
			case "tcp":
				cmdArgs = append(cmdArgs, "-t")
			case "udp":
				cmdArgs = append(cmdArgs, "-u")
			default:
				return fmt.Errorf("can't forward protocol: %s", protocol)
			}

			arg := fmt.Sprintf("%s%d-%d:%d-%d", addr,
				i.HostPort,
				i.HostPort+i.Range-1,
				i.ContainerPort,
				i.ContainerPort+i.Range-1)
			cmdArgs = append(cmdArgs, arg)
		}
	}

	// first append options set in the config
	cmdArgs = append(cmdArgs, opts.Config.Network.PastaOptions...)
	// then append the ones that were set on the cli
	cmdArgs = append(cmdArgs, opts.ExtraOptions...)

	for i, opt := range cmdArgs {
		switch opt {
		case "-t", "--tcp-ports":
			NoTCPInitPorts = false
		case "-u", "--udp-ports":
			NoUDPInitPorts = false
		case "-T", "--tcp-ns":
			NoTCPNamespacePorts = false
		case "-U", "--udp-ns":
			NoUDPNamespacePorts = false
		case "--map-gw":
			NoMapGW = false
			// not an actual pasta(1) option
			cmdArgs = append(cmdArgs[:i], cmdArgs[i+1:]...)
		}
	}

	if NoTCPInitPorts {
		cmdArgs = append(cmdArgs, "-t", "none")
	}
	if NoUDPInitPorts {
		cmdArgs = append(cmdArgs, "-u", "none")
	}
	if NoTCPNamespacePorts {
		cmdArgs = append(cmdArgs, "-T", "none")
	}
	if NoUDPNamespacePorts {
		cmdArgs = append(cmdArgs, "-U", "none")
	}
	if NoMapGW {
		cmdArgs = append(cmdArgs, "--no-map-gw")
	}

	cmdArgs = append(cmdArgs, "--netns", opts.Netns)

	logrus.Debugf("pasta arguments: %s", strings.Join(cmdArgs, " "))

	// pasta forks once ready, and quits once we delete the target namespace
	_, err = exec.Command(path, cmdArgs...).Output()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return fmt.Errorf("pasta failed with exit code %d:\n%s",
				exitErr.ExitCode(), exitErr.Stderr)
		}
		return fmt.Errorf("failed to start pasta: %w", err)
	}

	return nil
}
