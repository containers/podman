// +build amd64 arm64

package machine

import (
	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/spf13/cobra"
)

var (
	setCmd = &cobra.Command{
		Use:               "set [options] [NAME]",
		Short:             "Sets a virtual machine setting",
		Long:              "Sets an updatable virtual machine setting",
		RunE:              setMachine,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine set --rootful=false`,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

var (
	setOpts = machine.SetOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: setCmd,
		Parent:  machineCmd,
	})
	flags := setCmd.Flags()

	rootfulFlagName := "rootful"
	flags.BoolVar(&setOpts.Rootful, rootfulFlagName, false, "Whether this machine should prefer rootful container execution")
}

func setMachine(cmd *cobra.Command, args []string) error {
	var (
		vm  machine.VM
		err error
	)

	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}
	provider := getSystemDefaultProvider()
	vm, err = provider.LoadVMByName(vmName)
	if err != nil {
		return err
	}

	return vm.Set(vmName, setOpts)
}
