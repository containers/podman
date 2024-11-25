package images

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/containers/buildah/define"
	imageTypes "github.com/containers/image/v5/types"
	ldefine "github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/auth"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/regexp"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/go-units"
	"github.com/hashicorp/go-multierror"
	jsoniter "github.com/json-iterator/go"
	gzip "github.com/klauspost/pgzip"
	"github.com/sirupsen/logrus"
)

type devino struct {
	Dev uint64
	Ino uint64
}

var iidRegex = regexp.Delayed(`^[0-9a-f]{12}`)

type BuildResponse struct {
	Stream string                 `json:"stream,omitempty"`
	Error  *jsonmessage.JSONError `json:"errorDetail,omitempty"`
	// NOTE: `error` is being deprecated check https://github.com/moby/moby/blob/master/pkg/jsonmessage/jsonmessage.go#L148
	ErrorMessage string          `json:"error,omitempty"` // deprecate this slowly
	Aux          json.RawMessage `json:"aux,omitempty"`
}

// Modify the build contexts that uses a local windows path. The windows path is
// converted into the corresping guest path in the default Windows machine
// (e.g. C:\test ==> /mnt/c/test).
func convertAdditionalBuildContexts(additionalBuildContexts map[string]*define.AdditionalBuildContext) {
	for _, context := range additionalBuildContexts {
		if !context.IsImage && !context.IsURL {
			path, err := specgen.ConvertWinMountPath(context.Value)
			// It's not worth failing if the path can't be converted
			if err == nil {
				context.Value = path
			}
		}
	}
}

// convertVolumeSrcPath converts windows paths in the HOST-DIR part of a volume
// into the corresponding path in the default Windows machine.
// (e.g. C:\test:/src/docs ==> /mnt/c/test:/src/docs).
// If any error occurs while parsing the volume string, the original volume
// string is returned.
func convertVolumeSrcPath(volume string) string {
	splitVol := specgen.SplitVolumeString(volume)
	if len(splitVol) < 2 || len(splitVol) > 3 {
		return volume
	}
	convertedSrcPath, err := specgen.ConvertWinMountPath(splitVol[0])
	if err != nil {
		return volume
	}
	if len(splitVol) == 2 {
		return convertedSrcPath + ":" + splitVol[1]
	} else {
		return convertedSrcPath + ":" + splitVol[1] + ":" + splitVol[2]
	}
}

