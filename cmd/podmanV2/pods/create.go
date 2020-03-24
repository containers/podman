package pods

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podmanV2/common"
	"github.com/containers/libpod/cmd/podmanV2/parse"
	"github.com/containers/libpod/cmd/podmanV2/registry"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/errorhandling"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	podCreateDescription = `After creating the pod, the pod ID is printed to stdout.

  You can then start it at any time with the  podman pod start <pod_id> command. The pod will be created with the initial state 'created'.`

	createCommand = &cobra.Command{
		Use:   "create",
		Args:  cobra.NoArgs,
		Short: "Create a new empty pod",
		Long:  podCreateDescription,
		RunE:  create,
	}
)

var (
	createOptions     entities.PodCreateOptions
	labels, labelFile []string
	podIDFile         string
	share             string
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: createCommand,
		Parent:  podCmd,
	})
	flags := createCommand.Flags()
	flags.SetInterspersed(false)
	flags.AddFlagSet(common.GetNetFlags())
	flags.StringVar(&createOptions.CGroupParent, "cgroup-parent", "", "Set parent cgroup for the pod")
	flags.BoolVar(&createOptions.Infra, "infra", true, "Create an infra container associated with the pod to share namespaces with")
	flags.StringVar(&createOptions.InfraImage, "infra-image", define.DefaultInfraImage, "The image of the infra container to associate with the pod")
	flags.StringVar(&createOptions.InfraCommand, "infra-command", define.DefaultInfraCommand, "The command to run on the infra container when the pod is started")
	flags.StringSliceVar(&labelFile, "label-file", []string{}, "Read in a line delimited file of labels")
	flags.StringSliceVarP(&labels, "label", "l", []string{}, "Set metadata on pod (default [])")
	flags.StringVarP(&createOptions.Name, "name", "n", "", "Assign a name to the pod")
	flags.StringVarP(&createOptions.Hostname, "hostname", "", "", "Set a hostname to the pod")
	flags.StringVar(&podIDFile, "pod-id-file", "", "Write the pod ID to the file")
	flags.StringVar(&share, "share", common.DefaultKernelNamespaces, "A comma delimited list of kernel namespaces the pod will share")
}

func create(cmd *cobra.Command, args []string) error {
	var (
		err       error
		podIdFile *os.File
	)
	createOptions.Labels, err = parse.GetAllLabels(labelFile, labels)
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}

	if !createOptions.Infra && cmd.Flag("share").Changed && share != "none" && share != "" {
		return errors.Errorf("You cannot share kernel namespaces on the pod level without an infra container")
	}
	createOptions.Share = strings.Split(share, ",")
	if cmd.Flag("pod-id-file").Changed {
		podIdFile, err = util.OpenExclusiveFile(podIDFile)
		if err != nil && os.IsExist(err) {
			return errors.Errorf("pod id file exists. Ensure another pod is not using it or delete %s", podIDFile)
		}
		if err != nil {
			return errors.Errorf("error opening pod-id-file %s", podIDFile)
		}
		defer errorhandling.CloseQuiet(podIdFile)
		defer errorhandling.SyncQuiet(podIdFile)
	}

	createOptions.Net, err = common.NetFlagsToNetOptions(cmd)
	if err != nil {
		return err
	}
	netInput, err := cmd.Flags().GetString("network")
	if err != nil {
		return err
	}
	n := specgen.Namespace{}
	switch netInput {
	case "bridge":
		n.NSMode = specgen.Bridge
	case "host":
		n.NSMode = specgen.Host
	case "slip4netns":
		n.NSMode = specgen.Slirp
	default:
		if strings.HasPrefix(netInput, "container:") { //nolint
			split := strings.Split(netInput, ":")
			if len(split) != 2 {
				return errors.Errorf("invalid network paramater: %q", netInput)
			}
			n.NSMode = specgen.FromContainer
			n.Value = split[1]
		} else if strings.HasPrefix(netInput, "ns:") {
			return errors.New("the ns: network option is not supported for pods")
		} else {
			n.NSMode = specgen.Bridge
			createOptions.Net.CNINetworks = strings.Split(netInput, ",")
		}
	}
	if len(createOptions.Net.PublishPorts) > 0 {
		if !createOptions.Infra {
			return errors.Errorf("you must have an infra container to publish port bindings to the host")
		}
	}

	response, err := registry.ContainerEngine().PodCreate(context.Background(), createOptions)
	if err != nil {
		return err
	}
	fmt.Println(response.Id)
	return nil
}
