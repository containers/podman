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
	"net"
	"os/exec"
	"slices"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/libnetwork/util"
	"github.com/containers/common/pkg/config"
	"github.com/sirupsen/logrus"
)

const (
	dnsForwardOpt = "--dns-forward"

	// dnsForwardIpv4 static ip used as nameserver address inside the netns,
	// given this is a "link local" ip it should be very unlikely that it causes conflicts
	dnsForwardIpv4 = "169.254.0.1"
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

func Setup(opts *SetupOptions) error {
	_, err := Setup2(opts)
	return err
}

// Setup2 start the pasta process for the given netns.
// The pasta binary is looked up in the HelperBinariesDir and $PATH.
// Note that there is no need for any special cleanup logic, the pasta
// process will automatically exit when the netns path is deleted.
func Setup2(opts *SetupOptions) (*SetupResult, error) {
	path, err := opts.Config.FindHelperBinary(BinaryName, true)
	if err != nil {
		return nil, fmt.Errorf("could not find pasta, the network namespace can't be configured: %w", err)
	}

	cmdArgs, dnsForwardIPs, err := createPastaArgs(opts)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("pasta arguments: %s", strings.Join(cmdArgs, " "))

	// pasta forks once ready, and quits once we delete the target namespace
	out, err := exec.Command(path, cmdArgs...).CombinedOutput()
	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("pasta failed with exit code %d:\n%s",
				exitErr.ExitCode(), string(out))
		}
		return nil, fmt.Errorf("failed to start pasta: %w", err)
	}

	if len(out) > 0 {
		// TODO: This should be warning but right now pasta still prints
		// things with --quiet that we do not care about.
		// For now info is fine and we can bump it up later, it is only a
		// nice to have.
		logrus.Infof("pasta logged warnings: %q", string(out))
	}

	var ipv4, ipv6 bool
	result := &SetupResult{}
	err = ns.WithNetNSPath(opts.Netns, func(_ ns.NetNS) error {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return err
		}
		for _, addr := range addrs {
			// make sure to skip localhost and other special addresses
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.IsGlobalUnicast() {
				result.IPAddresses = append(result.IPAddresses, ipnet.IP)
				if !ipv4 && util.IsIPv4(ipnet.IP) {
					ipv4 = true
				}
				if !ipv6 && util.IsIPv6(ipnet.IP) {
					ipv6 = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result.IPv6 = ipv6
	for _, ip := range dnsForwardIPs {
		ipp := net.ParseIP(ip)
		// add the namesever ip only if the address family matches
		if ipv4 && util.IsIPv4(ipp) || ipv6 && util.IsIPv6(ipp) {
			result.DNSForwardIPs = append(result.DNSForwardIPs, ip)
		}
	}

	return result, nil
}

// createPastaArgs creates the pasta arguments, it returns the args to be passed to pasta(1) and as second arg the dns forward ips used.
func createPastaArgs(opts *SetupOptions) ([]string, []string, error) {
	noTCPInitPorts := true
	noUDPInitPorts := true
	noTCPNamespacePorts := true
	noUDPNamespacePorts := true
	noMapGW := true

	cmdArgs := []string{"--config-net"}
	// first append options set in the config
	cmdArgs = append(cmdArgs, opts.Config.Network.PastaOptions.Get()...)
	// then append the ones that were set on the cli
	cmdArgs = append(cmdArgs, opts.ExtraOptions...)

	cmdArgs = slices.DeleteFunc(cmdArgs, func(s string) bool {
		// --map-gw is not a real pasta(1) option so we must remove it
		// and not add --no-map-gw below
		if s == "--map-gw" {
			noMapGW = false
			return true
		}
		return false
	})

	var dnsForwardIPs []string
	for i, opt := range cmdArgs {
		switch opt {
		case "-t", "--tcp-ports":
			noTCPInitPorts = false
		case "-u", "--udp-ports":
			noUDPInitPorts = false
		case "-T", "--tcp-ns":
			noTCPNamespacePorts = false
		case "-U", "--udp-ns":
			noUDPNamespacePorts = false
		case dnsForwardOpt:
			// if there is no arg after it pasta will likely error out anyway due invalid cli args
			if len(cmdArgs) > i+1 {
				dnsForwardIPs = append(dnsForwardIPs, cmdArgs[i+1])
			}
		}
	}

	for _, i := range opts.Ports {
		protocols := strings.Split(i.Protocol, ",")
		for _, protocol := range protocols {
			var addr string

			if i.HostIP != "" {
				addr = i.HostIP + "/"
			}

			switch protocol {
			case "tcp":
				noTCPInitPorts = false
				cmdArgs = append(cmdArgs, "-t")
			case "udp":
				noUDPInitPorts = false
				cmdArgs = append(cmdArgs, "-u")
			default:
				return nil, nil, fmt.Errorf("can't forward protocol: %s", protocol)
			}

			arg := fmt.Sprintf("%s%d-%d:%d-%d", addr,
				i.HostPort,
				i.HostPort+i.Range-1,
				i.ContainerPort,
				i.ContainerPort+i.Range-1)
			cmdArgs = append(cmdArgs, arg)
		}
	}

	if len(dnsForwardIPs) == 0 {
		// the user did not request custom --dns-forward so add our own.
		cmdArgs = append(cmdArgs, dnsForwardOpt, dnsForwardIpv4)
		dnsForwardIPs = append(dnsForwardIPs, dnsForwardIpv4)
	}

	if noTCPInitPorts {
		cmdArgs = append(cmdArgs, "-t", "none")
	}
	if noUDPInitPorts {
		cmdArgs = append(cmdArgs, "-u", "none")
	}
	if noTCPNamespacePorts {
		cmdArgs = append(cmdArgs, "-T", "none")
	}
	if noUDPNamespacePorts {
		cmdArgs = append(cmdArgs, "-U", "none")
	}
	if noMapGW {
		cmdArgs = append(cmdArgs, "--no-map-gw")
	}

	// always pass --quiet to silence the info output from pasta
	cmdArgs = append(cmdArgs, "--quiet", "--netns", opts.Netns)

	return cmdArgs, dnsForwardIPs, nil
}