// Build creates an image using a containerfile reference
func Build(ctx context.Context, containerFiles []string, options types.BuildOptions) (*types.BuildReport, error) {
	if options.CommonBuildOpts == nil {
		options.CommonBuildOpts = new(define.CommonBuildOptions)
	}

	params := url.Values{}

	if caps := options.AddCapabilities; len(caps) > 0 {
		c, err := jsoniter.MarshalToString(caps)
		if err != nil {
			return nil, err
		}
		params.Add("addcaps", c)
	}

	if annotations := options.Annotations; len(annotations) > 0 {
		l, err := jsoniter.MarshalToString(annotations)
		if err != nil {
			return nil, err
		}
		params.Set("annotations", l)
	}

	if cppflags := options.CPPFlags; len(cppflags) > 0 {
		l, err := jsoniter.MarshalToString(cppflags)
		if err != nil {
			return nil, err
		}
		params.Set("cppflags", l)
	}

	if options.AllPlatforms {
		params.Add("allplatforms", "1")
	}

	params.Add("t", options.Output)
	for _, tag := range options.AdditionalTags {
		params.Add("t", tag)
	}
	if additionalBuildContexts := options.AdditionalBuildContexts; len(additionalBuildContexts) > 0 {
		// TODO: Additional build contexts should be packaged and sent as tar files
		// For the time being we make our best to make them accessible on remote
		// machines too (i.e. on macOS and Windows).
		convertAdditionalBuildContexts(additionalBuildContexts)
		additionalBuildContextMap, err := jsoniter.Marshal(additionalBuildContexts)
		if err != nil {
			return nil, err
		}
		params.Set("additionalbuildcontexts", string(additionalBuildContextMap))
	}
	if options.IDMappingOptions != nil {
		idmappingsOptions, err := jsoniter.Marshal(options.IDMappingOptions)
		if err != nil {
			return nil, err
		}
		params.Set("idmappingoptions", string(idmappingsOptions))
	}
	if buildArgs := options.Args; len(buildArgs) > 0 {
		bArgs, err := jsoniter.MarshalToString(buildArgs)
		if err != nil {
			return nil, err
		}
		params.Set("buildargs", bArgs)
	}
	if excludes := options.Excludes; len(excludes) > 0 {
		bArgs, err := jsoniter.MarshalToString(excludes)
		if err != nil {
			return nil, err
		}
		params.Set("excludes", bArgs)
	}
	if cpuPeriod := options.CommonBuildOpts.CPUPeriod; cpuPeriod > 0 {
		params.Set("cpuperiod", strconv.Itoa(int(cpuPeriod)))
	}
	if cpuQuota := options.CommonBuildOpts.CPUQuota; cpuQuota > 0 {
		params.Set("cpuquota", strconv.Itoa(int(cpuQuota)))
	}
	if cpuSetCpus := options.CommonBuildOpts.CPUSetCPUs; len(cpuSetCpus) > 0 {
		params.Set("cpusetcpus", cpuSetCpus)
	}
	if cpuSetMems := options.CommonBuildOpts.CPUSetMems; len(cpuSetMems) > 0 {
		params.Set("cpusetmems", cpuSetMems)
	}
	if cpuShares := options.CommonBuildOpts.CPUShares; cpuShares > 0 {
		params.Set("cpushares", strconv.Itoa(int(cpuShares)))
	}
	if len(options.CommonBuildOpts.CgroupParent) > 0 {
		params.Set("cgroupparent", options.CommonBuildOpts.CgroupParent)
	}

	params.Set("networkmode", strconv.Itoa(int(options.ConfigureNetwork)))
	params.Set("outputformat", options.OutputFormat)

	if devices := options.Devices; len(devices) > 0 {
		d, err := jsoniter.MarshalToString(devices)
		if err != nil {
			return nil, err
		}
		params.Add("devices", d)
	}

	if dnsservers := options.CommonBuildOpts.DNSServers; len(dnsservers) > 0 {
		c, err := jsoniter.MarshalToString(dnsservers)
		if err != nil {
			return nil, err
		}
		params.Add("dnsservers", c)
	}
	if dnsoptions := options.CommonBuildOpts.DNSOptions; len(dnsoptions) > 0 {
		c, err := jsoniter.MarshalToString(dnsoptions)
		if err != nil {
			return nil, err
		}
		params.Add("dnsoptions", c)
	}
	if dnssearch := options.CommonBuildOpts.DNSSearch; len(dnssearch) > 0 {
		c, err := jsoniter.MarshalToString(dnssearch)
		if err != nil {
			return nil, err
		}
		params.Add("dnssearch", c)
	}

	if caps := options.DropCapabilities; len(caps) > 0 {
		c, err := jsoniter.MarshalToString(caps)
		if err != nil {
			return nil, err
		}
		params.Add("dropcaps", c)
	}

	if options.ForceRmIntermediateCtrs {
		params.Set("forcerm", "1")
	}
	if options.RemoveIntermediateCtrs {
		params.Set("rm", "1")
	} else {
		params.Set("rm", "0")
	}
	if options.CommonBuildOpts.OmitHistory {
		params.Set("omithistory", "1")
	} else {
		params.Set("omithistory", "0")
	}
	if len(options.From) > 0 {
		params.Set("from", options.From)
	}
	if options.IgnoreUnrecognizedInstructions {
		params.Set("ignore", "1")
	}
	params.Set("isolation", strconv.Itoa(int(options.Isolation)))
	if options.CommonBuildOpts.HTTPProxy {
		params.Set("httpproxy", "1")
	}
	if options.Jobs != nil {
		params.Set("jobs", strconv.FormatUint(uint64(*options.Jobs), 10))
	}
	if labels := options.Labels; len(labels) > 0 {
		l, err := jsoniter.MarshalToString(labels)
		if err != nil {
			return nil, err
		}
		params.Set("labels", l)
	}

	if opt := options.CommonBuildOpts.LabelOpts; len(opt) > 0 {
		o, err := jsoniter.MarshalToString(opt)
		if err != nil {
			return nil, err
		}
		params.Set("labelopts", o)
	}

	if len(options.CommonBuildOpts.SeccompProfilePath) > 0 {
		params.Set("seccomp", options.CommonBuildOpts.SeccompProfilePath)
	}

	if len(options.CommonBuildOpts.ApparmorProfile) > 0 {
		params.Set("apparmor", options.CommonBuildOpts.ApparmorProfile)
	}

	for _, layerLabel := range options.LayerLabels {
		params.Add("layerLabel", layerLabel)
	}
	if options.Layers {
		params.Set("layers", "1")
	}
	if options.LogRusage {
		params.Set("rusage", "1")
	}
	if len(options.RusageLogFile) > 0 {
		params.Set("rusagelogfile", options.RusageLogFile)
	}

	params.Set("retry", strconv.Itoa(options.MaxPullPushRetries))
	params.Set("retry-delay", options.PullPushRetryDelay.String())

	if len(options.Manifest) > 0 {
		params.Set("manifest", options.Manifest)
	}
	if options.CacheFrom != nil {
		cacheFrom := []string{}
		for _, cacheSrc := range options.CacheFrom {
			cacheFrom = append(cacheFrom, cacheSrc.String())
		}
		cacheFromJSON, err := jsoniter.MarshalToString(cacheFrom)
		if err != nil {
			return nil, err
		}
		params.Set("cachefrom", cacheFromJSON)
	}

	switch options.SkipUnusedStages {
	case imageTypes.OptionalBoolTrue:
		params.Set("skipunusedstages", "1")
	case imageTypes.OptionalBoolFalse:
		params.Set("skipunusedstages", "0")
	}

	if options.CacheTo != nil {
		cacheTo := []string{}
		for _, cacheSrc := range options.CacheTo {
			cacheTo = append(cacheTo, cacheSrc.String())
		}
		cacheToJSON, err := jsoniter.MarshalToString(cacheTo)
		if err != nil {
			return nil, err
		}
		params.Set("cacheto", cacheToJSON)
	}
	if int64(options.CacheTTL) != 0 {
		params.Set("cachettl", options.CacheTTL.String())
	}
	if memSwap := options.CommonBuildOpts.MemorySwap; memSwap > 0 {
		params.Set("memswap", strconv.Itoa(int(memSwap)))
	}
	if mem := options.CommonBuildOpts.Memory; mem > 0 {
		params.Set("memory", strconv.Itoa(int(mem)))
	}
	switch options.CompatVolumes {
	case imageTypes.OptionalBoolTrue:
		params.Set("compatvolumes", "1")
	case imageTypes.OptionalBoolFalse:
		params.Set("compatvolumes", "0")
	}
	if options.NoCache {
		params.Set("nocache", "1")
	}
	if options.CommonBuildOpts.NoHosts {
		params.Set("nohosts", "1")
	}
	if t := options.Output; len(t) > 0 {
		params.Set("output", t)
	}
	if t := options.OSVersion; len(t) > 0 {
		params.Set("osversion", t)
	}
	for _, t := range options.OSFeatures {
		params.Set("osfeature", t)
	}
	var platform string
	if len(options.OS) > 0 {
		platform = options.OS
	}
	if len(options.Architecture) > 0 {
		if len(platform) == 0 {
			platform = "linux"
		}
		platform += "/" + options.Architecture
	} else if len(platform) > 0 {
		platform += "/" + runtime.GOARCH
	}
	if len(platform) > 0 {
		params.Set("platform", platform)
	}
	if len(options.Platforms) > 0 {
		params.Del("platform")
		for _, platformSpec := range options.Platforms {
			// podman-cli will send empty struct, in such
			// case don't add platform to param and let the
			// build backend decide the default platform.
			if platformSpec.OS == "" && platformSpec.Arch == "" && platformSpec.Variant == "" {
				continue
			}
			platform = platformSpec.OS + "/" + platformSpec.Arch
			if platformSpec.Variant != "" {
				platform += "/" + platformSpec.Variant
			}
			params.Add("platform", platform)
		}
	}

	for _, volume := range options.CommonBuildOpts.Volumes {
		params.Add("volume", convertVolumeSrcPath(volume))
	}

	for _, group := range options.GroupAdd {
		params.Add("groupadd", group)
	}

	var err error
	var contextDir string
	if contextDir, err = filepath.EvalSymlinks(options.ContextDirectory); err == nil {
		options.ContextDirectory = contextDir
	}

	params.Set("pullpolicy", options.PullPolicy.String())

	switch options.CommonBuildOpts.IdentityLabel {
	case imageTypes.OptionalBoolTrue:
		params.Set("identitylabel", "1")
	case imageTypes.OptionalBoolFalse:
		params.Set("identitylabel", "0")
	}
	if options.Quiet {
		params.Set("q", "1")
	}
	if options.RemoveIntermediateCtrs {
		params.Set("rm", "1")
	}
	if len(options.Target) > 0 {
		params.Set("target", options.Target)
	}

	if hosts := options.CommonBuildOpts.AddHost; len(hosts) > 0 {
		h, err := jsoniter.MarshalToString(hosts)
		if err != nil {
			return nil, err
		}
		params.Set("extrahosts", h)
	}
	if nsoptions := options.NamespaceOptions; len(nsoptions) > 0 {
		ns, err := jsoniter.MarshalToString(nsoptions)
		if err != nil {
			return nil, err
		}
		params.Set("nsoptions", ns)
	}
	if shmSize := options.CommonBuildOpts.ShmSize; len(shmSize) > 0 {
		shmBytes, err := units.RAMInBytes(shmSize)
		if err != nil {
			return nil, err
		}
		params.Set("shmsize", strconv.Itoa(int(shmBytes)))
	}
	if options.Squash {
		params.Set("squash", "1")
	}

	if options.Timestamp != nil {
		t := *options.Timestamp
		params.Set("timestamp", strconv.FormatInt(t.Unix(), 10))
	}

	if len(options.CommonBuildOpts.Ulimit) > 0 {
		ulimitsJSON, err := json.Marshal(options.CommonBuildOpts.Ulimit)
		if err != nil {
			return nil, err
		}
		params.Set("ulimits", string(ulimitsJSON))
	}

	for _, env := range options.Envs {
		params.Add("setenv", env)
	}

	for _, uenv := range options.UnsetEnvs {
		params.Add("unsetenv", uenv)
	}

	for _, ulabel := range options.UnsetLabels {
		params.Add("unsetlabel", ulabel)
	}

	var (
		headers http.Header
	)
	if options.SystemContext != nil {
		if options.SystemContext.DockerAuthConfig != nil {
			headers, err = auth.MakeXRegistryAuthHeader(options.SystemContext, options.SystemContext.DockerAuthConfig.Username, options.SystemContext.DockerAuthConfig.Password)
		} else {
			headers, err = auth.MakeXRegistryConfigHeader(options.SystemContext, "", "")
		}
		if options.SystemContext.DockerInsecureSkipTLSVerify == imageTypes.OptionalBoolTrue {
			params.Set("tlsVerify", "false")
		}
	}
	if err != nil {
		return nil, err
	}

	stdout := io.Writer(os.Stdout)
	if options.Out != nil {
		stdout = options.Out
	}

	contextDir, err = filepath.Abs(options.ContextDirectory)
	if err != nil {
		logrus.Errorf("Cannot find absolute path of %v: %v", options.ContextDirectory, err)
		return nil, err
	}

	tarContent := []string{options.ContextDirectory}
	newContainerFiles := []string{} // dockerfile paths, relative to context dir, ToSlash()ed

	dontexcludes := []string{"!Dockerfile", "!Containerfile", "!.dockerignore", "!.containerignore"}
	for _, c := range containerFiles {
		// Don not add path to containerfile if it is a URL
		if strings.HasPrefix(c, "http://") || strings.HasPrefix(c, "https://") {
			newContainerFiles = append(newContainerFiles, c)
			continue
		}
		if c == "/dev/stdin" {
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				return nil, err
			}
			tmpFile, err := os.CreateTemp("", "build")
			if err != nil {
				return nil, err
			}
			defer os.Remove(tmpFile.Name()) // clean up
			defer tmpFile.Close()
			if _, err := tmpFile.Write(content); err != nil {
				return nil, err
			}
			c = tmpFile.Name()
		}
		c = filepath.Clean(c)
		cfDir := filepath.Dir(c)
		if absDir, err := filepath.EvalSymlinks(cfDir); err == nil {
			name := filepath.ToSlash(strings.TrimPrefix(c, cfDir+string(filepath.Separator)))
			c = filepath.Join(absDir, name)
		}

		containerfile, err := filepath.Abs(c)
		if err != nil {
			logrus.Errorf("Cannot find absolute path of %v: %v", c, err)
			return nil, err
		}

		// Check if Containerfile is in the context directory, if so truncate the context directory off path
		// Do NOT add to tarfile
		if strings.HasPrefix(containerfile, contextDir+string(filepath.Separator)) {
			containerfile = strings.TrimPrefix(containerfile, contextDir+string(filepath.Separator))
			dontexcludes = append(dontexcludes, "!"+containerfile)
			dontexcludes = append(dontexcludes, "!"+containerfile+".dockerignore")
			dontexcludes = append(dontexcludes, "!"+containerfile+".containerignore")
		} else {
			// If Containerfile does not exist, assume it is in context directory and do Not add to tarfile
			if err := fileutils.Lexists(containerfile); err != nil {
				if !os.IsNotExist(err) {
					return nil, err
				}
				containerfile = c
				dontexcludes = append(dontexcludes, "!"+containerfile)
				dontexcludes = append(dontexcludes, "!"+containerfile+".dockerignore")
				dontexcludes = append(dontexcludes, "!"+containerfile+".containerignore")
			} else {
				// If Containerfile does exist and not in the context directory, add it to the tarfile
				tarContent = append(tarContent, containerfile)
			}
		}
		newContainerFiles = append(newContainerFiles, filepath.ToSlash(containerfile))
	}

	if len(newContainerFiles) > 0 {
		cFileJSON, err := json.Marshal(newContainerFiles)
		if err != nil {
			return nil, err
		}
		params.Set("dockerfile", string(cFileJSON))
	}

	excludes := options.Excludes
	if len(excludes) == 0 {
		excludes, _, err = util.ParseDockerignore(newContainerFiles, options.ContextDirectory)
		if err != nil {
			return nil, err
		}
	}

	saveFormat := ldefine.OCIArchive
	if options.OutputFormat == define.Dockerv2ImageManifest {
		saveFormat = ldefine.V2s2Archive
	}

	// build secrets are usually absolute host path or relative to context dir on host
	// in any case move secret to current context and ship the tar.
	if secrets := options.CommonBuildOpts.Secrets; len(secrets) > 0 {
		secretsForRemote := []string{}

		for _, secret := range secrets {
			secretOpt := strings.Split(secret, ",")
			if len(secretOpt) > 0 {
				modifiedOpt := []string{}
				for _, token := range secretOpt {
					opt, val, hasVal := strings.Cut(token, "=")
					if hasVal {
						if opt == "src" {
							// read specified secret into a tmp file
							// move tmp file to tar and change secret source to relative tmp file
							tmpSecretFile, err := os.CreateTemp(options.ContextDirectory, "podman-build-secret")
							if err != nil {
								return nil, err
							}
							defer os.Remove(tmpSecretFile.Name()) // clean up
							defer tmpSecretFile.Close()
							srcSecretFile, err := os.Open(val)
							if err != nil {
								return nil, err
							}
							defer srcSecretFile.Close()
							_, err = io.Copy(tmpSecretFile, srcSecretFile)
							if err != nil {
								return nil, err
							}

							// add tmp file to context dir
							tarContent = append(tarContent, tmpSecretFile.Name())

							modifiedSrc := fmt.Sprintf("src=%s", filepath.Base(tmpSecretFile.Name()))
							modifiedOpt = append(modifiedOpt, modifiedSrc)
						} else {
							modifiedOpt = append(modifiedOpt, token)
						}
					}
				}
				secretsForRemote = append(secretsForRemote, strings.Join(modifiedOpt, ","))
			}
		}

		c, err := jsoniter.MarshalToString(secretsForRemote)
		if err != nil {
			return nil, err
		}
		params.Add("secrets", c)
	}

	tarfile, err := nTar(append(excludes, dontexcludes...), tarContent...)
	if err != nil {
		logrus.Errorf("Cannot tar container entries %v error: %v", tarContent, err)
		return nil, err
	}
	defer func() {
		if err := tarfile.Close(); err != nil {
			logrus.Errorf("%v\n", err)
		}
	}()

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, tarfile, http.MethodPost, "/build", params, headers)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if !response.IsSuccess() {
		return nil, response.Process(err)
	}

	body := response.Body.(io.Reader)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		if v, found := os.LookupEnv("PODMAN_RETAIN_BUILD_ARTIFACT"); found {
			if keep, _ := strconv.ParseBool(v); keep {
				t, _ := os.CreateTemp("", "build_*_client")
				defer t.Close()
				body = io.TeeReader(response.Body, t)
			}
		}
	}

	dec := json.NewDecoder(body)

	var id string
	for {
		var s BuildResponse
		select {
		// FIXME(vrothberg): it seems we always hit the EOF case below,
		// even when the server quit but it seems desirable to
		// distinguish a proper build from a transient EOF.
		case <-response.Request.Context().Done():
			return &types.BuildReport{ID: id, SaveFormat: saveFormat}, nil
		default:
			// non-blocking select
		}

		if err := dec.Decode(&s); err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, fmt.Errorf("server probably quit: %w", err)
			}
			// EOF means the stream is over in which case we need
			// to have read the id.
			if errors.Is(err, io.EOF) && id != "" {
				break
			}
			return &types.BuildReport{ID: id, SaveFormat: saveFormat}, fmt.Errorf("decoding stream: %w", err)
		}

		switch {
		case s.Stream != "":
			raw := []byte(s.Stream)
			stdout.Write(raw)
			if iidRegex.Match(raw) {
				id = strings.TrimSuffix(s.Stream, "\n")
			}
		case s.Error != nil:
			// If there's an error, return directly.  The stream
			// will be closed on return.
			return &types.BuildReport{ID: id, SaveFormat: saveFormat}, errors.New(s.Error.Message)
		default:
			return &types.BuildReport{ID: id, SaveFormat: saveFormat}, errors.New("failed to parse build results stream, unexpected input")
		}
	}
	return &types.BuildReport{ID: id, SaveFormat: saveFormat}, nil
}

