package images

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	buildahCLI "github.com/containers/buildah/pkg/cli"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/libpod/cmd/podman/registry"
	"github.com/containers/libpod/cmd/podman/utils"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// buildFlagsWrapper are local to cmd/ as the build code is using Buildah-internal
// types.  Hence, after parsing, we are converting buildFlagsWrapper to the entities'
// options which essentially embed the Buildah types.
type buildFlagsWrapper struct {
	// Buildah stuff first
	buildahCLI.BudResults
	buildahCLI.LayerResults
	buildahCLI.FromAndBudResults
	buildahCLI.NameSpaceResults
	buildahCLI.UserNSResults

	// SquashAll squashes all layers into a single layer.
	SquashAll bool
}

var (
	// Command: podman _diff_ Object_ID
	buildDescription = "Builds an OCI or Docker image using instructions from one or more Containerfiles and a specified build context directory."
	buildCmd         = &cobra.Command{
		Use:              "build [flags] [CONTEXT]",
		Short:            "Build an image using instructions from Containerfiles",
		Long:             buildDescription,
		TraverseChildren: true,
		RunE:             build,
		Example: `podman build .
  podman build --creds=username:password -t imageName -f Containerfile.simple .
  podman build --layers --force-rm --tag imageName .`,
	}

	imageBuildCmd = &cobra.Command{
		Args:  buildCmd.Args,
		Use:   buildCmd.Use,
		Short: buildCmd.Short,
		Long:  buildCmd.Long,
		RunE:  buildCmd.RunE,
		Example: `podman image build .
  podman image build --creds=username:password -t imageName -f Containerfile.simple .
  podman image build --layers --force-rm --tag imageName .`,
	}

	buildOpts = buildFlagsWrapper{}
)

// useLayers returns false if BUILDAH_LAYERS is set to "0" or "false"
// otherwise it returns true
func useLayers() string {
	layers := os.Getenv("BUILDAH_LAYERS")
	if strings.ToLower(layers) == "false" || layers == "0" {
		return "false"
	}
	return "true"
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: buildCmd,
	})
	buildFlags(buildCmd.Flags())

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: imageBuildCmd,
		Parent:  imageCmd,
	})
	buildFlags(imageBuildCmd.Flags())
}

func buildFlags(flags *pflag.FlagSet) {
	// Podman flags
	flags.BoolVarP(&buildOpts.SquashAll, "squash-all", "", false, "Squash all layers into a single layer")

	// Bud flags
	budFlags := buildahCLI.GetBudFlags(&buildOpts.BudResults)
	// --pull flag
	flag := budFlags.Lookup("pull")
	if err := flag.Value.Set("true"); err != nil {
		logrus.Errorf("unable to set --pull to true: %v", err)
	}
	flag.DefValue = "true"
	flags.AddFlagSet(&budFlags)

	// Layer flags
	layerFlags := buildahCLI.GetLayerFlags(&buildOpts.LayerResults)
	// --layers flag
	flag = layerFlags.Lookup("layers")
	useLayersVal := useLayers()
	if err := flag.Value.Set(useLayersVal); err != nil {
		logrus.Errorf("unable to set --layers to %v: %v", useLayersVal, err)
	}
	flag.DefValue = useLayersVal
	// --force-rm flag
	flag = layerFlags.Lookup("force-rm")
	if err := flag.Value.Set("true"); err != nil {
		logrus.Errorf("unable to set --force-rm to true: %v", err)
	}
	flag.DefValue = "true"
	flags.AddFlagSet(&layerFlags)

	// FromAndBud flags
	fromAndBudFlags, err := buildahCLI.GetFromAndBudFlags(&buildOpts.FromAndBudResults, &buildOpts.UserNSResults, &buildOpts.NameSpaceResults)
	if err != nil {
		logrus.Errorf("error setting up build flags: %v", err)
		os.Exit(1)
	}
	flags.AddFlagSet(&fromAndBudFlags)
	_ = flags.MarkHidden("signature-policy")
}

