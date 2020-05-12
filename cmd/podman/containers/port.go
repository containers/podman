package containers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/containers/libpod/cmd/podman/parse"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	portDescription = `List port mappings for the CONTAINER, or lookup the public-facing port that is NAT-ed to the PRIVATE_PORT
`
	portCommand = &cobra.Command{
		Use:   "port [flags] CONTAINER [PORT]",
		Short: "List port mappings or a specific mapping for the container",
		Long:  portDescription,
		RunE:  port,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, true, false)
		},
		Example: `podman port --all
  podman port ctrID 80/tcp
  podman port --latest 80`,
	}

	containerPortCommand = &cobra.Command{
		Use:   "port [flags] CONTAINER [PORT]",
		Short: portCommand.Short,
		Long:  portDescription,
		RunE:  portCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return parse.CheckAllLatestAndCIDFile(cmd, args, true, false)
		},
		Example: `podman container port --all
  podman container port --latest 80`,
	}
)

var (
	portOpts entities.ContainerPortOptions
)

func portFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&portOpts.All, "all", "a", false, "Display port information for all containers")
	flags.BoolVarP(&portOpts.Latest, "latest", "l", false, "Act on the latest container podman is aware of")
	if registry.IsRemote() {
		_ = flags.MarkHidden("latest")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: portCommand,
	})

	flags := portCommand.Flags()
	portFlags(flags)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: containerPortCommand,
		Parent:  containerCmd,
	})

	containerPortflags := containerPortCommand.Flags()
	portFlags(containerPortflags)

}

func port(cmd *cobra.Command, args []string) error {
	var (
		container string
		err       error
		userPort  ocicni.PortMapping
	)

	if len(args) == 0 && !portOpts.Latest && !portOpts.All {
		return errors.Errorf("you must supply a running container name or id")
	}
	if !portOpts.Latest && len(args) >= 1 {
		container = args[0]
	}
	port := ""
	if len(args) > 1 && !portOpts.Latest {
		port = args[1]
	}
	if len(args) == 1 && portOpts.Latest {
		port = args[0]
	}
	if len(port) > 0 {
		fields := strings.Split(port, "/")
		if len(fields) > 2 || len(fields) < 1 {
			return errors.Errorf("port formats are port/protocol. '%s' is invalid", port)
		}
		if len(fields) == 1 {
			fields = append(fields, "tcp")
		}

		portNum, err := strconv.Atoi(fields[0])
		if err != nil {
			return err
		}
		userPort = ocicni.PortMapping{
			HostPort:      0,
			ContainerPort: int32(portNum),
			Protocol:      fields[1],
			HostIP:        "",
		}
	}

	reports, err := registry.ContainerEngine().ContainerPort(registry.GetContext(), container, portOpts)
	if err != nil {
		return err
	}
	var found bool
	// Iterate mappings
	for _, report := range reports {
		for _, v := range report.Ports {
			hostIP := v.HostIP
			// Set host IP to 0.0.0.0 if blank
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}
			if portOpts.All {
				fmt.Printf("%s\t", report.Id[:12])
			}
			// If not searching by port or port/proto, then dump what we see
			if port == "" {
				fmt.Printf("%d/%s -> %s:%d\n", v.ContainerPort, v.Protocol, hostIP, v.HostPort)
				continue
			}
			if v.ContainerPort == userPort.ContainerPort {
				fmt.Printf("%s:%d\n", hostIP, v.HostPort)
				found = true
				break
			}
		}
		if !found && port != "" {
			return errors.Errorf("failed to find published port %q", port)
		}
	}
	return nil
}
