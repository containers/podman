package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/image/v5/types"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	buildCommand     cliconfig.BuildValues
	buildDescription = "Builds an OCI or Docker image using instructions from one or more Containerfiles and a specified build context directory."
	layerValues      buildahcli.LayerResults
	budFlagsValues   buildahcli.BudResults
	fromAndBudValues buildahcli.FromAndBudResults
	userNSValues     buildahcli.UserNSResults
	namespaceValues  buildahcli.NameSpaceResults
	podBuildValues   cliconfig.PodmanBuildResults

	_buildCommand = &cobra.Command{
		Use:   "build [flags] CONTEXT",
		Short: "Build an image using instructions from Containerfiles",
		Long:  buildDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			buildCommand.InputArgs = args
			buildCommand.GlobalFlags = MainGlobalOpts
			buildCommand.BudResults = &budFlagsValues
			buildCommand.UserNSResults = &userNSValues
			buildCommand.FromAndBudResults = &fromAndBudValues
			buildCommand.LayerResults = &layerValues
			buildCommand.NameSpaceResults = &namespaceValues
			buildCommand.PodmanBuildResults = &podBuildValues
			buildCommand.Remote = remoteclient
			return buildCmd(&buildCommand)
		},
		Example: `podman build .
  podman build --creds=username:password -t imageName -f Containerfile.simple .
  podman build --layers --force-rm --tag imageName .`,
	}
)

func init() {
	buildCommand.Command = _buildCommand
	buildCommand.SetHelpTemplate(HelpTemplate())
	buildCommand.SetUsageTemplate(UsageTemplate())
	flags := buildCommand.Flags()
	flags.SetInterspersed(true)

	budFlags := buildahcli.GetBudFlags(&budFlagsValues)
	flag := budFlags.Lookup("pull")
	if err := flag.Value.Set("true"); err != nil {
		logrus.Error("unable to set pull flag to true")
	}
	flag.DefValue = "true"
	layerFlags := buildahcli.GetLayerFlags(&layerValues)
	flag = layerFlags.Lookup("layers")
	if err := flag.Value.Set(useLayers()); err != nil {
		logrus.Error("unable to set uselayers")
	}
	flag.DefValue = useLayers()
	flag = layerFlags.Lookup("force-rm")
	if err := flag.Value.Set("true"); err != nil {
		logrus.Error("unable to set force-rm flag to true")
	}
	flag.DefValue = "true"
	podmanBuildFlags := GetPodmanBuildFlags(&podBuildValues)
	flag = podmanBuildFlags.Lookup("squash-all")
	if err := flag.Value.Set("false"); err != nil {
		logrus.Error("unable to set squash-all flag to false")
	}

	flag.DefValue = "true"
	fromAndBugFlags := buildahcli.GetFromAndBudFlags(&fromAndBudValues, &userNSValues, &namespaceValues)

	flags.AddFlagSet(&budFlags)
	flags.AddFlagSet(&fromAndBugFlags)
	flags.AddFlagSet(&layerFlags)
	flags.AddFlagSet(&podmanBuildFlags)
	markFlagHidden(flags, "signature-policy")
}

// GetPodmanBuildFlags flags used only by `podman build` and not by
// `buildah bud`.
func GetPodmanBuildFlags(flags *cliconfig.PodmanBuildResults) pflag.FlagSet {
	fs := pflag.FlagSet{}
	fs.BoolVar(&flags.SquashAll, "squash-all", false, "Squash all layers into a single layer.")
	return fs
}

func getContainerfiles(files []string) []string {
	var containerfiles []string
	for _, f := range files {
		if f == "-" {
			containerfiles = append(containerfiles, "/dev/stdin")
		} else {
			containerfiles = append(containerfiles, f)
		}
	}
	return containerfiles
}

func getNsValues(c *cliconfig.BuildValues) ([]buildah.NamespaceOption, error) {
	var ret []buildah.NamespaceOption
	if c.Network != "" {
		if c.Network == "host" {
			ret = append(ret, buildah.NamespaceOption{
				Name: string(specs.NetworkNamespace),
				Host: true,
			})
		} else if c.Network == "container" {
			ret = append(ret, buildah.NamespaceOption{
				Name: string(specs.NetworkNamespace),
			})
		} else if c.Network[0] == '/' {
			ret = append(ret, buildah.NamespaceOption{
				Name: string(specs.NetworkNamespace),
				Path: c.Network,
			})
		} else {
			return nil, fmt.Errorf("unsupported configuration network=%s", c.Network)
		}
	}
	return ret, nil
}

