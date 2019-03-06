package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	buildahcli "github.com/containers/buildah/pkg/cli"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	buildCommand     cliconfig.BuildValues
	buildDescription = "Builds an OCI or Docker image using instructions from one or more Dockerfiles and a specified build context directory."
	layerValues      buildahcli.LayerResults
	budFlagsValues   buildahcli.BudResults
	fromAndBudValues buildahcli.FromAndBudResults
	userNSValues     buildahcli.UserNSResults
	namespaceValues  buildahcli.NameSpaceResults

	_buildCommand = &cobra.Command{
		Use:   "build [flags] CONTEXT",
		Short: "Build an image using instructions from Dockerfiles",
		Long:  buildDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			buildCommand.InputArgs = args
			buildCommand.GlobalFlags = MainGlobalOpts
			buildCommand.BudResults = &budFlagsValues
			buildCommand.UserNSResults = &userNSValues
			buildCommand.FromAndBudResults = &fromAndBudValues
			buildCommand.LayerResults = &layerValues
			buildCommand.NameSpaceResults = &namespaceValues
			return buildCmd(&buildCommand)
		},
		Example: `podman build .
  podman build --cert-dir ~/auth --creds=username:password -t imageName -f Dockerfile.simple .
  podman build --layers --force-rm --tag imageName .`,
	}
)

func init() {
	buildCommand.Command = _buildCommand
	buildCommand.SetHelpTemplate(HelpTemplate())
	buildCommand.SetUsageTemplate(UsageTemplate())
	flags := buildCommand.Flags()
	flags.SetInterspersed(false)

	budFlags := buildahcli.GetBudFlags(&budFlagsValues)
	flag := budFlags.Lookup("pull")
	flag.Value.Set("true")
	flag.DefValue = "true"
	layerFlags := buildahcli.GetLayerFlags(&layerValues)
	flag = layerFlags.Lookup("layers")
	flag.Value.Set(useLayers())
	flag.DefValue = (useLayers())
	flag = layerFlags.Lookup("force-rm")
	flag.Value.Set("true")
	flag.DefValue = "true"

	fromAndBugFlags := buildahcli.GetFromAndBudFlags(&fromAndBudValues, &userNSValues, &namespaceValues)

	flags.AddFlagSet(&budFlags)
	flags.AddFlagSet(&layerFlags)
	flags.AddFlagSet(&fromAndBugFlags)
}

func getDockerfiles(files []string) []string {
	var dockerfiles []string
	for _, f := range files {
		if f == "-" {
			dockerfiles = append(dockerfiles, "/dev/stdin")
		} else {
			dockerfiles = append(dockerfiles, f)
		}
	}
	return dockerfiles
}

func buildCmd(c *cliconfig.BuildValues) error {
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

	dockerfiles := getDockerfiles(c.File)
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
		cliArgs = Tail(cliArgs)
	} else {
		// No context directory or URL was specified.  Try to use the
		// home of the first locally-available Dockerfile.
		for i := range dockerfiles {
			if strings.HasPrefix(dockerfiles[i], "http://") ||
				strings.HasPrefix(dockerfiles[i], "https://") ||
				strings.HasPrefix(dockerfiles[i], "git://") ||
				strings.HasPrefix(dockerfiles[i], "github.com/") {
				continue
			}
			absFile, err := filepath.Abs(dockerfiles[i])
			if err != nil {
				return errors.Wrapf(err, "error determining path to file %q", dockerfiles[i])
			}
			contextDir = filepath.Dir(absFile)
			dockerfiles[i], err = filepath.Rel(contextDir, absFile)
			if err != nil {
				return errors.Wrapf(err, "error determining path to file %q", dockerfiles[i])
			}
			break
		}
	}
	if contextDir == "" {
		return errors.Errorf("no context directory specified, and no dockerfile specified")
	}
	if len(dockerfiles) == 0 {
		dockerfiles = append(dockerfiles, filepath.Join(contextDir, "Dockerfile"))
	}

	runtime, err := adapter.GetRuntime(&c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not get runtime")
	}

	runtimeFlags := []string{}
	for _, arg := range c.RuntimeFlags {
		runtimeFlags = append(runtimeFlags, "--"+arg)
	}
	// end from buildah

	defer runtime.Shutdown(false)

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
		Volumes:      c.Volume,
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
		Target:                  c.Target,
	}
	return runtime.Build(getContext(), c, options, dockerfiles)
}

// Tail returns a string slice after the first element unless there are
// not enough elements, then it returns an empty slice.  This is to replace
// the urfavecli Tail method for args
func Tail(a []string) []string {
	if len(a) >= 2 {
		return []string(a)[1:]
	}
	return []string{}
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
