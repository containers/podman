//go:build remote

package images

import (
	"errors"
	"fmt"
	"os"
	"strings"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/common/pkg/auth"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// buildFlagsWrapper holds the CLI flags for remote builds
type buildFlagsWrapper struct {
	// Essential flags for remote builds
	Tag           []string
	File          []string
	BuildArg      []string
	Pull          string
	PullAlways    bool
	PullNever     bool
	Layers        bool
	ForceRm       bool
	Squash        bool
	SquashAll     bool
	Network       string
	Target        string
	Quiet         bool
	NoCache       bool
	Rm            bool
	Authfile      string
	Format        string
	Isolation     string
	Runtime       string
	RuntimeFlags  []string
	OS            string
	Arch          string
	Variant       string
	Platform      string
	Memory        string
	MemorySwap    string
	CPUPeriod     string
	CPUQuota      string
	CPUShares     string
	CPUSetCPUs    string
	CPUSetMems    string
	BuildContext  []string
	CapAdd        []string
	CapDrop       []string
	Devices       []string
	DNSServers    []string
	DNSSearch     []string
	DNSOptions    []string
	Envs          []string
	Label         []string
	Annotation    []string
	Volumes       []string
	IPC           string
	PID           string
	UserNS        string
	UTS           string
	Cgroup        string
	SecurityOpt   []string
	Secrets       []string
	SSHSources    []string
	Output        string
	IgnoreFile    string
	Timestamp     int64
	Logfile       string
	Jobs          uint
	AllPlatforms  bool
	Manifest      string
	UnsetEnvs     []string
	Ulimit        []string
}

var (
	buildDescription = "Builds an OCI or Docker image using instructions from one or more Containerfiles and a specified build context directory."
	buildCmd         = &cobra.Command{
		Use:               "build [options] [CONTEXT]",
		Short:             "Build an image using instructions from Containerfiles",
		Long:              buildDescription,
		Args:              cobra.MaximumNArgs(1),
		RunE:              build,
		ValidArgsFunction: common.AutocompleteDefaultOneArg,
		Example: `podman build .
  podman build --creds=username:password -t imageName -f Containerfile.simple .
  podman build --layers --force-rm --tag imageName .`,
	}

	imageBuildCmd = &cobra.Command{
		Args:              buildCmd.Args,
		Use:               buildCmd.Use,
		Short:             buildCmd.Short,
		Long:              buildCmd.Long,
		RunE:              buildCmd.RunE,
		ValidArgsFunction: buildCmd.ValidArgsFunction,
		Example: `podman image build .
  podman image build --creds=username:password -t imageName -f Containerfile.simple .
  podman image build --layers --force-rm --tag imageName .`,
	}

	buildxBuildCmd = &cobra.Command{
		Args:              buildCmd.Args,
		Use:               buildCmd.Use,
		Short:             buildCmd.Short,
		Long:              buildCmd.Long,
		RunE:              buildCmd.RunE,
		ValidArgsFunction: buildCmd.ValidArgsFunction,
		Example: `podman buildx build .
  podman buildx build --creds=username:password -t imageName -f Containerfile.simple .
  podman buildx build --layers --force-rm --tag imageName .`,
	}

	buildOpts = buildFlagsWrapper{}
)

func useLayers() string {
	layers := os.Getenv("BUILDAH_LAYERS")
	if strings.ToLower(layers) == "false" || layers == "0" {
		return "false"
	}
	return "true"
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildCmd,
	})
	buildFlags(buildCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageBuildCmd,
		Parent:  imageCmd,
	})
	buildFlags(imageBuildCmd)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildxBuildCmd,
		Parent:  buildxCmd,
	})
	buildFlags(buildxBuildCmd)
}

func buildFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	// buildx flags
	flags.Bool("load", false, "buildx --load")
	_ = flags.MarkHidden("load")
	flags.String("progress", "auto", "buildx --progress")
	_ = flags.MarkHidden("progress")

	// Podman flags
	flags.BoolVarP(&buildOpts.SquashAll, "squash-all", "", false, "Squash all layers into a single layer")
	flags.StringArrayVarP(&buildOpts.Tag, "tag", "t", []string{}, "Name and optionally a tag (format: `name:tag`)")
	flags.StringArrayVarP(&buildOpts.File, "file", "f", []string{}, "`pathname or URL` of a Dockerfile")
	flags.StringArrayVar(&buildOpts.BuildArg, "build-arg", []string{}, "Set build-time variables")
	flags.StringVar(&buildOpts.Pull, "pull", "true", "Pull image policy")
	flags.BoolVar(&buildOpts.PullAlways, "pull-always", false, "Always pull the image")
	flags.BoolVar(&buildOpts.PullNever, "pull-never", false, "Never pull the image")
	flags.BoolVar(&buildOpts.Layers, "layers", useLayers() == "true", "Use intermediate layers during build")
	flags.BoolVar(&buildOpts.ForceRm, "force-rm", true, "Always remove intermediate containers")
	flags.BoolVar(&buildOpts.Squash, "squash", false, "Squash all image layers into a single layer")
	flags.StringVar(&buildOpts.Network, "network", "", "Set the networking mode for RUN instructions")
	flags.StringVar(&buildOpts.Target, "target", "", "Set the target build stage to build")
	flags.BoolVarP(&buildOpts.Quiet, "quiet", "q", false, "Suppress output messages")
	flags.BoolVar(&buildOpts.NoCache, "no-cache", false, "Do not use existing cached images")
	flags.BoolVar(&buildOpts.Rm, "rm", true, "Remove intermediate containers after a successful build")
	flags.StringVar(&buildOpts.Authfile, "authfile", "", "Path to authentication file")
	flags.StringVar(&buildOpts.Format, "format", "oci", "Format of the built image")
	flags.StringVar(&buildOpts.Isolation, "isolation", "", "Type of isolation")
	flags.StringVar(&buildOpts.Runtime, "runtime", "", "Path to an alternate OCI runtime")
	flags.StringArrayVar(&buildOpts.RuntimeFlags, "runtime-flag", []string{}, "Runtime flags")
	flags.StringVar(&buildOpts.OS, "os", "", "Set OS for the build")
	flags.StringVar(&buildOpts.Arch, "arch", "", "Set ARCH for the build")
	flags.StringVar(&buildOpts.Variant, "variant", "", "Set architecture variant")
	flags.StringVar(&buildOpts.Platform, "platform", "", "Set OS/ARCH/VARIANT")
	flags.StringVar(&buildOpts.Memory, "memory", "", "Memory limit")
	flags.StringVar(&buildOpts.MemorySwap, "memory-swap", "", "Swap limit")
	flags.StringVar(&buildOpts.CPUPeriod, "cpu-period", "", "Limit CPU CFS period")
	flags.StringVar(&buildOpts.CPUQuota, "cpu-quota", "", "Limit CPU CFS quota")
	flags.StringVar(&buildOpts.CPUShares, "cpu-shares", "", "CPU shares")
	flags.StringVar(&buildOpts.CPUSetCPUs, "cpuset-cpus", "", "CPUs to use")
	flags.StringVar(&buildOpts.CPUSetMems, "cpuset-mems", "", "Memory nodes to use")
	flags.StringArrayVar(&buildOpts.BuildContext, "build-context", []string{}, "Additional build context")
	flags.StringArrayVar(&buildOpts.CapAdd, "cap-add", []string{}, "Add capabilities")
	flags.StringArrayVar(&buildOpts.CapDrop, "cap-drop", []string{}, "Drop capabilities")
	flags.StringArrayVar(&buildOpts.Devices, "device", []string{}, "Additional devices")
	flags.StringArrayVar(&buildOpts.DNSServers, "dns", []string{}, "DNS servers")
	flags.StringArrayVar(&buildOpts.DNSSearch, "dns-search", []string{}, "DNS search domains")
	flags.StringArrayVar(&buildOpts.DNSOptions, "dns-option", []string{}, "DNS options")
	flags.StringArrayVarP(&buildOpts.Envs, "env", "e", []string{}, "Set environment variables")
	flags.StringArrayVar(&buildOpts.Label, "label", []string{}, "Set metadata for an image")
	flags.StringArrayVar(&buildOpts.Annotation, "annotation", []string{}, "Set image annotation")
	flags.StringArrayVarP(&buildOpts.Volumes, "volume", "v", []string{}, "Bind mount a volume")
	flags.StringVar(&buildOpts.IPC, "ipc", "", "IPC namespace")
	flags.StringVar(&buildOpts.PID, "pid", "", "PID namespace")
	flags.StringVar(&buildOpts.UserNS, "userns", "", "User namespace")
	flags.StringVar(&buildOpts.UTS, "uts", "", "UTS namespace")
	flags.StringVar(&buildOpts.Cgroup, "cgroupns", "", "Cgroup namespace")
	flags.StringArrayVar(&buildOpts.SecurityOpt, "security-opt", []string{}, "Security options")
	flags.StringArrayVar(&buildOpts.Secrets, "secret", []string{}, "Secrets to expose")
	flags.StringArrayVar(&buildOpts.SSHSources, "ssh", []string{}, "SSH agent socket or keys")
	flags.StringVarP(&buildOpts.Output, "output", "o", "", "Output destination")
	flags.StringVar(&buildOpts.IgnoreFile, "ignorefile", "", "Path to ignore file")
	flags.Int64Var(&buildOpts.Timestamp, "timestamp", 0, "Set created timestamp")
	flags.StringVar(&buildOpts.Logfile, "logfile", "", "Log to file")
	flags.UintVarP(&buildOpts.Jobs, "jobs", "j", 1, "Number of stages to run in parallel")
	flags.BoolVar(&buildOpts.AllPlatforms, "all-platforms", false, "Build for all platforms")
	flags.StringVar(&buildOpts.Manifest, "manifest", "", "Add to manifest list")
	flags.StringArrayVar(&buildOpts.UnsetEnvs, "unsetenv", []string{}, "Unset environment variables")
	flags.StringArrayVar(&buildOpts.Ulimit, "ulimit", []string{}, "Ulimit options")

	// Hidden flags
	_ = flags.MarkHidden("disable-content-trust")
	_ = flags.MarkHidden("sign-by")
	_ = flags.MarkHidden("signature-policy")
	_ = flags.MarkHidden("tls-verify")
	_ = flags.MarkHidden("compress")
}

