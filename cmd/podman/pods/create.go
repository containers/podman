package pods

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/containers/libpod/cmd/podman/common"
	"github.com/containers/libpod/cmd/podman/parse"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/validate"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/pkg/errorhandling"
	createconfig "github.com/containers/libpod/pkg/spec"
	"github.com/containers/libpod/pkg/specgen"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	podCreateDescription = `After creating the pod, the pod ID is printed to stdout.

  You can then start it at any time with the  podman pod start <pod_id> command. The pod will be created with the initial state 'created'.`

	createCommand = &cobra.Command{
		Use:   "create",
		Args:  validate.NoArgs,
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
	flags.StringVar(&createOptions.InfraImage, "infra-image", containerConfig.Engine.InfraImage, "The image of the infra container to associate with the pod")
	flags.StringVar(&createOptions.InfraCommand, "infra-command", containerConfig.Engine.InfraCommand, "The command to run on the infra container when the pod is started")
	flags.StringSliceVar(&labelFile, "label-file", []string{}, "Read in a line delimited file of labels")
	flags.StringSliceVarP(&labels, "label", "l", []string{}, "Set metadata on pod (default [])")
	flags.StringVarP(&createOptions.Name, "name", "n", "", "Assign a name to the pod")
	flags.StringVarP(&createOptions.Hostname, "hostname", "", "", "Set a hostname to the pod")
	flags.StringVar(&podIDFile, "pod-id-file", "", "Write the pod ID to the file")
	flags.StringVar(&share, "share", createconfig.DefaultKernelNamespaces, "A comma delimited list of kernel namespaces the pod will share")
	flags.SetNormalizeFunc(aliasNetworkFlag)
}

func aliasNetworkFlag(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	if name == "net" {
		name = "network"
	}
	return pflag.NormalizedName(name)
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

	if !createOptions.Infra {
		if cmd.Flag("infra-command").Changed {
			return errors.New("cannot set infra-command without an infra container")
		}
		createOptions.InfraCommand = ""
		if cmd.Flag("infra-image").Changed {
			return errors.New("cannot set infra-image without an infra container")
		}
		createOptions.InfraImage = ""

		if cmd.Flag("share").Changed && share != "none" && share != "" {
			return fmt.Errorf("cannot set share(%s) namespaces without an infra container", cmd.Flag("share").Value)
		}
		createOptions.Share = nil
	} else {
		createOptions.Share = strings.Split(share, ",")
	}

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
	if cmd.Flag("network").Changed {
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
		case "slirp4netns":
			n.NSMode = specgen.Slirp
		default:
			// Container and NS mode are presently unsupported
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
