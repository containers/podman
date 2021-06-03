package images

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/containers/podman/v3/pkg/auth"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/docker/go-units"
	"github.com/hashicorp/go-multierror"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type devino struct {
	Dev uint64
	Ino uint64
}

var (
	iidRegex = regexp.MustCompile(`^[0-9a-f]{12}`)
)

// Build creates an image using a containerfile reference
func Build(ctx context.Context, containerFiles []string, options entities.BuildOptions) (*entities.BuildReport, error) {
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
	params.Add("t", options.Output)
	for _, tag := range options.AdditionalTags {
		params.Add("t", tag)
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
	if cpuShares := options.CommonBuildOpts.CPUShares; cpuShares > 0 {
		params.Set("cpushares", strconv.Itoa(int(cpuShares)))
	}
	if cpuSetCpus := options.CommonBuildOpts.CPUSetCPUs; len(cpuSetCpus) > 0 {
		params.Set("cpusetcpus", cpuSetCpus)
	}
	if cpuPeriod := options.CommonBuildOpts.CPUPeriod; cpuPeriod > 0 {
		params.Set("cpuperiod", strconv.Itoa(int(cpuPeriod)))
	}
	if cpuQuota := options.CommonBuildOpts.CPUQuota; cpuQuota > 0 {
		params.Set("cpuquota", strconv.Itoa(int(cpuQuota)))
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

	if options.Layers {
		params.Set("layers", "1")
	}
	if options.LogRusage {
		params.Set("rusage", "1")
	}
	if len(options.Manifest) > 0 {
		params.Set("manifest", options.Manifest)
	}
	if memSwap := options.CommonBuildOpts.MemorySwap; memSwap > 0 {
		params.Set("memswap", strconv.Itoa(int(memSwap)))
	}
	if mem := options.CommonBuildOpts.Memory; mem > 0 {
		params.Set("memory", strconv.Itoa(int(mem)))
	}
	if options.NoCache {
		params.Set("nocache", "1")
	}
	if t := options.Output; len(t) > 0 {
		params.Set("output", t)
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
	} else {
		if len(platform) > 0 {
			platform += "/" + runtime.GOARCH
		}
	}
	if len(platform) > 0 {
		params.Set("platform", platform)
	}

	params.Set("pullpolicy", options.PullPolicy.String())

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
	var (
		headers map[string]string
		err     error
	)
	if options.SystemContext == nil {
		headers, err = auth.Header(options.SystemContext, auth.XRegistryConfigHeader, "", "", "")
	} else {
		if options.SystemContext.DockerAuthConfig != nil {
			headers, err = auth.Header(options.SystemContext, auth.XRegistryAuthHeader, options.SystemContext.AuthFilePath, options.SystemContext.DockerAuthConfig.Username, options.SystemContext.DockerAuthConfig.Password)
		} else {
			headers, err = auth.Header(options.SystemContext, auth.XRegistryConfigHeader, options.SystemContext.AuthFilePath, "", "")
		}
	}
	if err != nil {
		return nil, err
	}

	stdout := io.Writer(os.Stdout)
	if options.Out != nil {
		stdout = options.Out
	}

	excludes := options.Excludes
	if len(excludes) == 0 {
		excludes, err = parseDockerignore(options.ContextDirectory)
		if err != nil {
			return nil, err
		}
	}

	contextDir, err := filepath.Abs(options.ContextDirectory)
	if err != nil {
		logrus.Errorf("cannot find absolute path of %v: %v", options.ContextDirectory, err)
		return nil, err
	}

	tarContent := []string{options.ContextDirectory}
	newContainerFiles := []string{}
	for _, c := range containerFiles {
		containerfile, err := filepath.Abs(c)
		if err != nil {
			logrus.Errorf("cannot find absolute path of %v: %v", c, err)
			return nil, err
		}

		// Check if Containerfile is in the context directory, if so truncate the contextdirectory off path
		// Do NOT add to tarfile
		if strings.HasPrefix(containerfile, contextDir+string(filepath.Separator)) {
			containerfile = strings.TrimPrefix(containerfile, contextDir+string(filepath.Separator))
		} else {
			// If Containerfile does not exists assume it is in context directory, do Not add to tarfile
			if _, err := os.Lstat(containerfile); err != nil {
				if !os.IsNotExist(err) {
					return nil, err
				}
				containerfile = c
			} else {
				// If Containerfile does exists but is not in context directory add it to the tarfile
				tarContent = append(tarContent, containerfile)
			}
		}
		newContainerFiles = append(newContainerFiles, containerfile)
	}
	if len(newContainerFiles) > 0 {
		cFileJSON, err := json.Marshal(newContainerFiles)
		if err != nil {
			return nil, err
		}
		params.Set("dockerfile", string(cFileJSON))
	}

	tarfile, err := nTar(excludes, tarContent...)
	if err != nil {
		logrus.Errorf("cannot tar container entries %v error: %v", tarContent, err)
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
	response, err := conn.DoRequest(tarfile, http.MethodPost, "/build", params, headers)
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
				t, _ := ioutil.TempFile("", "build_*_client")
				defer t.Close()
				body = io.TeeReader(response.Body, t)
			}
		}
	}

	dec := json.NewDecoder(body)

	var id string
	var mErr error
	for {
		var s struct {
			Stream string `json:"stream,omitempty"`
			Error  string `json:"error,omitempty"`
		}
		if err := dec.Decode(&s); err != nil {
			if errors.Is(err, io.EOF) {
				if mErr == nil && id == "" {
					mErr = errors.New("stream dropped, unexpected failure")
				}
				break
			}
			s.Error = err.Error() + "\n"
		}

		select {
		case <-response.Request.Context().Done():
			return &entities.BuildReport{ID: id}, mErr
		default:
			// non-blocking select
		}

		switch {
		case s.Stream != "":
			stdout.Write([]byte(s.Stream))
			if iidRegex.Match([]byte(s.Stream)) {
				id = strings.TrimSuffix(s.Stream, "\n")
			}
		case s.Error != "":
			mErr = errors.New(s.Error)
		default:
			return &entities.BuildReport{ID: id}, errors.New("failed to parse build results stream, unexpected input")
		}
	}
	return &entities.BuildReport{ID: id}, mErr
}

