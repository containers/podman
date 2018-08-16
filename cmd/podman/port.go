package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	portFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "display port information for all containers",
		},
		LatestFlag,
	}
	portDescription = `
   podman port

	List port mappings for the CONTAINER, or lookup the public-facing port that is NAT-ed to the PRIVATE_PORT
`

	portCommand = cli.Command{
		Name:        "port",
		Usage:       "List port mappings or a specific mapping for the container",
		Description: portDescription,
		Flags:       portFlags,
		Action:      portCmd,
		ArgsUsage:   "CONTAINER-NAME [mapping]",
	}
)

func portCmd(c *cli.Context) error {
	var (
		userProto, containerName string
		userPort                 int
		container                *libpod.Container
		containers               []*libpod.Container
	)

	args := c.Args()
	if err := validateFlags(c, portFlags); err != nil {
		return err
	}

	if c.Bool("latest") && c.Bool("all") {
		return errors.Errorf("the 'all' and 'latest' options cannot be used together")
	}
	if c.Bool("all") && len(args) > 0 {
		return errors.Errorf("no additional arguments can be used with 'all'")
	}
	if len(args) == 0 && !c.Bool("latest") && !c.Bool("all") {
		return errors.Errorf("you must supply a running container name or id")
	}
	if !c.Bool("latest") && !c.Bool("all") {
		containerName = args[0]
	}

	port := ""
	if len(args) > 1 && !c.Bool("latest") {
		port = args[1]
	}
	if len(args) == 1 && c.Bool("latest") {
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

	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}
	defer runtime.Shutdown(false)

	if !c.Bool("latest") && !c.Bool("all") {
		container, err = runtime.LookupContainer(containerName)
		if err != nil {
			return errors.Wrapf(err, "unable to find container %s", containerName)
		}
		containers = append(containers, container)
	} else if c.Bool("latest") {
		container, err = runtime.GetLatestContainer()
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
		if c.Bool("all") {
			fmt.Println(con.ID())
		}
		// Iterate mappings
		for _, v := range con.Config().PortMappings {
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
