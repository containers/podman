package containers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/config"
	storageTransport "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/specgen"
	"github.com/containers/podman/v3/pkg/util"
	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	createDescription = `Creates a new container from the given image or storage and prepares it for running the specified command.

  The container ID is then printed to stdout. You can then start it at any time with the podman start <container_id> command. The container will be created with the initial state 'created'.`
	createCommand = &cobra.Command{
		Use:               "create [options] IMAGE [COMMAND [ARG...]]",
		Short:             "Create but do not start a container",
		Long:              createDescription,
		RunE:              create,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteCreateRun,
		Example: `podman create alpine ls
  podman create --annotation HELLO=WORLD alpine ls
  podman create -t -i --name myctr alpine ls`,
	}

	containerCreateCommand = &cobra.Command{
		Args:              createCommand.Args,
		Use:               createCommand.Use,
		Short:             createCommand.Short,
		Long:              createCommand.Long,
		RunE:              createCommand.RunE,
		ValidArgsFunction: createCommand.ValidArgsFunction,
		Example: `podman container create alpine ls
  podman container create --annotation HELLO=WORLD alpine ls
  podman container create -t -i --name myctr alpine ls`,
	}
)

var (
	cliVals common.ContainerCLIOpts
)

func createFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.SetInterspersed(false)
	common.DefineCreateFlags(cmd, &cliVals)
	common.DefineNetFlags(cmd)

	flags.SetNormalizeFunc(utils.AliasFlags)

	if registry.IsRemote() {
		_ = flags.MarkHidden("conmon-pidfile")
		_ = flags.MarkHidden("pidfile")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCommand,
	})
	createFlags(createCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerCreateCommand,
		Parent:  containerCmd,
	})
	createFlags(containerCreateCommand)
}

func create(cmd *cobra.Command, args []string) error {
	var (
		err error
	)
	cliVals.Net, err = common.NetFlagsToNetOptions(cmd, cliVals.Pod == "")
	if err != nil {
		return err
	}

	if err := createInit(cmd); err != nil {
		return err
	}

	imageName := args[0]
	rawImageName := ""
	if !cliVals.RootFS {
		rawImageName = args[0]
		name, err := pullImage(args[0])
		if err != nil {
			return err
		}
		imageName = name
	}
	s := specgen.NewSpecGenerator(imageName, cliVals.RootFS)
	if err := common.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	s.RawImageName = rawImageName

	if _, err := createPodIfNecessary(s, cliVals.Net); err != nil {
		return err
	}

	if cliVals.Replace {
		if err := replaceContainer(cliVals.Name); err != nil {
			return err
		}
	}

	report, err := registry.ContainerEngine().ContainerCreate(registry.GetContext(), s)
	if err != nil {
		return err
	}

	if cliVals.CIDFile != "" {
		if err := util.CreateCidFile(cliVals.CIDFile, report.Id); err != nil {
			return err
		}
	}

	fmt.Println(report.Id)
	return nil
}

func replaceContainer(name string) error {
	if len(name) == 0 {
		return errors.New("cannot replace container without --name being set")
	}
	rmOptions := entities.RmOptions{
		Force:  true, // force stop & removal
		Ignore: true, // ignore errors when a container doesn't exit
	}
	return removeContainers([]string{name}, rmOptions, false)
}

func createInit(c *cobra.Command) error {
	if c.Flag("shm-size").Changed {
		cliVals.ShmSize = c.Flag("shm-size").Value.String()
	}

	if (c.Flag("dns").Changed || c.Flag("dns-opt").Changed || c.Flag("dns-search").Changed) && (cliVals.Net.Network.NSMode == specgen.NoNetwork || cliVals.Net.Network.IsContainer()) {
		return errors.Errorf("conflicting options: dns and the network mode.")
	}

	if c.Flag("cpu-period").Changed && c.Flag("cpus").Changed {
		return errors.Errorf("--cpu-period and --cpus cannot be set together")
	}
	if c.Flag("cpu-quota").Changed && c.Flag("cpus").Changed {
		return errors.Errorf("--cpu-quota and --cpus cannot be set together")
	}

	noHosts, err := c.Flags().GetBool("no-hosts")
	if err != nil {
		return err
	}
	if noHosts && c.Flag("add-host").Changed {
		return errors.Errorf("--no-hosts and --add-host cannot be set together")
	}
	cliVals.UserNS = c.Flag("userns").Value.String()
	// if user did not modify --userns flag and did turn on
	// uid/gid mappings, set userns flag to "private"
	if !c.Flag("userns").Changed && cliVals.UserNS == "host" {
		if len(cliVals.UIDMap) > 0 ||
			len(cliVals.GIDMap) > 0 ||
			cliVals.SubUIDName != "" ||
			cliVals.SubGIDName != "" {
			cliVals.UserNS = "private"
		}
	}

	cliVals.IPC = c.Flag("ipc").Value.String()
	cliVals.UTS = c.Flag("uts").Value.String()
	cliVals.PID = c.Flag("pid").Value.String()
	cliVals.CgroupNS = c.Flag("cgroupns").Value.String()
	if c.Flag("entrypoint").Changed {
		val := c.Flag("entrypoint").Value.String()
		cliVals.Entrypoint = &val
	}

	if c.Flags().Changed("group-add") {
		groups := []string{}
		for _, g := range cliVals.GroupAdd {
			if g == "keep-groups" {
				if len(cliVals.GroupAdd) > 1 {
					return errors.New("the '--group-add keep-groups' option is not allowed with any other --group-add options")
				}
				if registry.IsRemote() {
					return errors.New("the '--group-add keep-groups' option is not supported in remote mode")
				}
				cliVals.Annotation = append(cliVals.Annotation, "run.oci.keep_original_groups=1")
			} else {
				groups = append(groups, g)
			}
		}
		cliVals.GroupAdd = groups
	}

	if c.Flags().Changed("pids-limit") {
		val := c.Flag("pids-limit").Value.String()
		pidsLimit, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return err
		}
		cliVals.PIDsLimit = &pidsLimit
	}
	if c.Flags().Changed("env") {
		env, err := c.Flags().GetStringArray("env")
		if err != nil {
			return errors.Wrapf(err, "retrieve env flag")
		}
		cliVals.Env = env
	}

	if c.Flag("cgroups").Changed && cliVals.CGroupsMode == "split" && registry.IsRemote() {
		return errors.Errorf("the option --cgroups=%q is not supported in remote mode", cliVals.CGroupsMode)
	}

	// Docker-compatibility: the "-h" flag for run/create is reserved for
	// the hostname (see https://github.com/containers/podman/issues/1367).

	return nil
}

