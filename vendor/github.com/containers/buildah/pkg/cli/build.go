package cli

// the cli package contains spf13/cobra related structs that help make up
// the command line for buildah commands. this file's contents are better
// suited for pkg/parse, but since pkg/parse imports pkg/util which also
// imports pkg/parse, having it there would create a cyclic dependency, so
// here we are.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/pkg/util"
	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type BuildOptions struct {
	*LayerResults
	*BudResults
	*UserNSResults
	*FromAndBudResults
	*NameSpaceResults
	Logwriter *os.File
}

const (
	MaxPullPushRetries = 3
	PullPushRetryDelay = 2 * time.Second
)

// GenBuildOptions translates command line flags into a BuildOptions structure
func GenBuildOptions(c *cobra.Command, inputArgs []string, iopts BuildOptions) (define.BuildOptions, []string, []string, error) {
	options := define.BuildOptions{}

	var removeAll []string

	output := ""
	cleanTmpFile := false
	tags := []string{}
	if iopts.Network == "none" {
		if c.Flag("dns").Changed {
			return options, nil, nil, errors.New("the --dns option cannot be used with --network=none")
		}
		if c.Flag("dns-option").Changed {
			return options, nil, nil, errors.New("the --dns-option option cannot be used with --network=none")
		}
		if c.Flag("dns-search").Changed {
			return options, nil, nil, errors.New("the --dns-search option cannot be used with --network=none")
		}
	}
	if c.Flag("tag").Changed {
		tags = iopts.Tag
		if len(tags) > 0 {
			output = tags[0]
			tags = tags[1:]
		}
		if c.Flag("manifest").Changed {
			for _, tag := range tags {
				if tag == iopts.Manifest {
					return options, nil, nil, errors.New("the same name must not be specified for both '--tag' and '--manifest'")
				}
			}
		}
	}
	if err := auth.CheckAuthFile(iopts.BudResults.Authfile); err != nil {
		return options, nil, nil, err
	}

	if c.Flag("logsplit").Changed {
		if !c.Flag("logfile").Changed {
			return options, nil, nil, errors.New("cannot use --logsplit without --logfile")
		}
	}

	iopts.BudResults.Authfile, cleanTmpFile = util.MirrorToTempFileIfPathIsDescriptor(iopts.BudResults.Authfile)
	if cleanTmpFile {
		removeAll = append(removeAll, iopts.BudResults.Authfile)
	}

	pullPolicy, err := parse.PullPolicyFromOptions(c)
	if err != nil {
		return options, nil, nil, err
	}

	args := make(map[string]string)
	if c.Flag("build-arg-file").Changed {
		for _, argfile := range iopts.BuildArgFile {
			if err := readBuildArgFile(argfile, args); err != nil {
				return options, nil, nil, err
			}
		}
	}
	if c.Flag("build-arg").Changed {
		for _, arg := range iopts.BuildArg {
			readBuildArg(arg, args)
		}
	}

	additionalBuildContext := make(map[string]*define.AdditionalBuildContext)
	if c.Flag("build-context").Changed {
		for _, contextString := range iopts.BuildContext {
			av := strings.SplitN(contextString, "=", 2)
			if len(av) > 1 {
				parseAdditionalBuildContext, err := parse.GetAdditionalBuildContext(av[1])
				if err != nil {
					return options, nil, nil, fmt.Errorf("while parsing additional build context: %w", err)
				}
				additionalBuildContext[av[0]] = &parseAdditionalBuildContext
			} else {
				return options, nil, nil, fmt.Errorf("while parsing additional build context: %q, accepts value in the form of key=value", av)
			}
		}
	}

	containerfiles := getContainerfiles(iopts.File)
	format, err := GetFormat(iopts.Format)
	if err != nil {
		return options, nil, nil, err
	}
	layers := UseLayers()
	if c.Flag("layers").Changed {
		layers = iopts.Layers
	}
	contextDir := ""
	cliArgs := inputArgs

	// Nothing provided, we assume the current working directory as build
	// context
	if len(cliArgs) == 0 {
		contextDir, err = os.Getwd()
		if err != nil {
			return options, nil, nil, fmt.Errorf("unable to choose current working directory as build context: %w", err)
		}
	} else {
		// The context directory could be a URL.  Try to handle that.
		tempDir, subDir, err := define.TempDirForURL("", "buildah", cliArgs[0])
		if err != nil {
			return options, nil, nil, fmt.Errorf("prepping temporary context directory: %w", err)
		}
		if tempDir != "" {
			// We had to download it to a temporary directory.
			// Delete it later.
			removeAll = append(removeAll, tempDir)
			contextDir = filepath.Join(tempDir, subDir)
		} else {
			// Nope, it was local.  Use it as is.
			absDir, err := filepath.Abs(cliArgs[0])
			if err != nil {
				return options, nil, nil, fmt.Errorf("determining path to directory: %w", err)
			}
			contextDir = absDir
		}
	}

	if len(containerfiles) == 0 {
		// Try to find the Containerfile/Dockerfile within the contextDir
		containerfile, err := util.DiscoverContainerfile(contextDir)
		if err != nil {
			return options, nil, nil, err
		}
		containerfiles = append(containerfiles, containerfile)
		contextDir = filepath.Dir(containerfile)
	}

	contextDir, err = filepath.EvalSymlinks(contextDir)
	if err != nil {
		return options, nil, nil, fmt.Errorf("evaluating symlinks in build context path: %w", err)
	}

	var stdin io.Reader
	if iopts.Stdin {
		stdin = os.Stdin
	}

	var stdout, stderr, reporter *os.File
	stdout = os.Stdout
	stderr = os.Stderr
	reporter = os.Stderr
	if iopts.Logwriter != nil {
		logrus.SetOutput(iopts.Logwriter)
		stdout = iopts.Logwriter
		stderr = iopts.Logwriter
		reporter = iopts.Logwriter
	}

	systemContext, err := parse.SystemContextFromOptions(c)
	if err != nil {
		return options, nil, nil, fmt.Errorf("building system context: %w", err)
	}

	isolation, err := parse.IsolationOption(iopts.Isolation)
	if err != nil {
		return options, nil, nil, err
	}

	runtimeFlags := []string{}
	for _, arg := range iopts.RuntimeFlags {
		runtimeFlags = append(runtimeFlags, "--"+arg)
	}

	commonOpts, err := parse.CommonBuildOptions(c)
	if err != nil {
		return options, nil, nil, err
	}

	if (c.Flag("rm").Changed || c.Flag("force-rm").Changed) && (!c.Flag("layers").Changed && !c.Flag("no-cache").Changed) {
		return options, nil, nil, errors.New("'rm' and 'force-rm' can only be set with either 'layers' or 'no-cache'")
	}

	if c.Flag("compress").Changed {
		logrus.Debugf("--compress option specified but is ignored")
	}

	compression := define.Gzip
	if iopts.DisableCompression {
		compression = define.Uncompressed
	}

	if c.Flag("disable-content-trust").Changed {
		logrus.Debugf("--disable-content-trust option specified but is ignored")
	}

	namespaceOptions, networkPolicy, err := parse.NamespaceOptions(c)
	if err != nil {
		return options, nil, nil, err
	}
	usernsOption, idmappingOptions, err := parse.IDMappingOptions(c, isolation)
	if err != nil {
		return options, nil, nil, fmt.Errorf("parsing ID mapping options: %w", err)
	}
	namespaceOptions.AddOrReplace(usernsOption...)

	platforms, err := parse.PlatformsFromOptions(c)
	if err != nil {
		return options, nil, nil, err
	}

	decryptConfig, err := DecryptConfig(iopts.DecryptionKeys)
	if err != nil {
		return options, nil, nil, fmt.Errorf("unable to obtain decrypt config: %w", err)
	}

	var excludes []string
	if iopts.IgnoreFile != "" {
		if excludes, _, err = parse.ContainerIgnoreFile(contextDir, iopts.IgnoreFile, containerfiles); err != nil {
			return options, nil, nil, err
		}
	}
	var timestamp *time.Time
	if c.Flag("timestamp").Changed {
		t := time.Unix(iopts.Timestamp, 0).UTC()
		timestamp = &t
	}
	if c.Flag("output").Changed {
		buildOption, err := parse.GetBuildOutput(iopts.BuildOutput)
		if err != nil {
			return options, nil, nil, err
		}
		if buildOption.IsStdout {
			iopts.Quiet = true
		}
	}
	var confidentialWorkloadOptions define.ConfidentialWorkloadOptions
	if c.Flag("cw").Changed {
		confidentialWorkloadOptions, err = parse.GetConfidentialWorkloadOptions(iopts.CWOptions)
		if err != nil {
			return options, nil, nil, err
		}
	}
	var cacheTo []reference.Named
	var cacheFrom []reference.Named
	cacheTo = nil
	cacheFrom = nil
	if c.Flag("cache-to").Changed {
		cacheTo, err = parse.RepoNamesToNamedReferences(iopts.CacheTo)
		if err != nil {
			return options, nil, nil, fmt.Errorf("unable to parse value provided `%s` to --cache-to: %w", iopts.CacheTo, err)
		}
	}
	if c.Flag("cache-from").Changed {
		cacheFrom, err = parse.RepoNamesToNamedReferences(iopts.CacheFrom)
		if err != nil {
			return options, nil, nil, fmt.Errorf("unable to parse value provided `%s` to --cache-from: %w", iopts.CacheTo, err)
		}
	}
	var cacheTTL time.Duration
	if c.Flag("cache-ttl").Changed {
		cacheTTL, err = time.ParseDuration(iopts.CacheTTL)
		if err != nil {
			return options, nil, nil, fmt.Errorf("unable to parse value provided %q as --cache-ttl: %w", iopts.CacheTTL, err)
		}
		// If user explicitly specified `--cache-ttl=0s`
		// it would effectively mean that user is asking
		// to use no cache at all. In such use cases
		// buildah can skip looking for cache entirely
		// by setting `--no-cache=true` internally.
		if int64(cacheTTL) == 0 {
			logrus.Debug("Setting --no-cache=true since --cache-ttl was set to 0s which effectively means user wants to ignore cache")
			if c.Flag("no-cache").Changed && !iopts.NoCache {
				return options, nil, nil, fmt.Errorf("cannot use --cache-ttl with duration as 0 and --no-cache=false")
			}
			iopts.NoCache = true
		}
	}

	if c.Flag("network").Changed && c.Flag("isolation").Changed {
		if isolation == define.IsolationChroot {
			if ns := namespaceOptions.Find(string(specs.NetworkNamespace)); ns != nil {
				if !ns.Host {
					return options, nil, nil, fmt.Errorf("cannot set --network other than host with --isolation %s", c.Flag("isolation").Value.String())
				}
			}
		}
	}

	var sbomScanOptions []define.SBOMScanOptions
	if c.Flag("sbom").Changed || c.Flag("sbom-scanner-command").Changed || c.Flag("sbom-scanner-image").Changed || c.Flag("sbom-image-output").Changed || c.Flag("sbom-merge-strategy").Changed || c.Flag("sbom-output").Changed || c.Flag("sbom-image-output").Changed || c.Flag("sbom-purl-output").Changed || c.Flag("sbom-image-purl-output").Changed {
		sbomScanOption, err := parse.SBOMScanOptions(c)
		if err != nil {
			return options, nil, nil, err
		}
		if !slices.Contains(sbomScanOption.ContextDir, contextDir) {
			sbomScanOption.ContextDir = append(sbomScanOption.ContextDir, contextDir)
		}
		for _, abc := range additionalBuildContext {
			if !abc.IsURL && !abc.IsImage {
				sbomScanOption.ContextDir = append(sbomScanOption.ContextDir, abc.Value)
			}
		}
		sbomScanOption.PullPolicy = pullPolicy
		sbomScanOptions = append(sbomScanOptions, *sbomScanOption)
	}

	options = define.BuildOptions{
		AddCapabilities:         iopts.CapAdd,
		AdditionalBuildContexts: additionalBuildContext,
		AdditionalTags:          tags,
		AllPlatforms:            iopts.AllPlatforms,
		Annotations:             iopts.Annotation,
		Architecture:            systemContext.ArchitectureChoice,
		Args:                    args,
		BlobDirectory:           iopts.BlobCache,
		BuildOutput:             iopts.BuildOutput,
		CacheFrom:               cacheFrom,
		CacheTo:                 cacheTo,
		CacheTTL:                cacheTTL,
		CDIConfigDir:            iopts.CDIConfigDir,
		CNIConfigDir:            iopts.CNIConfigDir,
		CNIPluginPath:           iopts.CNIPlugInPath,
		CompatVolumes:           types.NewOptionalBool(iopts.CompatVolumes),
		ConfidentialWorkload:    confidentialWorkloadOptions,
		CPPFlags:                iopts.CPPFlags,
		CommonBuildOpts:         commonOpts,
		Compression:             compression,
		ConfigureNetwork:        networkPolicy,
		ContextDirectory:        contextDir,
		Devices:                 iopts.Devices,
		DropCapabilities:        iopts.CapDrop,
		Err:                     stderr,
		Excludes:                excludes,
		ForceRmIntermediateCtrs: iopts.ForceRm,
		From:                    iopts.From,
		GroupAdd:                iopts.GroupAdd,
		IDMappingOptions:        idmappingOptions,
		IIDFile:                 iopts.Iidfile,
		IgnoreFile:              iopts.IgnoreFile,
		In:                      stdin,
		Isolation:               isolation,
		Jobs:                    &iopts.Jobs,
		Labels:                  iopts.Label,
		LayerLabels:             iopts.LayerLabel,
		Layers:                  layers,
		LogFile:                 iopts.Logfile,
		LogRusage:               iopts.LogRusage,
		LogSplitByPlatform:      iopts.LogSplitByPlatform,
		Manifest:                iopts.Manifest,
		MaxPullPushRetries:      iopts.Retry,
		NamespaceOptions:        namespaceOptions,
		NoCache:                 iopts.NoCache,
		OS:                      systemContext.OSChoice,
		OSFeatures:              iopts.OSFeatures,
		OSVersion:               iopts.OSVersion,
		OciDecryptConfig:        decryptConfig,
		Out:                     stdout,
		Output:                  output,
		OutputFormat:            format,
		Platforms:               platforms,
		PullPolicy:              pullPolicy,
		Quiet:                   iopts.Quiet,
		RemoveIntermediateCtrs:  iopts.Rm,
		ReportWriter:            reporter,
		Runtime:                 iopts.Runtime,
		RuntimeArgs:             runtimeFlags,
		RusageLogFile:           iopts.RusageLogFile,
		SBOMScanOptions:         sbomScanOptions,
		SignBy:                  iopts.SignBy,
		SignaturePolicyPath:     iopts.SignaturePolicy,
		SkipUnusedStages:        types.NewOptionalBool(iopts.SkipUnusedStages),
		Squash:                  iopts.Squash,
		SystemContext:           systemContext,
		Target:                  iopts.Target,
		Timestamp:               timestamp,
		TransientMounts:         iopts.Volumes,
		UnsetEnvs:               iopts.UnsetEnvs,
		UnsetLabels:             iopts.UnsetLabels,
	}
	if iopts.RetryDelay != "" {
		options.PullPushRetryDelay, err = time.ParseDuration(iopts.RetryDelay)
		if err != nil {
			return options, nil, nil, fmt.Errorf("unable to parse value provided %q as --retry-delay: %w", iopts.RetryDelay, err)
		}
		// Following log line is used in integration test.
		logrus.Debugf("Setting MaxPullPushRetries to %d and PullPushRetryDelay to %v", iopts.Retry, options.PullPushRetryDelay)
	}

	if iopts.Quiet {
		options.ReportWriter = io.Discard
	}

	options.Envs = LookupEnvVarReferences(iopts.Envs, os.Environ())

	return options, containerfiles, removeAll, nil
}

func readBuildArgFile(buildargfile string, args map[string]string) error {
	argfile, err := os.ReadFile(buildargfile)
	if err != nil {
		return err
	}
	for _, arg := range strings.Split(string(argfile), "\n") {
		if len(arg) == 0 || arg[0] == '#' {
			continue
		}
		readBuildArg(arg, args)
	}
	return err
}

func readBuildArg(buildarg string, args map[string]string) {
	av := strings.SplitN(buildarg, "=", 2)
	if len(av) > 1 {
		args[av[0]] = av[1]
	} else {
		// check if the env is set in the local environment and use that value if it is
		if val, present := os.LookupEnv(av[0]); present {
			args[av[0]] = val
		} else {
			delete(args, av[0])
		}
	}
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