func buildCmd(c *cliconfig.BuildValues) error {
	if (c.Flags().Changed("squash") && c.Flags().Changed("layers")) ||
		(c.Flags().Changed("squash-all") && c.Flags().Changed("layers")) ||
		(c.Flags().Changed("squash-all") && c.Flags().Changed("squash")) {
		return fmt.Errorf("cannot specify squash, squash-all and layers options together")
	}

	// The following was taken directly from containers/buildah/cmd/bud.go
	// TODO Find a away to vendor more of this in rather than copy from bud
	output := ""
	tags := []string{}
	if c.Flag("tag").Changed {
		tags = c.Tag
		if len(tags) > 0 {
			output = tags[0]
			tags = tags[1:]
		}
	}
	if c.BudResults.Authfile != "" {
		if _, err := os.Stat(c.BudResults.Authfile); err != nil {
			return errors.Wrapf(err, "error getting authfile %s", c.BudResults.Authfile)
		}
	}

	pullPolicy := imagebuildah.PullNever
	if c.Pull {
		pullPolicy = imagebuildah.PullIfMissing
	}
	if c.PullAlways {
		pullPolicy = imagebuildah.PullAlways
	}

	args := make(map[string]string)
	if c.Flag("build-arg").Changed {
		for _, arg := range c.BuildArg {
			av := strings.SplitN(arg, "=", 2)
			if len(av) > 1 {
				args[av[0]] = av[1]
			} else {
				delete(args, av[0])
			}
		}
	}

	containerfiles := getContainerfiles(c.File)
	format, err := getFormat(&c.PodmanCommand)
	if err != nil {
		return nil
	}
	contextDir := ""
	cliArgs := c.InputArgs

	layers := c.Layers // layers for podman defaults to true
	// Check to see if the BUILDAH_LAYERS environment variable is set and override command-line
	if _, ok := os.LookupEnv("BUILDAH_LAYERS"); ok {
		layers = buildahcli.UseLayers()
	}

	if len(cliArgs) > 0 {
		// The context directory could be a URL.  Try to handle that.
		tempDir, subDir, err := imagebuildah.TempDirForURL("", "buildah", cliArgs[0])
		if err != nil {
			return errors.Wrapf(err, "error prepping temporary context directory")
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
			absDir, err := filepath.Abs(cliArgs[0])
			if err != nil {
				return errors.Wrapf(err, "error determining path to directory %q", cliArgs[0])
			}
			contextDir = absDir
		}
	} else {
		// No context directory or URL was specified.  Try to use the
		// home of the first locally-available Containerfile.
		for i := range containerfiles {
			if strings.HasPrefix(containerfiles[i], "http://") ||
				strings.HasPrefix(containerfiles[i], "https://") ||
				strings.HasPrefix(containerfiles[i], "git://") ||
				strings.HasPrefix(containerfiles[i], "github.com/") {
				continue
			}
			absFile, err := filepath.Abs(containerfiles[i])
			if err != nil {
				return errors.Wrapf(err, "error determining path to file %q", containerfiles[i])
			}
			contextDir = filepath.Dir(absFile)
			containerfiles[i], err = filepath.Rel(contextDir, absFile)
			if err != nil {
				return errors.Wrapf(err, "error determining path to file %q", containerfiles[i])
			}
			break
		}
	}
	if contextDir == "" {
		return errors.Errorf("no context directory specified, and no containerfile specified")
	}
	if !fileIsDir(contextDir) {
		return errors.Errorf("context must be a directory: %v", contextDir)
	}
	if len(containerfiles) == 0 {
		if checkIfFileExists(filepath.Join(contextDir, "Containerfile")) {
			containerfiles = append(containerfiles, filepath.Join(contextDir, "Containerfile"))
		} else {
			containerfiles = append(containerfiles, filepath.Join(contextDir, "Dockerfile"))
		}
	}

	runtime, err := adapter.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}

	runtimeFlags := []string{}
	for _, arg := range c.RuntimeFlags {
		runtimeFlags = append(runtimeFlags, "--"+arg)
	}

	conf, err := runtime.GetConfig()
	if err != nil {
		return err
	}
	if conf != nil && conf.CgroupManager == define.SystemdCgroupsManager {
		runtimeFlags = append(runtimeFlags, "--systemd-cgroup")
	}
	// end from buildah

	defer runtime.DeferredShutdown(false)

	var stdout, stderr, reporter *os.File
	stdout = os.Stdout
	stderr = os.Stderr
	reporter = os.Stderr
	if c.Flag("logfile").Changed {
		f, err := os.OpenFile(c.Logfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err != nil {
			return errors.Errorf("error opening logfile %q: %v", c.Logfile, err)
		}
		defer f.Close()
		logrus.SetOutput(f)
		stdout = f
		stderr = f
		reporter = f
	}

	var memoryLimit, memorySwap int64
	if c.Flags().Changed("memory") {
		memoryLimit, err = units.RAMInBytes(c.Memory)
		if err != nil {
			return err
		}
	}

	if c.Flags().Changed("memory-swap") {
		memorySwap, err = units.RAMInBytes(c.MemorySwap)
		if err != nil {
			return err
		}
	}

	nsValues, err := getNsValues(c)
	if err != nil {
		return err
	}

	buildOpts := buildah.CommonBuildOptions{
		AddHost:      c.AddHost,
		CgroupParent: c.CgroupParent,
		CPUPeriod:    c.CPUPeriod,
		CPUQuota:     c.CPUQuota,
		CPUShares:    c.CPUShares,
		CPUSetCPUs:   c.CPUSetCPUs,
		CPUSetMems:   c.CPUSetMems,
		Memory:       memoryLimit,
		MemorySwap:   memorySwap,
		ShmSize:      c.ShmSize,
		Ulimit:       c.Ulimit,
		Volumes:      c.Volumes,
	}

	// `buildah bud --layers=false` acts like `docker build --squash` does.
	// That is all of the new layers created during the build process are
	// condensed into one, any layers present prior to this build are retained
	// without condensing.  `buildah bud --squash` squashes both new and old
	// layers down into one.  Translate Podman commands into Buildah.
	// Squash invoked, retain old layers, squash new layers into one.
	if c.Flags().Changed("squash") && c.Squash {
		c.Squash = false
		layers = false
	}
	// Squash-all invoked, squash both new and old layers into one.
	if c.Flags().Changed("squash-all") {
		c.Squash = true
		layers = false
	}

	options := imagebuildah.BuildOptions{
		CommonBuildOpts:         &buildOpts,
		AdditionalTags:          tags,
		Annotations:             c.Annotation,
		Args:                    args,
		CNIConfigDir:            c.CNIConfigDir,
		CNIPluginPath:           c.CNIPlugInPath,
		Compression:             imagebuildah.Gzip,
		ContextDirectory:        contextDir,
		DefaultMountsFilePath:   c.GlobalFlags.DefaultMountsFile,
		Err:                     stderr,
		ForceRmIntermediateCtrs: c.ForceRm,
		IIDFile:                 c.Iidfile,
		Labels:                  c.Label,
		Layers:                  layers,
		NamespaceOptions:        nsValues,
		NoCache:                 c.NoCache,
		Out:                     stdout,
		Output:                  output,
		OutputFormat:            format,
		PullPolicy:              pullPolicy,
		Quiet:                   c.Quiet,
		RemoveIntermediateCtrs:  c.Rm,
		ReportWriter:            reporter,
		RuntimeArgs:             runtimeFlags,
		SignaturePolicyPath:     c.SignaturePolicy,
		Squash:                  c.Squash,
		SystemContext: &types.SystemContext{
			OSChoice:           c.OverrideOS,
			ArchitectureChoice: c.OverrideArch,
		},
		Target: c.Target,
	}
	return runtime.Build(getContext(), c, options, containerfiles)
}

// useLayers returns false if BUILDAH_LAYERS is set to "0" or "false"
// otherwise it returns true
func useLayers() string {
	layers := os.Getenv("BUILDAH_LAYERS")
	if strings.ToLower(layers) == "false" || layers == "0" {
		return "false"
	}
	return "true"
}