// build executes the build command.
func build(cmd *cobra.Command, args []string) error {
	if (cmd.Flags().Changed("squash") && cmd.Flags().Changed("layers")) ||
		(cmd.Flags().Changed("squash-all") && cmd.Flags().Changed("layers")) ||
		(cmd.Flags().Changed("squash-all") && cmd.Flags().Changed("squash")) {
		return errors.New("cannot specify --squash, --squash-all and --layers options together")
	}

	contextDir, containerFiles, err := extractContextAndFiles(args, buildOpts.File)
	if err != nil {
		return err
	}

	ie, err := registry.NewImageEngine(cmd, args)
	if err != nil {
		return err
	}

	apiBuildOpts, err := buildFlagsWrapperToOptions(cmd, contextDir, &buildOpts)
	if err != nil {
		return err
	}

	_, err = ie.Build(registry.GetContext(), containerFiles, *apiBuildOpts)
	return err
}

// extractContextAndFiles parses args and files to extract a context directory
// and {Container,Docker}files.
//
// TODO: this was copied and altered from the v1 client which in turn was
// copied and altered from the Buildah code. Ideally, all of this code should
// be cleanly consolidated into a package that is shared between Buildah and
// Podman.
func extractContextAndFiles(args, files []string) (string, []string, error) {
	// Extract container files from the CLI (i.e., --file/-f) first.
	var containerFiles []string
	for _, f := range files {
		if f == "-" {
			containerFiles = append(containerFiles, "/dev/stdin")
		} else {
			containerFiles = append(containerFiles, f)
		}
	}

	// Determine context directory.
	var contextDir string
	if len(args) > 0 {
		// The context directory could be a URL.  Try to handle that.
		tempDir, subDir, err := imagebuildah.TempDirForURL("", "buildah", args[0])
		if err != nil {
			return "", nil, errors.Wrapf(err, "error prepping temporary context directory")
		}
		if tempDir != "" {
			// We had to download it to a temporary directory.
			// Delete it later.
			defer func() {
				if err = os.RemoveAll(tempDir); err != nil {
					logrus.Errorf("error removing temporary directory %q: %v", contextDir, err)
				}
			}()
			contextDir = filepath.Join(tempDir, subDir)
		} else {
			// Nope, it was local.  Use it as is.
			absDir, err := filepath.Abs(args[0])
			if err != nil {
				return "", nil, errors.Wrapf(err, "error determining path to directory %q", args[0])
			}
			contextDir = absDir
		}
	} else {
		// No context directory or URL was specified.  Try to use the home of
		// the first locally-available Containerfile.
		for i := range containerFiles {
			if strings.HasPrefix(containerFiles[i], "http://") ||
				strings.HasPrefix(containerFiles[i], "https://") ||
				strings.HasPrefix(containerFiles[i], "git://") ||
				strings.HasPrefix(containerFiles[i], "github.com/") {
				continue
			}
			absFile, err := filepath.Abs(containerFiles[i])
			if err != nil {
				return "", nil, errors.Wrapf(err, "error determining path to file %q", containerFiles[i])
			}
			contextDir = filepath.Dir(absFile)
			break
		}
	}

	if contextDir == "" {
		return "", nil, errors.Errorf("no context directory and no Containerfile specified")
	}
	if !utils.IsDir(contextDir) {
		return "", nil, errors.Errorf("context must be a directory: %q", contextDir)
	}
	if len(containerFiles) == 0 {
		if utils.FileExists(filepath.Join(contextDir, "Containerfile")) {
			containerFiles = append(containerFiles, filepath.Join(contextDir, "Containerfile"))
		} else {
			containerFiles = append(containerFiles, filepath.Join(contextDir, "Dockerfile"))
		}
	}

	return contextDir, containerFiles, nil
}