// build executes the build command for remote
func build(cmd *cobra.Command, args []string) error {
	if (cmd.Flags().Changed("squash") && cmd.Flags().Changed("layers")) ||
		(cmd.Flags().Changed("squash-all") && cmd.Flags().Changed("squash")) {
		return errors.New("cannot specify --squash with --layers and --squash-all with --squash")
	}

	if buildOpts.Network == "none" {
		if cmd.Flag("dns").Changed {
			return errors.New("the --dns option cannot be used with --network=none")
		}
		if cmd.Flag("dns-option").Changed {
			return errors.New("the --dns-option option cannot be used with --network=none")
		}
		if cmd.Flag("dns-search").Changed {
			return errors.New("the --dns-search option cannot be used with --network=none")
		}
	}

	// Validate authfile if specified
	if cmd.Flag("authfile").Changed {
		if err := auth.CheckAuthFile(buildOpts.Authfile); err != nil {
			return err
		}
	}

	// Extract container files from CLI
	var containerFiles []string
	for _, f := range buildOpts.File {
		if f == "-" {
			containerFiles = append(containerFiles, "/dev/stdin")
		} else {
			containerFiles = append(containerFiles, f)
		}
	}

	// Determine context directory
	var contextDir string
	if len(args) > 0 {
		contextDir = args[0]
	} else {
		contextDir = "."
	}

	// Handle stdin: when context is "-", Dockerfile comes from stdin
	if contextDir == "-" {
		// Add /dev/stdin to containerFiles if not already present
		if len(containerFiles) == 0 {
			containerFiles = append(containerFiles, "/dev/stdin")
		}
		// Use current directory as context
		contextDir = "."
	}

	// Build entities.BuildOptions from flags
	output := ""
	tags := buildOpts.Tag
	if len(tags) > 0 {
		output = tags[0]
		tags = tags[1:]
	}

	// Pull policy
	pullPolicy := buildahDefine.PullIfMissing
	if cmd.Flags().Changed("pull") && strings.EqualFold(strings.TrimSpace(buildOpts.Pull), "true") {
		pullPolicy = buildahDefine.PullAlways
	}
	if buildOpts.PullAlways || strings.EqualFold(strings.TrimSpace(buildOpts.Pull), "always") {
		pullPolicy = buildahDefine.PullAlways
	}
	if buildOpts.PullNever || strings.EqualFold(strings.TrimSpace(buildOpts.Pull), "never") {
		pullPolicy = buildahDefine.PullNever
	}

	// Build args
	buildArgs := make(map[string]string)
	if cmd.Flag("build-arg").Changed {
		for _, arg := range buildOpts.BuildArg {
			av := strings.SplitN(arg, "=", 2)
			if len(av) > 1 {
				buildArgs[av[0]] = av[1]
			} else {
				if val, present := os.LookupEnv(av[0]); present {
					buildArgs[av[0]] = val
				} else {
					delete(buildArgs, av[0])
				}
			}
		}
	}

	// Handle squash flags
	squash := buildOpts.Squash
	layers := buildOpts.Layers
	if cmd.Flags().Changed("squash") && buildOpts.Squash {
		squash = false
		layers = false
	}
	if cmd.Flags().Changed("squash-all") {
		squash = true
		if !cmd.Flags().Changed("layers") {
			layers = false
		}
	}

	// Parse security options into label and seccomp settings
	var labelOpts []string
	var seccompProfile string
	for _, opt := range buildOpts.SecurityOpt {
		if strings.HasPrefix(opt, "label=") {
			labelOpts = append(labelOpts, strings.TrimPrefix(opt, "label="))
		} else if strings.HasPrefix(opt, "seccomp=") {
			seccompProfile = strings.TrimPrefix(opt, "seccomp=")
		}
	}

	// Handle ignore file
	var excludes []string
	if buildOpts.IgnoreFile != "" {
		var err error
		excludes, err = parseDockerignore(buildOpts.IgnoreFile)
		if err != nil {
			return fmt.Errorf("unable to parse ignore file: %w", err)
		}
	}

	// Handle logfile for client-side logging
	var logfile *os.File
	if buildOpts.Logfile != "" {
		var err error
		logfile, err = os.Create(buildOpts.Logfile)
		if err != nil {
			return fmt.Errorf("failed to create logfile: %w", err)
		}
		defer logfile.Close()
	}

	opts := entities.BuildOptions{
		BuildOptions: buildahDefine.BuildOptions{
			CommonBuildOpts: &buildahDefine.CommonBuildOptions{
				DNSSearch:          buildOpts.DNSSearch,
				DNSServers:         buildOpts.DNSServers,
				DNSOptions:         buildOpts.DNSOptions,
				LabelOpts:          labelOpts,
				SeccompProfilePath: seccompProfile,
				Ulimit:             buildOpts.Ulimit,
			},
			AdditionalTags:          tags,
			Args:                    buildArgs,
			ContextDirectory:        contextDir,
			Excludes:                excludes,
			ForceRmIntermediateCtrs: buildOpts.ForceRm,
			Layers:                  layers,
			NoCache:                 buildOpts.NoCache,
			Out:                     logfile,
			Output:                  output,
			PullPolicy:              pullPolicy,
			Quiet:                   buildOpts.Quiet,
			RemoveIntermediateCtrs:  buildOpts.Rm,
			Squash:                  squash,
			Target:                  buildOpts.Target,
			UnsetEnvs:               buildOpts.UnsetEnvs,
		},
	}

	// Call the engine to perform the build (which will use bindings for remote)
	report, err := registry.ImageEngine().Build(registry.GetContext(), containerFiles, opts)
	if err != nil {
		return err
	}

	if cmd.Flag("quiet").Changed {
		fmt.Println(report.ID)
	}

	return nil
}

// parseDockerignore reads and parses the ignore file
func parseDockerignore(ignoreFile string) ([]string, error) {
	excludes := []string{}
	ignore, err := os.ReadFile(ignoreFile)
	if err != nil {
		return excludes, err
	}
	for _, e := range strings.Split(string(ignore), "\n") {
		if len(e) == 0 || e[0] == '#' {
			continue
		}
		excludes = append(excludes, e)
	}
	return excludes, nil
}