func nTar(excludes []string, sources ...string) (io.ReadCloser, error) {
	pm, err := fileutils.NewPatternMatcher(excludes)
	if err != nil {
		return nil, errors.Wrapf(err, "error processing excludes list %v", excludes)
	}

	if len(sources) == 0 {
		return nil, errors.New("No source(s) provided for build")
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
		for _, src := range sources {
			s, err := filepath.Abs(src)
			if err != nil {
				logrus.Errorf("cannot stat one of source context: %v", err)
				merr = multierror.Append(merr, err)
				return
			}

			err = filepath.Walk(s, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if path == s {
					return nil // skip root dir
				}

				name := strings.TrimPrefix(path, s+string(filepath.Separator))

				excluded, err := pm.Matches(filepath.ToSlash(name)) // nolint:staticcheck
				if err != nil {
					return errors.Wrapf(err, "error checking if %q is excluded", name)
				}
				if excluded {
					return nil
				}

				if info.Mode().IsRegular() { // add file item
					di, isHardLink := checkHardLink(info)
					if err != nil {
						return err
					}

					hdr, err := tar.FileInfoHeader(info, "")
					if err != nil {
						return err
					}
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
				} else if info.Mode().IsDir() { // add folders
					hdr, lerr := tar.FileInfoHeader(info, name)
					if lerr != nil {
						return lerr
					}
					hdr.Name = name
					if lerr := tw.WriteHeader(hdr); lerr != nil {
						return lerr
					}
				} else if info.Mode()&os.ModeSymlink != 0 { // add symlinks as it, not content
					link, err := os.Readlink(path)
					if err != nil {
						return err
					}
					hdr, lerr := tar.FileInfoHeader(info, link)
					if lerr != nil {
						return lerr
					}
					hdr.Name = name
					if lerr := tw.WriteHeader(hdr); lerr != nil {
						return lerr
					}
				} //skip other than file,folder and symlinks
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

func parseDockerignore(root string) ([]string, error) {
	ignore, err := ioutil.ReadFile(filepath.Join(root, ".dockerignore"))
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrapf(err, "error reading .dockerignore: '%s'", root)
	}
	rawexcludes := strings.Split(string(ignore), "\n")
	excludes := make([]string, 0, len(rawexcludes))
	for _, e := range rawexcludes {
		if len(e) == 0 || e[0] == '#' {
			continue
		}
		excludes = append(excludes, e)
	}
	return excludes, nil
}
