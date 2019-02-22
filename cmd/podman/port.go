package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	portCommand     cliconfig.PortValues
	portDescription = `
   podman port

	List port mappings for the CONTAINER, or lookup the public-facing port that is NAT-ed to the PRIVATE_PORT
`
	_portCommand = &cobra.Command{
		Use:   "port",
		Short: "List port mappings or a specific mapping for the container",
		Long:  portDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			portCommand.InputArgs = args
			portCommand.GlobalFlags = MainGlobalOpts
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
		container                *libpod.Container
		containers               []*libpod.Container
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
	if !c.Latest && !c.All {
		containerName = args[0]
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

	runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if !c.Latest && !c.All {
		container, err = runtime.LookupContainer(containerName)
		if err != nil {
			return errors.Wrapf(err, "unable to find container %s", containerName)
		}
		containers = append(containers, container)
	} else if c.Latest {
		container, err = runtime.GetLatestContainer()
		if err != nil {
			return errors.Wrapf(err, "unable to get last created container")
		}
		containers = append(containers, container)
	} else {
		containers, err = runtime.GetRunningContainers()
		if err != nil {
			return errors.Wrapf(err, "unable to get all containers")
		}
	}

	for _, con := range containers {
		if state, _ := con.State(); state != libpod.ContainerStateRunning {
			continue
		}
		if c.All {
			fmt.Println(con.ID())
		}

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
