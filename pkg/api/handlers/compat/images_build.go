package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/auth"
	"github.com/containers/podman/v2/pkg/channel"
	"github.com/containers/storage/pkg/archive"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func BuildImage(w http.ResponseWriter, r *http.Request) {
	if hdr, found := r.Header["Content-Type"]; found && len(hdr) > 0 {
		contentType := hdr[0]
		switch contentType {
		case "application/tar":
			logrus.Warnf("tar file content type is  %s, should use \"application/x-tar\" content type", contentType)
		case "application/x-tar":
			break
		default:
			utils.BadRequest(w, "Content-Type", hdr[0],
				fmt.Errorf("Content-Type: %s is not supported. Should be \"application/x-tar\"", hdr[0]))
			return
		}
	}

	contextDirectory, err := extractTarFile(r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	defer func() {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			if v, found := os.LookupEnv("PODMAN_RETAIN_BUILD_ARTIFACT"); found {
				if keep, _ := strconv.ParseBool(v); keep {
					return
				}
			}
		}
		err := os.RemoveAll(filepath.Dir(contextDirectory))
		if err != nil {
			logrus.Warn(errors.Wrapf(err, "failed to remove build scratch directory %q", filepath.Dir(contextDirectory)))
		}
	}()

	query := struct {
		AddHosts               string `schema:"extrahosts"`
		AdditionalCapabilities string `schema:"addcaps"`
		Annotations            string `schema:"annotations"`
		BuildArgs              string `schema:"buildargs"`
		CacheFrom              string `schema:"cachefrom"`
		ConfigureNetwork       int64  `schema:"networkmode"`
		CpuPeriod              uint64 `schema:"cpuperiod"`  // nolint
		CpuQuota               int64  `schema:"cpuquota"`   // nolint
		CpuSetCpus             string `schema:"cpusetcpus"` // nolint
		CpuShares              uint64 `schema:"cpushares"`  // nolint
		Devices                string `schema:"devices"`
		Dockerfile             string `schema:"dockerfile"`
		DropCapabilities       string `schema:"dropcaps"`
		ForceRm                bool   `schema:"forcerm"`
		From                   string `schema:"from"`
		HTTPProxy              bool   `schema:"httpproxy"`
		Isolation              int64  `schema:"isolation"`
		Jobs                   uint64 `schema:"jobs"` // nolint
		Labels                 string `schema:"labels"`
		Layers                 bool   `schema:"layers"`
		LogRusage              bool   `schema:"rusage"`
		Manifest               string `schema:"manifest"`
		MemSwap                int64  `schema:"memswap"`
		Memory                 int64  `schema:"memory"`
		NoCache                bool   `schema:"nocache"`
		OutputFormat           string `schema:"outputformat"`
		Platform               string `schema:"platform"`
		Pull                   bool   `schema:"pull"`
		Quiet                  bool   `schema:"q"`
		Registry               string `schema:"registry"`
		Rm                     bool   `schema:"rm"`
		//FIXME SecurityOpt in remote API is not handled
		SecurityOpt string   `schema:"securityopt"`
		ShmSize     int      `schema:"shmsize"`
		Squash      bool     `schema:"squash"`
		Tag         []string `schema:"t"`
		Target      string   `schema:"target"`
	}{
		Dockerfile: "Dockerfile",
		Registry:   "docker.io",
		Rm:         true,
		ShmSize:    64 * 1024 * 1024,
		Tag:        []string{},
	}

	decoder := r.Context().Value("decoder").(*schema.Decoder)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, err)
		return
	}

	// convert label formats
	var addCaps = []string{}
	if _, found := r.URL.Query()["addcaps"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.AdditionalCapabilities), &m); err != nil {
			utils.BadRequest(w, "addcaps", query.AdditionalCapabilities, err)
			return
		}
		addCaps = m
	}
	addhosts := []string{}
	if _, found := r.URL.Query()["extrahosts"]; found {
		if err := json.Unmarshal([]byte(query.AddHosts), &addhosts); err != nil {
			utils.BadRequest(w, "extrahosts", query.AddHosts, err)
			return
		}
	}

	// convert label formats
	var dropCaps = []string{}
	if _, found := r.URL.Query()["dropcaps"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.DropCapabilities), &m); err != nil {
			utils.BadRequest(w, "dropcaps", query.DropCapabilities, err)
			return
		}
		dropCaps = m
	}

	// convert label formats
	var devices = []string{}
	if _, found := r.URL.Query()["devices"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.DropCapabilities), &m); err != nil {
			utils.BadRequest(w, "devices", query.DropCapabilities, err)
			return
		}
		devices = m
	}

	var output string
	if len(query.Tag) > 0 {
		output = query.Tag[0]
	}
	format := buildah.Dockerv2ImageManifest
	if utils.IsLibpodRequest(r) {
		format = query.OutputFormat
	}
	var additionalTags []string
	if len(query.Tag) > 1 {
		additionalTags = query.Tag[1:]
	}

	var buildArgs = map[string]string{}
	if _, found := r.URL.Query()["buildargs"]; found {
		if err := json.Unmarshal([]byte(query.BuildArgs), &buildArgs); err != nil {
			utils.BadRequest(w, "buildargs", query.BuildArgs, err)
			return
		}
	}

	// convert label formats
	var annotations = []string{}
	if _, found := r.URL.Query()["annotations"]; found {
		if err := json.Unmarshal([]byte(query.Annotations), &annotations); err != nil {
			utils.BadRequest(w, "annotations", query.Annotations, err)
			return
		}
	}

	// convert label formats
	var labels = []string{}
	if _, found := r.URL.Query()["labels"]; found {
		if err := json.Unmarshal([]byte(query.Labels), &labels); err != nil {
			utils.BadRequest(w, "labels", query.Labels, err)
			return
		}
	}

	pullPolicy := buildah.PullIfMissing
	if _, found := r.URL.Query()["pull"]; found {
		if query.Pull {
			pullPolicy = buildah.PullAlways
		}
	}

	creds, authfile, key, err := auth.GetCredentials(r)
	if err != nil {
		// Credential value(s) not returned as their value is not human readable
		utils.BadRequest(w, key.String(), "n/a", err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	// Channels all mux'ed in select{} below to follow API build protocol
	stdout := channel.NewWriter(make(chan []byte, 1))
	defer stdout.Close()

	auxout := channel.NewWriter(make(chan []byte, 1))
	defer auxout.Close()

	stderr := channel.NewWriter(make(chan []byte, 1))
	defer stderr.Close()

	reporter := channel.NewWriter(make(chan []byte, 1))
	defer reporter.Close()

	buildOptions := imagebuildah.BuildOptions{
		AddCapabilities: addCaps,
		AdditionalTags:  additionalTags,
		Annotations:     annotations,
		Args:            buildArgs,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			AddHost:    addhosts,
			CPUPeriod:  query.CpuPeriod,
			CPUQuota:   query.CpuQuota,
			CPUShares:  query.CpuShares,
			CPUSetCPUs: query.CpuSetCpus,
			HTTPProxy:  query.HTTPProxy,
			Memory:     query.Memory,
			MemorySwap: query.MemSwap,
			ShmSize:    strconv.Itoa(query.ShmSize),
		},
		Compression:                    archive.Gzip,
		ConfigureNetwork:               buildah.NetworkConfigurationPolicy(query.ConfigureNetwork),
		ContextDirectory:               contextDirectory,
		Devices:                        devices,
		DropCapabilities:               dropCaps,
		Err:                            auxout,
		ForceRmIntermediateCtrs:        query.ForceRm,
		From:                           query.From,
		IgnoreUnrecognizedInstructions: true,
		// FIXME, This is very broken.  Buildah will only work with chroot
		//		Isolation:                      buildah.Isolation(query.Isolation),
		Isolation: buildah.IsolationChroot,

		Labels:                 labels,
		Layers:                 query.Layers,
		Manifest:               query.Manifest,
		NoCache:                query.NoCache,
		Out:                    stdout,
		Output:                 output,
		OutputFormat:           format,
		PullPolicy:             pullPolicy,
		Quiet:                  query.Quiet,
		Registry:               query.Registry,
		RemoveIntermediateCtrs: query.Rm,
		ReportWriter:           reporter,
		Squash:                 query.Squash,
		SystemContext: &types.SystemContext{
			AuthFilePath:     authfile,
			DockerAuthConfig: creds,
		},
		Target: query.Target,
	}

	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	runCtx, cancel := context.WithCancel(context.Background())
	var imageID string
	go func() {
		defer cancel()
		imageID, _, err = runtime.Build(r.Context(), buildOptions, query.Dockerfile)
		if err != nil {
			stderr.Write([]byte(err.Error() + "\n"))
		}
	}()

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// Send headers and prime client for stream to come
	w.WriteHeader(http.StatusOK)
	w.Header().Add("Content-Type", "application/json")
	flush()

	var failed bool

	body := w.(io.Writer)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		if v, found := os.LookupEnv("PODMAN_RETAIN_BUILD_ARTIFACT"); found {
			if keep, _ := strconv.ParseBool(v); keep {
				t, _ := ioutil.TempFile("", "build_*_server")
				defer t.Close()
				body = io.MultiWriter(t, w)
			}
		}
	}

	enc := json.NewEncoder(body)
	enc.SetEscapeHTML(true)
