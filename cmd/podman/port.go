package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	portCommand     cliconfig.PortValues
	portDescription = `List port mappings for the CONTAINER, or lookup the public-facing port that is NAT-ed to the PRIVATE_PORT
`
	_portCommand = &cobra.Command{
		Use:   "port [flags] CONTAINER",
		Short: "List port mappings or a specific mapping for the container",
		Long:  portDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			portCommand.InputArgs = args
			portCommand.GlobalFlags = MainGlobalOpts
			portCommand.Remote = remoteclient
			return portCmd(&portCommand)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			return checkAllAndLatest(cmd, args, true)
		},
		Example: `podman port --all
  podman port ctrID 80/tcp
  podman port --latest 80`,
	}
)

func init() {
	portCommand.Command = _portCommand
	portCommand.SetHelpTemplate(HelpTemplate())
	portCommand.SetUsageTemplate(UsageTemplate())
	flags := portCommand.Flags()

	flags.BoolVarP(&portCommand.All, "all", "a", false, "Display port information for all containers")
	flags.BoolVarP(&portCommand.Latest, "latest", "l", false, "Act on the latest container podman is aware of")

	markFlagHiddenForRemoteClient("latest", flags)
}

func portCmd(c *cliconfig.PortValues) error {
	var (
		userProto, containerName string
		userPort                 int
	)
	args := c.InputArgs

	if c.Latest && c.All {
		return errors.Errorf("the 'all' and 'latest' options cannot be used together")
	}
	if c.All && len(args) > 0 {
		return errors.Errorf("no additional arguments can be used with 'all'")
	}
	if len(args) == 0 && !c.Latest && !c.All {
		return errors.Errorf("you must supply a running container name or id")
	}

	port := ""
	if len(args) > 1 && !c.Latest {
		port = args[1]
	}
	if len(args) == 1 && c.Latest {
		port = args[0]
	}
	if port != "" {
		fields := strings.Split(port, "/")
		// User supplied at least port
		var err error
		// User supplied port and protocol
		if len(fields) == 2 {
			userProto = fields[1]
		}
		if len(fields) >= 1 {
			p := fields[0]
			userPort, err = strconv.Atoi(p)
			if err != nil {
				return errors.Wrapf(err, "unable to format port")
			}
		}
		// Format is incorrect
		if len(fields) > 2 || len(fields) < 1 {
			return errors.Errorf("port formats are port/protocol. '%s' is invalid", port)
		}
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.DeferredShutdown(false)

	containers, err := runtime.Port(c)
	if err != nil {
		return err
	}
	for _, con := range containers {
		portmappings, err := con.PortMappings()
		if err != nil {
			return err
		}
		// Iterate mappings
		for _, v := range portmappings {
			hostIP := v.HostIP
			// Set host IP to 0.0.0.0 if blank
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			if c.All {
				fmt.Printf("%s\t", con.ID()[:12])
			}
			// If not searching by port or port/proto, then dump what we see
			if port == "" {
				fmt.Printf("%d/%s -> %s:%d\n", v.ContainerPort, v.Protocol, hostIP, v.HostPort)
				continue
			}
			// We have a match on ports
			if v.ContainerPort == int32(userPort) {
				if userProto == "" || userProto == v.Protocol {
					fmt.Printf("%s:%d\n", hostIP, v.HostPort)
					break
				}
			} else {
				return errors.Errorf("No public port '%d' published for %s", userPort, containerName)
			}
		}
	}

	return nil
}
