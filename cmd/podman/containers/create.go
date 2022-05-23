package containers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/config"
	cutil "github.com/containers/common/pkg/util"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/utils"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/mattn/go-isatty"
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
	InitContainerType string
	cliVals           entities.ContainerCreateOptions
)

func createFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	initContainerFlagName := "init-ctr"
	flags.StringVar(
		&InitContainerType,
		initContainerFlagName, "",
		"Make this a pod init container.",
	)

	flags.SetInterspersed(false)
	common.DefineCreateFlags(cmd, &cliVals, false, false)
	common.DefineNetFlags(cmd)

	flags.SetNormalizeFunc(utils.AliasFlags)

	if registry.IsRemote() {
		if cliVals.IsInfra {
			_ = flags.MarkHidden("infra-conmon-pidfile")
		} else {
			_ = flags.MarkHidden("conmon-pidfile")
		}

		_ = flags.MarkHidden("pidfile")
	}

	_ = cmd.RegisterFlagCompletionFunc(initContainerFlagName, completion.AutocompleteDefault)
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
	flags := cmd.Flags()
	cliVals.Net, err = common.NetFlagsToNetOptions(nil, *flags)
	if err != nil {
		return err
	}

	// Check if initctr is used with --pod and the value is correct
	if initctr := InitContainerType; cmd.Flags().Changed("init-ctr") {
		if !cmd.Flags().Changed("pod") {
			return errors.New("must specify pod value with init-ctr")
		}
		if !cutil.StringInSlice(initctr, []string{define.AlwaysInitContainer, define.OneShotInitContainer}) {
			return errors.Errorf("init-ctr value must be '%s' or '%s'", define.AlwaysInitContainer, define.OneShotInitContainer)
		}
		cliVals.InitContainerType = initctr
	}

	cliVals, err = CreateInit(cmd, cliVals, false)
	if err != nil {
		return err
	}
	imageName := args[0]
	rawImageName := ""
	if !cliVals.RootFS {
		rawImageName = args[0]
		name, err := PullImage(args[0], cliVals)
		if err != nil {
			return err
		}
		imageName = name
	}
	s := specgen.NewSpecGenerator(imageName, cliVals.RootFS)
	if err := specgenutil.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	s.RawImageName = rawImageName

	if err := createPodIfNecessary(cmd, s, cliVals.Net); err != nil {
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

	if cliVals.LogDriver != define.PassthroughLogging {
		fmt.Println(report.Id)
	}
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

func CreateInit(c *cobra.Command, vals entities.ContainerCreateOptions, isInfra bool) (entities.ContainerCreateOptions, error) {
	vals.UserNS = c.Flag("userns").Value.String()
	// if user did not modify --userns flag and did turn on
	// uid/gid mappings, set userns flag to "private"
	if !c.Flag("userns").Changed && vals.UserNS == "host" {
		if len(vals.UIDMap) > 0 ||
			len(vals.GIDMap) > 0 ||
			vals.SubUIDName != "" ||
			vals.SubGIDName != "" {
			vals.UserNS = "private"
		}
	}
	if c.Flag("kernel-memory") != nil && c.Flag("kernel-memory").Changed {
		logrus.Warnf("The --kernel-memory flag is no longer supported. This flag is a noop.")
	}

	if cliVals.LogDriver == define.PassthroughLogging {
		if isatty.IsTerminal(0) || isatty.IsTerminal(1) || isatty.IsTerminal(2) {
			return vals, errors.New("the '--log-driver passthrough' option cannot be used on a TTY")
		}
		if registry.IsRemote() {
			return vals, errors.New("the '--log-driver passthrough' option is not supported in remote mode")
		}
	}

	if !isInfra {
		if c.Flag("shm-size").Changed {
			vals.ShmSize = c.Flag("shm-size").Value.String()
		}
		if c.Flag("cpu-period").Changed && c.Flag("cpus").Changed {
			return vals, errors.Errorf("--cpu-period and --cpus cannot be set together")
		}
		if c.Flag("cpu-quota").Changed && c.Flag("cpus").Changed {
			return vals, errors.Errorf("--cpu-quota and --cpus cannot be set together")
		}
		vals.IPC = c.Flag("ipc").Value.String()
		vals.UTS = c.Flag("uts").Value.String()
		vals.PID = c.Flag("pid").Value.String()
		vals.CgroupNS = c.Flag("cgroupns").Value.String()

		if c.Flags().Changed("group-add") {
			groups := []string{}
			for _, g := range cliVals.GroupAdd {
				if g == "keep-groups" {
					if len(cliVals.GroupAdd) > 1 {
						return vals, errors.New("the '--group-add keep-groups' option is not allowed with any other --group-add options")
					}
					if registry.IsRemote() {
						return vals, errors.New("the '--group-add keep-groups' option is not supported in remote mode")
					}
					vals.Annotation = append(vals.Annotation, "run.oci.keep_original_groups=1")
				} else {
					groups = append(groups, g)
				}
			}
			vals.GroupAdd = groups
		}

		if c.Flags().Changed("oom-score-adj") {
			val, err := c.Flags().GetInt("oom-score-adj")
			if err != nil {
				return vals, err
			}
			vals.OOMScoreAdj = &val
		}
		if c.Flags().Changed("pids-limit") {
			val := c.Flag("pids-limit").Value.String()
			// Convert -1 to 0, so that -1 maps to unlimited pids limit
			if val == "-1" {
				val = "0"
			}
			pidsLimit, err := strconv.ParseInt(val, 10, 32)
			if err != nil {
				return vals, err
			}
			vals.PIDsLimit = &pidsLimit
		}
		if c.Flags().Changed("env") {
			env, err := c.Flags().GetStringArray("env")
			if err != nil {
				return vals, errors.Wrapf(err, "retrieve env flag")
			}
			vals.Env = env
		}
		if c.Flag("cgroups").Changed && vals.CgroupsMode == "split" && registry.IsRemote() {
			return vals, errors.Errorf("the option --cgroups=%q is not supported in remote mode", vals.CgroupsMode)
		}

		if c.Flag("pod").Changed && !strings.HasPrefix(c.Flag("pod").Value.String(), "new:") && c.Flag("userns").Changed {
			return vals, errors.Errorf("--userns and --pod cannot be set together")
		}
	}
	if (c.Flag("dns").Changed || c.Flag("dns-opt").Changed || c.Flag("dns-search").Changed) && vals.Net != nil && (vals.Net.Network.NSMode == specgen.NoNetwork || vals.Net.Network.IsContainer()) {
		return vals, errors.Errorf("conflicting options: dns and the network mode: " + string(vals.Net.Network.NSMode))
	}
	noHosts, err := c.Flags().GetBool("no-hosts")
	if err != nil {
		return vals, err
	}
	if noHosts && c.Flag("add-host").Changed {
		return vals, errors.Errorf("--no-hosts and --add-host cannot be set together")
	}

	if !isInfra && c.Flag("entrypoint").Changed {
		val := c.Flag("entrypoint").Value.String()
		vals.Entrypoint = &val
	}

	// Docker-compatibility: the "-h" flag for run/create is reserved for
	// the hostname (see https://github.com/containers/podman/issues/1367).

	return vals, nil
}

func PullImage(imageName string, cliVals entities.ContainerCreateOptions) (string, error) {
	pullPolicy, err := config.ParsePullPolicy(cliVals.Pull)
	if err != nil {
		return "", err
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
	}

	skipTLSVerify := types.OptionalBoolUndefined
	if cliVals.TLSVerify.Present() {
		skipTLSVerify = types.NewOptionalBool(!cliVals.TLSVerify.Value())
	}

	pullReport, pullErr := registry.ImageEngine().Pull(registry.GetContext(), imageName, entities.ImagePullOptions{
		Authfile:        cliVals.Authfile,
		Quiet:           cliVals.Quiet,
		Arch:            cliVals.Arch,
		OS:              cliVals.OS,
		Variant:         cliVals.Variant,
		SignaturePolicy: cliVals.SignaturePolicy,
		PullPolicy:      pullPolicy,
		SkipTLSVerify:   skipTLSVerify,
	})
	if pullErr != nil {
		return "", pullErr
	}

	// Return the input name such that the image resolves to correct
	// repo/tag in the backend (see #8082).  Unless we're referring to
	// the image via a transport.
	if _, err := alltransports.ParseImageName(imageName); err == nil {
		imageName = pullReport.Images[0]
	}

	return imageName, nil
}

// createPodIfNecessary automatically creates a pod when requested.  if the pod name
// has the form new:ID, the pod ID is created and the name in the spec generator is replaced
// with ID.
func createPodIfNecessary(cmd *cobra.Command, s *specgen.SpecGenerator, netOpts *entities.NetOptions) error {
	if !strings.HasPrefix(s.Pod, "new:") {
		return nil
	}
	podName := strings.Replace(s.Pod, "new:", "", 1)
	if len(podName) < 1 {
		return errors.Errorf("new pod name must be at least one character")
	}

	var err error
	uns := specgen.Namespace{NSMode: specgen.Default}
	if cliVals.UserNS != "" {
		uns, err = specgen.ParseNamespace(cliVals.UserNS)
		if err != nil {
			return err
		}
	}
	createOptions := entities.PodCreateOptions{
		Name:          podName,
		Infra:         true,
		Net:           netOpts,
		CreateCommand: os.Args,
		Hostname:      s.ContainerBasicConfig.Hostname,
		Cpus:          cliVals.CPUS,
		CpusetCpus:    cliVals.CPUSetCPUs,
		Pid:           cliVals.PID,
		Userns:        uns,
	}
	// Unset config values we passed to the pod to prevent them being used twice for the container and pod.
	s.ContainerBasicConfig.Hostname = ""
	s.ContainerNetworkConfig = specgen.ContainerNetworkConfig{}

	s.Pod = podName
	podSpec := entities.PodSpec{}
	podGen := specgen.NewPodSpecGenerator()
	podSpec.PodSpecGen = *podGen
	podGen, err = entities.ToPodSpecGen(podSpec.PodSpecGen, &createOptions)
	if err != nil {
		return err
	}

	infraOpts := entities.NewInfraContainerCreateOptions()
	infraOpts.Net = netOpts
	infraOpts.Quiet = true
	infraOpts.Hostname, err = cmd.Flags().GetString("hostname")
	if err != nil {
		return err
	}
	podGen.InfraContainerSpec = specgen.NewSpecGenerator("", false)
	podGen.InfraContainerSpec.NetworkOptions = podGen.NetworkOptions
	err = specgenutil.FillOutSpecGen(podGen.InfraContainerSpec, &infraOpts, []string{})
	if err != nil {
		return err
	}
	podSpec.PodSpecGen = *podGen
	_, err = registry.ContainerEngine().PodCreate(context.Background(), podSpec)
	return err
}