loop:
	for {
		m := struct {
			Stream string `json:"stream,omitempty"`
			Error  string `json:"error,omitempty"`
		}{}

		select {
		case e := <-stdout.Chan():
			m.Stream = string(e)
			if err := enc.Encode(m); err != nil {
				stderr.Write([]byte(err.Error()))
			}
			flush()
		case e := <-auxout.Chan():
			m.Stream = string(e)
			if err := enc.Encode(m); err != nil {
				stderr.Write([]byte(err.Error()))
			}
			flush()
		case e := <-reporter.Chan():
			m.Stream = string(e)
			if err := enc.Encode(m); err != nil {
				stderr.Write([]byte(err.Error()))
			}
			flush()
		case e := <-stderr.Chan():
			failed = true
			m.Error = string(e)
			if err := enc.Encode(m); err != nil {
				logrus.Warnf("Failed to json encode error %v", err)
			}
			flush()
		case <-runCtx.Done():
			if !failed {
				if !utils.IsLibpodRequest(r) {
					m.Stream = fmt.Sprintf("Successfully built %12.12s\n", imageID)
					if err := enc.Encode(m); err != nil {
						logrus.Warnf("Failed to json encode error %v", err)
					}
					flush()
				}
			}
			break loop
		}
	}
}

func extractTarFile(r *http.Request) (string, error) {
	// build a home for the request body
	anchorDir, err := ioutil.TempDir("", "libpod_builder")
	if err != nil {
		return "", err
	}

	path := filepath.Join(anchorDir, "tarBall")
	tarBall, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer tarBall.Close()

	// Content-Length not used as too many existing API clients didn't honor it
	_, err = io.Copy(tarBall, r.Body)
	r.Body.Close()
	if err != nil {
		return "", fmt.Errorf("failed Request: Unable to copy tar file from request body %s", r.RequestURI)
	}

	buildDir := filepath.Join(anchorDir, "build")
	err = os.Mkdir(buildDir, 0700)
	if err != nil {
		return "", err
	}

	_, _ = tarBall.Seek(0, 0)
	err = archive.Untar(tarBall, buildDir, nil)
	return buildDir, err
}
