package pods

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/podman/v2/cmd/podman/common"
	"github.com/containers/podman/v2/cmd/podman/parse"
	"github.com/containers/podman/v2/cmd/podman/registry"
	"github.com/containers/podman/v2/cmd/podman/validate"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/errorhandling"
	"github.com/containers/podman/v2/pkg/specgen"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	podCreateDescription = `After creating the pod, the pod ID is printed to stdout.

  You can then start it at any time with the  podman pod start <pod_id> command. The pod will be created with the initial state 'created'.`

	createCommand = &cobra.Command{
		Use:   "create [options]",
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
	replace           bool
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
	flags.StringVar(&createOptions.InfraConmonPidFile, "infra-conmon-pidfile", "", "Path to the file that will receive the POD of the infra container's conmon")
	flags.String("infra-image", containerConfig.Engine.InfraImage, "The image of the infra container to associate with the pod")
	flags.String("infra-command", containerConfig.Engine.InfraCommand, "The command to run on the infra container when the pod is started")
	flags.StringSliceVar(&labelFile, "label-file", []string{}, "Read in a line delimited file of labels")
	flags.StringSliceVarP(&labels, "label", "l", []string{}, "Set metadata on pod (default [])")
	flags.StringVarP(&createOptions.Name, "name", "n", "", "Assign a name to the pod")
	flags.StringVarP(&createOptions.Hostname, "hostname", "", "", "Set a hostname to the pod")
	flags.StringVar(&podIDFile, "pod-id-file", "", "Write the pod ID to the file")
	flags.BoolVar(&replace, "replace", false, "If a pod with the same exists, replace it")
	flags.StringVar(&share, "share", specgen.DefaultKernelNamespaces, "A comma delimited list of kernel namespaces the pod will share")
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
		err     error
		podIDFD *os.File
	)
	createOptions.Labels, err = parse.GetAllLabels(labelFile, labels)
	if err != nil {
		return errors.Wrapf(err, "unable to process labels")
	}

	if !createOptions.Infra {
		logrus.Debugf("Not creating an infra container")
		if cmd.Flag("infra-conmon-pidfile").Changed {
			return errors.New("cannot set infra-conmon-pid without an infra container")
		}
		if cmd.Flag("infra-command").Changed {
			return errors.New("cannot set infra-command without an infra container")
		}
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
		if cmd.Flag("infra-command").Changed {
			// Only send content to server side if user changed defaults
			createOptions.InfraCommand, err = cmd.Flags().GetString("infra-command")
			if err != nil {
				return err
			}
		}
		if cmd.Flag("infra-image").Changed {
			// Only send content to server side if user changed defaults
			createOptions.InfraImage, err = cmd.Flags().GetString("infra-image")
			if err != nil {
				return err
			}
		}
	}

	if cmd.Flag("pod-id-file").Changed {
		podIDFD, err = util.OpenExclusiveFile(podIDFile)
		if err != nil && os.IsExist(err) {
			return errors.Errorf("pod id file exists. Ensure another pod is not using it or delete %s", podIDFile)
		}
		if err != nil {
			return errors.Errorf("error opening pod-id-file %s", podIDFile)
		}
		defer errorhandling.CloseQuiet(podIDFD)
		defer errorhandling.SyncQuiet(podIDFD)
	}

	createOptions.Net, err = common.NetFlagsToNetOptions(cmd)
	if err != nil {
		return err
	}
	createOptions.Net.Network = specgen.Namespace{}
	if cmd.Flag("network").Changed {
		netInput, err := cmd.Flags().GetString("network")
		if err != nil {
			return err
		}
		parts := strings.SplitN(netInput, ":", 2)

		n := specgen.Namespace{}
		switch {
		case netInput == "bridge":
			n.NSMode = specgen.Bridge
		case netInput == "host":
			n.NSMode = specgen.Host
		case netInput == "slirp4netns", strings.HasPrefix(netInput, "slirp4netns:"):
			n.NSMode = specgen.Slirp
			if len(parts) > 1 {
				createOptions.Net.NetworkOptions = make(map[string][]string)
				createOptions.Net.NetworkOptions[parts[0]] = strings.Split(parts[1], ",")
			}
		default:
			// Container and NS mode are presently unsupported
			n.NSMode = specgen.Bridge
			createOptions.Net.CNINetworks = strings.Split(netInput, ",")
		}
		createOptions.Net.Network = n
	}
	if len(createOptions.Net.PublishPorts) > 0 {
		if !createOptions.Infra {
			return errors.Errorf("you must have an infra container to publish port bindings to the host")
		}
	}

	createOptions.CreateCommand = os.Args

	if replace {
		if err := replacePod(createOptions.Name); err != nil {
			return err
		}
	}

	response, err := registry.ContainerEngine().PodCreate(context.Background(), createOptions)
	if err != nil {
		return err
	}
	if len(podIDFile) > 0 {
		if err = ioutil.WriteFile(podIDFile, []byte(response.Id), 0644); err != nil {
			return errors.Wrapf(err, "failed to write pod ID to file %q", podIDFile)
		}
	}
	fmt.Println(response.Id)
	return nil
}

func replacePod(name string) error {
	if len(name) == 0 {
		return errors.New("cannot replace pod without --name being set")
	}
	rmOptions := entities.PodRmOptions{
		Force:  true, // stop and remove pod
		Ignore: true, // ignore if pod doesn't exist
	}
	return removePods([]string{name}, rmOptions, false)
}
