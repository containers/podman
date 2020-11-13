package images

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/podman/v2/pkg/auth"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/docker/go-units"
	"github.com/hashicorp/go-multierror"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Build creates an image using a containerfile reference
func Build(ctx context.Context, containerFiles []string, options entities.BuildOptions) (*entities.BuildReport, error) {
	params := url.Values{}

	if t := options.Output; len(t) > 0 {
		params.Set("t", t)
	}
	for _, tag := range options.AdditionalTags {
		params.Add("t", tag)
	}
	if options.Quiet {
		params.Set("q", "1")
	}
	if options.NoCache {
		params.Set("nocache", "1")
	}
	//	 TODO cachefrom
	if options.PullPolicy == buildah.PullAlways {
		params.Set("pull", "1")
	}
	if options.RemoveIntermediateCtrs {
		params.Set("rm", "1")
	}
	if options.ForceRmIntermediateCtrs {
		params.Set("forcerm", "1")
	}
	if mem := options.CommonBuildOpts.Memory; mem > 0 {
		params.Set("memory", strconv.Itoa(int(mem)))
	}
	if memSwap := options.CommonBuildOpts.MemorySwap; memSwap > 0 {
		params.Set("memswap", strconv.Itoa(int(memSwap)))
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
	if buildArgs := options.Args; len(buildArgs) > 0 {
		bArgs, err := jsoniter.MarshalToString(buildArgs)
		if err != nil {
			return nil, err
		}
		params.Set("buildargs", bArgs)
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
	if labels := options.Labels; len(labels) > 0 {
		l, err := jsoniter.MarshalToString(labels)
		if err != nil {
			return nil, err
		}
		params.Set("labels", l)
	}
	if options.CommonBuildOpts.HTTPProxy {
		params.Set("httpproxy", "1")
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

	// TODO network?

	var platform string
	if OS := options.OS; len(OS) > 0 {
		platform += OS
	}
	if arch := options.Architecture; len(arch) > 0 {
		platform += "/" + arch
	}
	if len(platform) > 0 {
		params.Set("platform", platform)
	}

	entries := make([]string, len(containerFiles))
	copy(entries, containerFiles)
	entries = append(entries, options.ContextDirectory)
	tarfile, err := nTar(entries...)
	if err != nil {
		return nil, err
	}
	defer tarfile.Close()
	params.Set("dockerfile", filepath.Base(containerFiles[0]))

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
	re := regexp.MustCompile(`[0-9a-f]{12}`)

	var id string
	for {
		var s struct {
			Stream string `json:"stream,omitempty"`
			Error  string `json:"error,omitempty"`
		}
		if err := dec.Decode(&s); err != nil {
			if errors.Is(err, io.EOF) {
				return &entities.BuildReport{ID: id}, nil
			}
			s.Error = err.Error() + "\n"
		}

		switch {
		case s.Stream != "":
			stdout.Write([]byte(s.Stream))
			if re.Match([]byte(s.Stream)) {
				id = strings.TrimSuffix(s.Stream, "\n")
			}
		case s.Error != "":
			return nil, errors.New(s.Error)
		default:
			return &entities.BuildReport{ID: id}, errors.New("failed to parse build results stream, unexpected input")
		}
	}
}

func nTar(sources ...string) (io.ReadCloser, error) {
	if len(sources) == 0 {
		return nil, errors.New("No source(s) provided for build")
	}

	pr, pw := io.Pipe()
	tw := tar.NewWriter(pw)

	var merr error
	go func() {
		defer pw.Close()
		defer tw.Close()

		for _, src := range sources {
			s := src
			err := filepath.Walk(s, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.Mode().IsRegular() || path == s {
					return nil
				}

				f, lerr := os.Open(path)
				if lerr != nil {
					return lerr
				}

				name := strings.TrimPrefix(path, s+string(filepath.Separator))
				hdr, lerr := tar.FileInfoHeader(info, name)
				if lerr != nil {
					f.Close()
					return lerr
				}
				hdr.Name = name
				if lerr := tw.WriteHeader(hdr); lerr != nil {
					f.Close()
					return lerr
				}

				_, cerr := io.Copy(tw, f)
				f.Close()
				return cerr
			})
			merr = multierror.Append(merr, err)
		}
	}()
	return pr, merr
}