func nTar(excludes []string, sources ...string) (io.ReadCloser, error) {
	pm, err := fileutils.NewPatternMatcher(excludes)
	if err != nil {
		return nil, fmt.Errorf("processing excludes list %v: %w", excludes, err)
	}

	if len(sources) == 0 {
		return nil, errors.New("no source(s) provided for build")
	}

	pr, pw := io.Pipe()
	gw := gzip.NewWriter(pw)
	tw := tar.NewWriter(gw)

	var merr *multierror.Error
	go func() {
		defer pw.Close()
		defer gw.Close()
		defer tw.Close()
		seen := make(map[devino]string)
		for i, src := range sources {
			source, err := filepath.Abs(src)
			if err != nil {
				logrus.Errorf("Cannot stat one of source context: %v", err)
				merr = multierror.Append(merr, err)
				return
			}
			err = filepath.WalkDir(source, func(path string, dentry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				separator := string(filepath.Separator)
				// check if what we are given is an empty dir, if so then continue w/ it. Else return.
				// if we are given a file or a symlink, we do not want to exclude it.
				if source == path {
					separator = ""
					if dentry.IsDir() {
						var p *os.File
						p, err = os.Open(path)
						if err != nil {
							return err
						}
						defer p.Close()
						_, err = p.Readdir(1)
						if err == nil {
							return nil // non empty root dir, need to return
						}
						if err != io.EOF {
							logrus.Errorf("While reading directory %v: %v", path, err)
						}
					}
				}
				var name string
				if i == 0 {
					name = filepath.ToSlash(strings.TrimPrefix(path, source+separator))
				} else {
					if !dentry.Type().IsRegular() {
						return fmt.Errorf("path %s must be a regular file", path)
					}
					name = filepath.ToSlash(path)
				}
				// If name is absolute path, then it has to be containerfile outside of build context.
				// If not, we should check it for being excluded via pattern matcher.
				if !filepath.IsAbs(name) {
					excluded, err := pm.Matches(name) //nolint:staticcheck
					if err != nil {
						return fmt.Errorf("checking if %q is excluded: %w", name, err)
					}
					if excluded {
						// Note: filepath.SkipDir is not possible to use given .dockerignore semantics.
						// An exception to exclusions may include an excluded directory, therefore we
						// are required to visit all files. :(
						return nil
					}
				}
				switch {
				case dentry.Type().IsRegular(): // add file item
					info, err := dentry.Info()
					if err != nil {
						return err
					}
					di, isHardLink := checkHardLink(info)
					if err != nil {
						return err
					}

					hdr, err := tar.FileInfoHeader(info, "")
					if err != nil {
						return err
					}
					hdr.Uid, hdr.Gid = 0, 0
					orig, ok := seen[di]
					if ok {
						hdr.Typeflag = tar.TypeLink
						hdr.Linkname = orig
						hdr.Size = 0
						hdr.Name = name
						return tw.WriteHeader(hdr)
					}
					f, err := os.Open(path)
					if err != nil {
						return err
					}

					hdr.Name = name
					if err := tw.WriteHeader(hdr); err != nil {
						f.Close()
						return err
					}

					_, err = io.Copy(tw, f)
					f.Close()
					if err == nil && isHardLink {
						seen[di] = name
					}
					return err
				case dentry.IsDir(): // add folders
					info, err := dentry.Info()
					if err != nil {
						return err
					}
					hdr, lerr := tar.FileInfoHeader(info, name)
					if lerr != nil {
						return lerr
					}
					hdr.Name = name
					hdr.Uid, hdr.Gid = 0, 0
					if lerr := tw.WriteHeader(hdr); lerr != nil {
						return lerr
					}
				case dentry.Type()&os.ModeSymlink != 0: // add symlinks as it, not content
					link, err := os.Readlink(path)
					if err != nil {
						return err
					}
					info, err := dentry.Info()
					if err != nil {
						return err
					}
					hdr, lerr := tar.FileInfoHeader(info, link)
					if lerr != nil {
						return lerr
					}
					hdr.Name = name
					hdr.Uid, hdr.Gid = 0, 0
					if lerr := tw.WriteHeader(hdr); lerr != nil {
						return lerr
					}
				} // skip other than file,folder and symlinks
				return nil
			})
			merr = multierror.Append(merr, err)
		}
	}()
	rc := ioutils.NewReadCloserWrapper(pr, func() error {
		if merr != nil {
			merr = multierror.Append(merr, pr.Close())
			return merr.ErrorOrNil()
		}
		return pr.Close()
	})
	return rc, nil
}
