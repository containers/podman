package compat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/podman/v2/pkg/channel"
	"github.com/containers/storage/pkg/archive"
	"github.com/gorilla/schema"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func BuildImage(w http.ResponseWriter, r *http.Request) {
	authConfigs := map[string]handlers.AuthConfig{}
	if hdr, found := r.Header["X-Registry-Config"]; found && len(hdr) > 0 {
		authConfigsJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(hdr[0]))
		if json.NewDecoder(authConfigsJSON).Decode(&authConfigs) != nil {
			utils.BadRequest(w, "X-Registry-Config", hdr[0], json.NewDecoder(authConfigsJSON).Decode(&authConfigs))
			return
		}
	}

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
		Dockerfile  string   `schema:"dockerfile"`
		Tag         []string `schema:"t"`
		ExtraHosts  string   `schema:"extrahosts"`
		Remote      string   `schema:"remote"`
		Quiet       bool     `schema:"q"`
		NoCache     bool     `schema:"nocache"`
		CacheFrom   string   `schema:"cachefrom"`
		Pull        bool     `schema:"pull"`
		Rm          bool     `schema:"rm"`
		ForceRm     bool     `schema:"forcerm"`
		Memory      int64    `schema:"memory"`
		MemSwap     int64    `schema:"memswap"`
		CpuShares   uint64   `schema:"cpushares"`  // nolint
		CpuSetCpus  string   `schema:"cpusetcpus"` // nolint
		CpuPeriod   uint64   `schema:"cpuperiod"`  // nolint
		CpuQuota    int64    `schema:"cpuquota"`   // nolint
		BuildArgs   string   `schema:"buildargs"`
		ShmSize     int      `schema:"shmsize"`
		Squash      bool     `schema:"squash"`
		Labels      string   `schema:"labels"`
		NetworkMode string   `schema:"networkmode"`
		Platform    string   `schema:"platform"`
		Target      string   `schema:"target"`
		Outputs     string   `schema:"outputs"`
		Registry    string   `schema:"registry"`
	}{
		Dockerfile: "Dockerfile",
		Tag:        []string{},
		Rm:         true,
		ShmSize:    64 * 1024 * 1024,
		Registry:   "docker.io",
	}

	decoder := r.Context().Value("decoder").(*schema.Decoder)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, err)
		return
	}

	var output string
	if len(query.Tag) > 0 {
		output = query.Tag[0]
	}
	if _, found := r.URL.Query()["target"]; found {
		output = query.Target
	}

	var additionalNames []string
	if len(query.Tag) > 1 {
		additionalNames = query.Tag[1:]
	}

	var buildArgs = map[string]string{}
	if _, found := r.URL.Query()["buildargs"]; found {
		if err := json.Unmarshal([]byte(query.BuildArgs), &buildArgs); err != nil {
			utils.BadRequest(w, "buildargs", query.BuildArgs, err)
			return
		}
	}

	// convert label formats
	var labels = []string{}
	if _, found := r.URL.Query()["labels"]; found {
		var m = map[string]string{}
		if err := json.Unmarshal([]byte(query.Labels), &m); err != nil {
			utils.BadRequest(w, "labels", query.Labels, err)
			return
		}

		for k, v := range m {
			labels = append(labels, k+"="+v)
		}
	}

	pullPolicy := buildah.PullIfMissing
	if _, found := r.URL.Query()["pull"]; found {
		if query.Pull {
			pullPolicy = buildah.PullAlways
		}
	}

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
		ContextDirectory:               contextDirectory,
		PullPolicy:                     pullPolicy,
		Registry:                       query.Registry,
		IgnoreUnrecognizedInstructions: true,
		Quiet:                          query.Quiet,
		Isolation:                      buildah.IsolationChroot,
		Compression:                    archive.Gzip,
		Args:                           buildArgs,
		Output:                         output,
		AdditionalTags:                 additionalNames,
		Out:                            stdout,
		Err:                            auxout,
		ReportWriter:                   reporter,
		OutputFormat:                   buildah.Dockerv2ImageManifest,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			CPUPeriod:  query.CpuPeriod,
			CPUQuota:   query.CpuQuota,
			CPUShares:  query.CpuShares,
			CPUSetCPUs: query.CpuSetCpus,
			Memory:     query.Memory,
			MemorySwap: query.MemSwap,
			ShmSize:    strconv.Itoa(query.ShmSize),
		},
		Squash:                  query.Squash,
		Labels:                  labels,
		NoCache:                 query.NoCache,
		RemoveIntermediateCtrs:  query.Rm,
		ForceRmIntermediateCtrs: query.ForceRm,
		Target:                  query.Target,
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
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
		case <-runCtx.Done():
			if !failed {
				if utils.IsLibpodRequest(r) {
					m.Stream = imageID
				} else {
					m.Stream = fmt.Sprintf("Successfully built %12.12s\n", imageID)
				}
				if err := enc.Encode(m); err != nil {
					logrus.Warnf("Failed to json encode error %q", err.Error())
				}
				flush()
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
