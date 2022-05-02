package pods

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/sysinfo"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/containers"
	"github.com/containers/podman/v4/cmd/podman/parse"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/docker/docker/pkg/parsers"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	podCreateDescription = `After creating the pod, the pod ID is printed to stdout.

  You can then start it at any time with the  podman pod start <pod_id> command. The pod will be created with the initial state 'created'.`

	createCommand = &cobra.Command{
		Use:               "create [options] [NAME]",
		Args:              cobra.MaximumNArgs(1),
		Short:             "Create a new empty pod",
		Long:              podCreateDescription,
		RunE:              create,
		ValidArgsFunction: completion.AutocompleteNone,
		Example: `podman pod create
  podman pod create --label foo=bar mypod`,
	}
)

var (
	createOptions     entities.PodCreateOptions
	infraOptions      = entities.NewInfraContainerCreateOptions()
	infraImage        string
	labels, labelFile []string
	podIDFile         string
	replace           bool
	share             string
	shareParent       bool
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCommand,
		Parent:  podCmd,
	})
	flags := createCommand.Flags()
	flags.SetInterspersed(false)
	common.DefineCreateDefaults(&infraOptions)
	common.DefineCreateFlags(createCommand, &infraOptions, entities.InfraMode)
	common.DefineNetFlags(createCommand)

	flags.BoolVar(&createOptions.Infra, "infra", true, "Create an infra container associated with the pod to share namespaces with")

	nameFlagName := "name"
	flags.StringVarP(&createOptions.Name, nameFlagName, "n", "", "Assign a name to the pod")
	_ = createCommand.RegisterFlagCompletionFunc(nameFlagName, completion.AutocompleteNone)

	policyFlag := "exit-policy"
	flags.StringVarP(&createOptions.ExitPolicy, policyFlag, "", string(containerConfig.Engine.PodExitPolicy), "Behaviour when the last container exits")
	_ = createCommand.RegisterFlagCompletionFunc(policyFlag, common.AutocompletePodExitPolicy)

	infraImageFlagName := "infra-image"
	var defInfraImage string
	if !registry.IsRemote() {
		defInfraImage = containerConfig.Engine.InfraImage
	}
	flags.StringVar(&infraImage, infraImageFlagName, defInfraImage, "Image to use to override builtin infra container")
	_ = createCommand.RegisterFlagCompletionFunc(infraImageFlagName, common.AutocompleteImages)

	podIDFileFlagName := "pod-id-file"
	flags.StringVar(&podIDFile, podIDFileFlagName, "", "Write the pod ID to the file")
	_ = createCommand.RegisterFlagCompletionFunc(podIDFileFlagName, completion.AutocompleteDefault)

	flags.BoolVar(&replace, "replace", false, "If a pod with the same name exists, replace it")

	shareFlagName := "share"
	flags.StringVar(&share, shareFlagName, specgen.DefaultKernelNamespaces, "A comma delimited list of kernel namespaces the pod will share")
	_ = createCommand.RegisterFlagCompletionFunc(shareFlagName, common.AutocompletePodShareNamespace)

	shareParentFlagName := "share-parent"
	flags.BoolVar(&shareParent, shareParentFlagName, true, "Set the pod's cgroup as the cgroup parent for all containers joining the pod")

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func create(cmd *cobra.Command, args []string) error {
	var (
		err          error
		imageName    string
		rawImageName string
		podName      string
	)
	if len(args) > 0 {
		if len(createOptions.Name) > 0 {
			return fmt.Errorf("cannot specify --name and NAME at the same time")
		}
		createOptions.Name = args[0]
	}
	labelFile = infraOptions.LabelFile
	labels = infraOptions.Label
	createOptions.Labels, err = parse.GetAllLabels(labelFile, labels)
	if err != nil {
		return fmt.Errorf("unable to process labels: %w", err)
	}

	if cmd.Flag("infra-image").Changed {
		imageName = infraImage
	}
	img := imageName

	if !cmd.Flag("infra").Changed && (share == "none" || share == "") {
		// we do not want an infra container when not sharing namespaces
		createOptions.Infra = false
	}

	report, err := registry.ContainerEngine().NetworkExists(registry.Context(), "pasta")
	if err != nil {
		return err
	}
	pastaNetworkNameExists := report.Value

	if !createOptions.Infra {
		if cmd.Flag("no-hosts").Changed {
			return fmt.Errorf("cannot specify --no-hosts without an infra container")
		}
		flags := cmd.Flags()
		createOptions.Net, err = common.NetFlagsToNetOptions(nil, *flags, pastaNetworkNameExists)
		if err != nil {
			return err
		}
		logrus.Debugf("Not creating an infra container")
		createOptions.InfraImage = ""
		if createOptions.InfraName != "" {
			return errors.New("cannot set infra-name without an infra container")
		}

		if cmd.Flag("share").Changed && share != "none" && share != "" {
			return fmt.Errorf("cannot set share(%s) namespaces without an infra container", cmd.Flag("share").Value)
		}
		createOptions.Share = nil
	} else {
		// reassign certain options for lbpod api, these need to be populated in spec
		flags := cmd.Flags()
		infraOptions.Net, err = common.NetFlagsToNetOptions(nil, *flags, pastaNetworkNameExists)
		if err != nil {
			return err
		}
		infraOptions, err = containers.CreateInit(cmd, infraOptions, true)
		if err != nil {
			return err
		}
		if strings.Contains(share, "cgroup") && shareParent {
			return fmt.Errorf("cannot define the pod as the cgroup parent at the same time as joining the infra container's cgroupNS: %w", define.ErrInvalidArg)
		}

		if strings.HasPrefix(share, "+") {
			createOptions.Share = append(createOptions.Share, strings.Split(specgen.DefaultKernelNamespaces, ",")...)
			share = share[1:]
		}
		createOptions.Share = append(createOptions.Share, strings.Split(share, ",")...)
		createOptions.ShareParent = &shareParent
		if cmd.Flag("infra-command").Changed {
			// Only send content to server side if user changed defaults
			cmdIn, err := cmd.Flags().GetString("infra-command")
			infraOptions.Entrypoint = &cmdIn
			if err != nil {
				return err
			}
		}
		podName = createOptions.Name
		err = common.ContainerToPodOptions(&infraOptions, &createOptions)
		if err != nil {
			return err
		}
		createOptions.Name = podName
	}

	if len(createOptions.Net.PublishPorts) > 0 {
		if !createOptions.Infra {
			return fmt.Errorf("you must have an infra container to publish port bindings to the host")
		}
	}

	createOptions.CreateCommand = os.Args

	if replace {
		if err := replacePod(createOptions.Name); err != nil {
			return err
		}
	}

	numCPU := sysinfo.NumCPU()
	if numCPU == 0 {
		numCPU = runtime.NumCPU()
	}
	if createOptions.Cpus > float64(numCPU) {
		createOptions.Cpus = float64(numCPU)
	}
	copy := infraOptions.CPUSetCPUs
	cpuSet := infraOptions.CPUS
	if cpuSet == 0 {
		cpuSet = float64(sysinfo.NumCPU())
	}
	ret, err := parsers.ParseUintList(copy)
	copy = ""
	if err != nil {
		return fmt.Errorf("could not parse list: %w", err)
	}
	var vals []int
	for ind, val := range ret {
		if val {
			vals = append(vals, ind)
		}
	}
	sort.Ints(vals)
	for ind, core := range vals {
		switch {
		case core > int(cpuSet):
			if copy == "" {
				copy = "0-" + strconv.Itoa(int(cpuSet))
				infraOptions.CPUSetCPUs = copy
				break
			} else {
				infraOptions.CPUSetCPUs = copy
				break
			}
		case ind != 0:
			copy += "," + strconv.Itoa(core)
		default:
			copy = "" + strconv.Itoa(core)
		}
	}
	createOptions.Cpus = infraOptions.CPUS
	createOptions.CpusetCpus = infraOptions.CPUSetCPUs
	podSpec := specgen.NewPodSpecGenerator()
	podSpec, err = entities.ToPodSpecGen(*podSpec, &createOptions)
	if err != nil {
		return err
	}
	if createOptions.Infra {
		rawImageName = img
		podSpec.InfraImage = imageName
		if infraOptions.Entrypoint != nil {
			createOptions.InfraCommand = infraOptions.Entrypoint
		}
		podSpec.InfraContainerSpec = specgen.NewSpecGenerator(imageName, false)
		podSpec.InfraContainerSpec.RawImageName = rawImageName
		podSpec.InfraContainerSpec.NetworkOptions = podSpec.NetworkOptions
		err = specgenutil.FillOutSpecGen(podSpec.InfraContainerSpec, &infraOptions, []string{})
		if err != nil {
			return err
		}

		podSpec.Volumes = podSpec.InfraContainerSpec.Volumes
		podSpec.ImageVolumes = podSpec.InfraContainerSpec.ImageVolumes
		podSpec.OverlayVolumes = podSpec.InfraContainerSpec.OverlayVolumes
		podSpec.Mounts = podSpec.InfraContainerSpec.Mounts

		// Marshall and Unmarshal the spec in order to map similar entities
		wrapped, err := json.Marshal(podSpec.InfraContainerSpec)
		if err != nil {
			return err
		}
		err = json.Unmarshal(wrapped, podSpec)
		if err != nil {
			return err
		}
		podSpec.Name = podName
	}
	PodSpec := entities.PodSpec{PodSpecGen: *podSpec}
	response, err := registry.ContainerEngine().PodCreate(context.Background(), PodSpec)
	if err != nil {
		return err
	}

	if len(podIDFile) > 0 {
		if err := util.CreateIDFile(podIDFile, response.Id); err != nil {
			return fmt.Errorf("failed to write pod ID to file: %w", err)
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
	errs := removePods([]string{name}, rmOptions, false)
	return errs.PrintErrors()
}