// TODO: we should let the backend take care of the pull policy (which it
// does!). The code below is at risk of causing regression and code divergence.
func pullImage(imageName string) (string, error) {
	pullPolicy, err := config.ValidatePullPolicy(cliVals.Pull)
	if err != nil {
		return "", err
	}

	// Check if the image is missing and hence if we need to pull it.
	imageMissing := true
	imageRef, err := alltransports.ParseImageName(imageName)
	switch {
	case err != nil:
		// Assume we specified a local image without the explicit storage transport.
		fallthrough

	case imageRef.Transport().Name() == storageTransport.Transport.Name():
		br, err := registry.ImageEngine().Exists(registry.GetContext(), imageName)
		if err != nil {
			return "", err
		}
		imageMissing = !br.Value
	}

	if cliVals.Platform != "" || cliVals.Arch != "" || cliVals.OS != "" {
		if cliVals.Platform != "" {
			if cliVals.Arch != "" || cliVals.OS != "" {
				return "", errors.Errorf("--platform option can not be specified with --arch or --os")
			}
			split := strings.SplitN(cliVals.Platform, "/", 2)
			cliVals.OS = split[0]
			if len(split) > 1 {
				cliVals.Arch = split[1]
			}
		}

		if pullPolicy != config.PullPolicyAlways {
			logrus.Info("--platform --arch and --os causes the pull policy to be \"always\"")
			pullPolicy = config.PullPolicyAlways
		}
	}

	if imageMissing || pullPolicy == config.PullPolicyAlways {
		if pullPolicy == config.PullPolicyNever {
			return "", errors.Wrap(storage.ErrImageUnknown, imageName)
		}
		pullReport, pullErr := registry.ImageEngine().Pull(registry.GetContext(), imageName, entities.ImagePullOptions{
			Authfile:        cliVals.Authfile,
			Quiet:           cliVals.Quiet,
			Arch:            cliVals.Arch,
			OS:              cliVals.OS,
			Variant:         cliVals.Variant,
			SignaturePolicy: cliVals.SignaturePolicy,
			PullPolicy:      pullPolicy,
		})
		if pullErr != nil {
			return "", pullErr
		}
		imageName = pullReport.Images[0]
	}
	return imageName, nil
}

// createPodIfNecessary automatically creates a pod when requested.  if the pod name
// has the form new:ID, the pod ID is created and the name in the spec generator is replaced
// with ID.
func createPodIfNecessary(s *specgen.SpecGenerator, netOpts *entities.NetOptions) (*entities.PodCreateReport, error) {
	if !strings.HasPrefix(s.Pod, "new:") {
		return nil, nil
	}
	podName := strings.Replace(s.Pod, "new:", "", 1)
	if len(podName) < 1 {
		return nil, errors.Errorf("new pod name must be at least one character")
	}
	createOptions := entities.PodCreateOptions{
		Name:          podName,
		Infra:         true,
		Net:           netOpts,
		CreateCommand: os.Args,
		Hostname:      s.ContainerBasicConfig.Hostname,
	}
	// Unset config values we passed to the pod to prevent them being used twice for the container and pod.
	s.ContainerBasicConfig.Hostname = ""
	s.ContainerNetworkConfig = specgen.ContainerNetworkConfig{}

	s.Pod = podName
	return registry.ContainerEngine().PodCreate(context.Background(), createOptions)
}