// buildFlagsWrapperToOptions converts the local build flags to the build options used
// in the API which embed Buildah types used across the build code.  Doing the
// conversion here prevents the API from doing that (redundantly).
//
// TODO: this code should really be in Buildah.
func buildFlagsWrapperToOptions(c *cobra.Command, contextDir string, flags *buildFlagsWrapper) (*entities.BuildOptions, error) {
	output := ""
	tags := []string{}
	if c.Flag("tag").Changed {
		tags = flags.Tag
		if len(tags) > 0 {
			output = tags[0]
			tags = tags[1:]
		}
	}

	pullPolicy := imagebuildah.PullNever
	if flags.Pull {
		pullPolicy = imagebuildah.PullIfMissing
	}
	if flags.PullAlways {
		pullPolicy = imagebuildah.PullAlways
	}

	args := make(map[string]string)
	if c.Flag("build-arg").Changed {
		for _, arg := range flags.BuildArg {
			av := strings.SplitN(arg, "=", 2)
			if len(av) > 1 {
				args[av[0]] = av[1]
			} else {
				delete(args, av[0])
			}
		}
	}
	// Check to see if the BUILDAH_LAYERS environment variable is set and
	// override command-line.
	if _, ok := os.LookupEnv("BUILDAH_LAYERS"); ok {
		flags.Layers = true
	}

	// `buildah bud --layers=false` acts like `docker build --squash` does.
	// That is all of the new layers created during the build process are
	// condensed into one, any layers present prior to this build are
	// retained without condensing.  `buildah bud --squash` squashes both
	// new and old layers down into one.  Translate Podman commands into
	// Buildah.  Squash invoked, retain old layers, squash new layers into
	// one.
	if c.Flags().Changed("squash") && buildOpts.Squash {
		flags.Squash = false
		flags.Layers = false
	}
	// Squash-all invoked, squash both new and old layers into one.
	if c.Flags().Changed("squash-all") {
		flags.Squash = true
		flags.Layers = false
	}

	var stdin, stdout, stderr, reporter *os.File
	stdin = os.Stdin
	stdout = os.Stdout
	stderr = os.Stderr
	reporter = os.Stderr

	if c.Flag("logfile").Changed {
		f, err := os.OpenFile(flags.Logfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return nil, errors.Errorf("error opening logfile %q: %v", flags.Logfile, err)
		}
		defer f.Close()
		logrus.SetOutput(f)
		stdout = f
		stderr = f
		reporter = f
	}

	var memoryLimit, memorySwap int64
	var err error
	if c.Flags().Changed("memory") {
		memoryLimit, err = units.RAMInBytes(flags.Memory)
		if err != nil {
			return nil, err
		}
	}

	if c.Flags().Changed("memory-swap") {
		memorySwap, err = units.RAMInBytes(flags.MemorySwap)
		if err != nil {
			return nil, err
		}
	}

	nsValues, err := getNsValues(flags)
	if err != nil {
		return nil, err
	}

	networkPolicy := buildah.NetworkDefault
	for _, ns := range nsValues {
		if ns.Name == "none" {
			networkPolicy = buildah.NetworkDisabled
			break
		} else if !filepath.IsAbs(ns.Path) {
			networkPolicy = buildah.NetworkEnabled
			break
		}
	}

	// `buildah bud --layers=false` acts like `docker build --squash` does.
	// That is all of the new layers created during the build process are
	// condensed into one, any layers present prior to this build are retained
	// without condensing.  `buildah bud --squash` squashes both new and old
	// layers down into one.  Translate Podman commands into Buildah.
	// Squash invoked, retain old layers, squash new layers into one.
	if c.Flags().Changed("squash") && flags.Squash {
		flags.Squash = false
		flags.Layers = false
	}
	// Squash-all invoked, squash both new and old layers into one.
	if c.Flags().Changed("squash-all") {
		flags.Squash = true
		flags.Layers = false
	}

	compression := imagebuildah.Gzip
	if flags.DisableCompression {
		compression = imagebuildah.Uncompressed
	}

	isolation, err := parse.IsolationOption(flags.Isolation)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing ID mapping options")
	}

	usernsOption, idmappingOptions, err := parse.IDMappingOptions(c, isolation)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing ID mapping options")
	}
	nsValues = append(nsValues, usernsOption...)

	systemContext, err := parse.SystemContextFromOptions(c)
	if err != nil {
		return nil, errors.Wrapf(err, "error building system context")
	}

	format := ""
	flags.Format = strings.ToLower(flags.Format)
	switch {
	case strings.HasPrefix(flags.Format, buildah.OCI):
		format = buildah.OCIv1ImageManifest
	case strings.HasPrefix(flags.Format, buildah.DOCKER):
		format = buildah.Dockerv2ImageManifest
	default:
		return nil, errors.Errorf("unrecognized image type %q", flags.Format)
	}

	runtimeFlags := []string{}
	for _, arg := range flags.RuntimeFlags {
		runtimeFlags = append(runtimeFlags, "--"+arg)
	}

	// FIXME: the code below needs to be enabled (and adjusted) once the
	// global/root flags are supported.

	//	conf, err := runtime.GetConfig()
	//	if err != nil {
	//		return err
	//	}
	//	if conf != nil && conf.Engine.CgroupManager == config.SystemdCgroupsManager {
	//		runtimeFlags = append(runtimeFlags, "--systemd-cgroup")
	//	}

	opts := imagebuildah.BuildOptions{
		AddCapabilities: flags.CapAdd,
		AdditionalTags:  tags,
		Annotations:     flags.Annotation,
		Architecture:    flags.Arch,
		Args:            args,
		BlobDirectory:   flags.BlobCache,
		CNIConfigDir:    flags.CNIConfigDir,
		CNIPluginPath:   flags.CNIPlugInPath,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			AddHost:      flags.AddHost,
			CgroupParent: flags.CgroupParent,
			CPUPeriod:    flags.CPUPeriod,
			CPUQuota:     flags.CPUQuota,
			CPUShares:    flags.CPUShares,
			CPUSetCPUs:   flags.CPUSetCPUs,
			CPUSetMems:   flags.CPUSetMems,
			Memory:       memoryLimit,
			MemorySwap:   memorySwap,
			ShmSize:      flags.ShmSize,
			Ulimit:       flags.Ulimit,
			Volumes:      flags.Volumes,
		},
		Compression:      compression,
		ConfigureNetwork: networkPolicy,
		ContextDirectory: contextDir,
		//		DefaultMountsFilePath:   FIXME: this requires global flags to be working!
		Devices:                 flags.Devices,
		DropCapabilities:        flags.CapDrop,
		Err:                     stderr,
		ForceRmIntermediateCtrs: flags.ForceRm,
		IDMappingOptions:        idmappingOptions,
		IIDFile:                 flags.Iidfile,
		In:                      stdin,
		Isolation:               isolation,
		Labels:                  flags.Label,
		Layers:                  flags.Layers,
		NamespaceOptions:        nsValues,
		NoCache:                 flags.NoCache,
		OS:                      flags.OS,
		Out:                     stdout,
		Output:                  output,
		OutputFormat:            format,
		PullPolicy:              pullPolicy,
		Quiet:                   flags.Quiet,
		RemoveIntermediateCtrs:  flags.Rm,
		ReportWriter:            reporter,
		RuntimeArgs:             runtimeFlags,
		SignBy:                  flags.SignBy,
		SignaturePolicyPath:     flags.SignaturePolicy,
		Squash:                  flags.Squash,
		SystemContext:           systemContext,
		Target:                  flags.Target,
		TransientMounts:         flags.Volumes,
	}

	return &entities.BuildOptions{BuildOptions: opts}, nil
}

func getNsValues(flags *buildFlagsWrapper) ([]buildah.NamespaceOption, error) {
	var ret []buildah.NamespaceOption
	if flags.Network != "" {
		switch {
		case flags.Network == "host":
			ret = append(ret, buildah.NamespaceOption{
				Name: string(specs.NetworkNamespace),
				Host: true,
			})
		case flags.Network == "container":
			ret = append(ret, buildah.NamespaceOption{
				Name: string(specs.NetworkNamespace),
			})
		case flags.Network[0] == '/':
			ret = append(ret, buildah.NamespaceOption{
				Name: string(specs.NetworkNamespace),
				Path: flags.Network,
			})
		default:
			return nil, errors.Errorf("unsupported configuration network=%s", flags.Network)
		}
	}
	return ret, nil
}
