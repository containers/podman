// SPDX-License-Identifier: Apache-2.0
//
// networking_pasta_linux.go - Start pasta(1) for user-mode connectivity
//
// Copyright (c) 2022 Red Hat GmbH
// Author: Stefano Brivio <sbrivio@redhat.com>

package libpod

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

func (r *Runtime) setupPasta(ctr *Container, netns string) error {
	var NoTCPInitPorts = true
	var NoUDPInitPorts = true
	var NoTCPNamespacePorts = true
	var NoUDPNamespacePorts = true
	var NoMapGW = true

	path, err := r.config.FindHelperBinary("pasta", true)
	if err != nil {
		return fmt.Errorf("could not find pasta, the network namespace can't be configured: %w", err)
	}

	cmdArgs := []string{}
	cmdArgs = append(cmdArgs, "--config-net")

	for _, i := range ctr.convertPortMappings() {
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
			case "default":
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

	cmdArgs = append(cmdArgs, ctr.config.NetworkOptions["pasta"]...)

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

	cmdArgs = append(cmdArgs, "--netns", netns)

	logrus.Debugf("pasta arguments: %s", strings.Join(cmdArgs, " "))

	// pasta forks once ready, and quits once we delete the target namespace
	_, err = exec.Command(path, cmdArgs...).Output()
	if err != nil {
		return fmt.Errorf("failed to start pasta:\n%s",
			err.(*exec.ExitError).Stderr)
	}

	return nil
}
