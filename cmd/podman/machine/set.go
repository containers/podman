//go:build amd64 || arm64
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

	ProviderTypeFlagName := "type"
	flags.StringVar(&providerType, ProviderTypeFlagName, "", "Type of VM provider")
	_ = setCmd.RegisterFlagCompletionFunc(ProviderTypeFlagName, completion.AutocompleteNone)
}

func setMachine(cmd *cobra.Command, args []string) error {
	var (
		vmName   string
		vm       machine.VM
		err      error
		provider machine.Provider
	)

	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	vmName, provider, err = getProviderByVMName(vmName)
	if err != nil {
		return err
	}

	vm, err = provider.LoadVMByName(vmName)
	if err != nil {
		return err
	}

	return vm.Set(vmName, setOpts)
}
