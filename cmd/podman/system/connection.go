// +build !remote

package system

import (
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/spf13/cobra"
)

var (
	connectionDescription = `TBD
`
	connectionCommand = &cobra.Command{
		Use: "connection",
		//Args:    validate.NoArgs,
		Long:  connectionDescription,
		Short: "Add remote ssh connection",
		RunE:  connection,
		Example: `podman system connection server.foobar.com
podman system connection --identity ~/.ssh/dev_rsa --default root@server.foobar.com:222`,
	}
)

var connectionOptions = struct {
	Alias      string
	Default    bool
	Identity   string
	SocketPath string
	User       string
}{}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode},
		Command: connectionCommand,
		Parent:  systemCmd,
	})
	flags := connectionCommand.Flags()
	flags.StringVar(&connectionOptions.Alias, "alias", "", "alias name for connection")
	flags.BoolVar(&connectionOptions.Default, "default", false, "set as the default connection")
	flags.StringVar(&connectionOptions.Identity, "identity", "", "path to ssh identity file")
	//flags.StringVar(&connectionOptions.User, "user", "", "remote username")
	flags.StringVar(&connectionOptions.SocketPath, "socket-path", "", "path to podman socket on remote host")
}

func connection(cmd *cobra.Command, args []string) error {
	// if no user is provided, assume local user name
	// if no socket is provided, then do an ssh to look for it
	// default connection, if exists, is then assumed with podman remote
	// if no identity exists, should we be prompting for password?

	return nil
}
