//go:build amd64 || arm64
// +build amd64 arm64

package machine

import (
	"encoding/json"
	"os"

	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] [MACHINE...]",
		Short:             "Inspect an existing machine",
		Long:              "Provide details on a managed virtual machine",
		RunE:              inspect,
		Example:           `podman machine inspect myvm`,
		ValidArgsFunction: autocompleteMachine,
	}
	inspectFlag = inspectFlagType{}
)

type inspectFlagType struct {
	format string
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  machineCmd,
	})

	flags := inspectCmd.Flags()
	formatFlagName := "format"
	flags.StringVar(&inspectFlag.format, formatFlagName, "", "Format volume output using JSON or a Go template")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(machine.InspectInfo{}))
}

func inspect(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)
	if len(args) < 1 {
		args = append(args, defaultMachineName)
	}
	vms := make([]machine.InspectInfo, 0, len(args))
	provider := getSystemDefaultProvider()
	for _, vmName := range args {
		vm, err := provider.LoadVMByName(vmName)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ii, err := vm.Inspect()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		vms = append(vms, *ii)
	}
	if len(inspectFlag.format) > 0 {
		// need jhonce to work his template magic
		return define.ErrNotImplemented
	}
	if err := printJSON(vms); err != nil {
		logrus.Error(err)
	}
	return errs.PrintErrors()
}

func printJSON(data []machine.InspectInfo) error {
	enc := json.NewEncoder(os.Stdout)
	// by default, json marshallers will force utf=8 from
	// a string. this breaks healthchecks that use <,>, &&.
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "     ")
	return enc.Encode(data)
}
