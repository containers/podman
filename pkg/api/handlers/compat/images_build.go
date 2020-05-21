package compat

import (
	"bytes"
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
	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/api/handlers"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/storage/pkg/archive"
	"github.com/gorilla/schema"
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
		if hdr[0] != "application/x-tar" {
			utils.BadRequest(w, "Content-Type", hdr[0],
				fmt.Errorf("Content-Type: %s is not supported. Should be \"application/x-tar\"", hdr[0]))
		}
	}

	anchorDir, err := extractTarFile(r, w)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	defer os.RemoveAll(anchorDir)

	query := struct {
		Dockerfile  string `schema:"dockerfile"`
		Tag         string `schema:"t"`
		ExtraHosts  string `schema:"extrahosts"`
		Remote      string `schema:"remote"`
		Quiet       bool   `schema:"q"`
		NoCache     bool   `schema:"nocache"`
		CacheFrom   string `schema:"cachefrom"`
		Pull        bool   `schema:"pull"`
		Rm          bool   `schema:"rm"`
		ForceRm     bool   `schema:"forcerm"`
		Memory      int64  `schema:"memory"`
		MemSwap     int64  `schema:"memswap"`
		CpuShares   uint64 `schema:"cpushares"`
		CpuSetCpus  string `schema:"cpusetcpus"`
		CpuPeriod   uint64 `schema:"cpuperiod"`
		CpuQuota    int64  `schema:"cpuquota"`
		BuildArgs   string `schema:"buildargs"`
		ShmSize     int    `schema:"shmsize"`
		Squash      bool   `schema:"squash"`
		Labels      string `schema:"labels"`
		NetworkMode string `schema:"networkmode"`
		Platform    string `schema:"platform"`
		Target      string `schema:"target"`
		Outputs     string `schema:"outputs"`
		Registry    string `schema:"registry"`
	}{
		Dockerfile:  "Dockerfile",
		Tag:         "",
		ExtraHosts:  "",
		Remote:      "",
		Quiet:       false,
		NoCache:     false,
		CacheFrom:   "",
		Pull:        false,
		Rm:          true,
		ForceRm:     false,
		Memory:      0,
		MemSwap:     0,
		CpuShares:   0,
		CpuSetCpus:  "",
		CpuPeriod:   0,
		CpuQuota:    0,
		BuildArgs:   "",
		ShmSize:     64 * 1024 * 1024,
		Squash:      false,
		Labels:      "",
		NetworkMode: "",
		Platform:    "",
		Target:      "",
		Outputs:     "",
		Registry:    "docker.io",
	}
	decoder := r.Context().Value("decoder").(*schema.Decoder)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, err)
		return
	}

	var (
		// Tag is the name with optional tag...
		name = query.Tag
		tag  = "latest"
	)
	if strings.Contains(query.Tag, ":") {
		tokens := strings.SplitN(query.Tag, ":", 2)
		name = tokens[0]
		tag = tokens[1]
	}

	if _, found := r.URL.Query()["target"]; found {
		name = query.Target
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

	// build events will be recorded here
	var (
		buildEvents = []string{}
		progress    = bytes.Buffer{}
	)

	buildOptions := imagebuildah.BuildOptions{
		ContextDirectory:               filepath.Join(anchorDir, "build"),
		PullPolicy:                     pullPolicy,
		Registry:                       query.Registry,
		IgnoreUnrecognizedInstructions: true,
		Quiet:                          query.Quiet,
		Isolation:                      buildah.IsolationChroot,
		Runtime:                        "",
		RuntimeArgs:                    nil,
		TransientMounts:                nil,
		Compression:                    archive.Gzip,
		Args:                           buildArgs,
		Output:                         name,
		AdditionalTags:                 []string{tag},
		Log: func(format string, args ...interface{}) {
			buildEvents = append(buildEvents, fmt.Sprintf(format, args...))
		},
		In:                  nil,
		Out:                 &progress,
		Err:                 &progress,
		SignaturePolicyPath: "",
		ReportWriter:        &progress,
		OutputFormat:        buildah.Dockerv2ImageManifest,
		SystemContext:       nil,
		NamespaceOptions:    nil,
		ConfigureNetwork:    0,
		CNIPluginPath:       "",
		CNIConfigDir:        "",
		IDMappingOptions:    nil,
		AddCapabilities:     nil,
		DropCapabilities:    nil,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			AddHost:            nil,
			CgroupParent:       "",
			CPUPeriod:          query.CpuPeriod,
			CPUQuota:           query.CpuQuota,
			CPUShares:          query.CpuShares,
			CPUSetCPUs:         query.CpuSetCpus,
			CPUSetMems:         "",
			HTTPProxy:          false,
			Memory:             query.Memory,
			DNSSearch:          nil,
			DNSServers:         nil,
			DNSOptions:         nil,
			MemorySwap:         query.MemSwap,
			LabelOpts:          nil,
			SeccompProfilePath: "",
			ApparmorProfile:    "",
			ShmSize:            strconv.Itoa(query.ShmSize),
			Ulimit:             nil,
			Volumes:            nil,
		},
		DefaultMountsFilePath:   "",
		IIDFile:                 "",
		Squash:                  query.Squash,
		Labels:                  labels,
		Annotations:             nil,
		OnBuild:                 nil,
		Layers:                  false,
		NoCache:                 query.NoCache,
		RemoveIntermediateCtrs:  query.Rm,
		ForceRmIntermediateCtrs: query.ForceRm,
		BlobDirectory:           "",
		Target:                  query.Target,
		Devices:                 nil,
	}

	runtime := r.Context().Value("runtime").(*libpod.Runtime)
	id, _, err := runtime.Build(r.Context(), buildOptions, query.Dockerfile)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Find image ID that was built...
	utils.WriteResponse(w, http.StatusOK,
		struct {
			Stream string `json:"stream"`
		}{
			Stream: progress.String() + "\n" +
				strings.Join(buildEvents, "\n") +
				fmt.Sprintf("\nSuccessfully built %s\n", id),
		})
}

func extractTarFile(r *http.Request, w http.ResponseWriter) (string, error) {
	// build a home for the request body
	anchorDir, err := ioutil.TempDir("", "libpod_builder")
	if err != nil {
		return "", err
	}
	buildDir := filepath.Join(anchorDir, "build")

	path := filepath.Join(anchorDir, "tarBall")
	tarBall, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return "", err
	}
	defer tarBall.Close()

	// Content-Length not used as too many existing API clients didn't honor it
	_, err = io.Copy(tarBall, r.Body)
	r.Body.Close()

	if err != nil {
		utils.InternalServerError(w,
			fmt.Errorf("failed Request: Unable to copy tar file from request body %s", r.RequestURI))
	}

	_, _ = tarBall.Seek(0, 0)
	if err := archive.Untar(tarBall, buildDir, &archive.TarOptions{}); err != nil {
		return "", err
	}
	return anchorDir, nil
}
