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
	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/api/handlers/utils"
	"github.com/containers/storage/pkg/archive"
	"github.com/gorilla/schema"
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

	anchorDir, err := extractTarFile(r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	defer os.RemoveAll(anchorDir)

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
		CpuShares   uint64   `schema:"cpushares"`  //nolint
		CpuSetCpus  string   `schema:"cpusetcpus"` //nolint
		CpuPeriod   uint64   `schema:"cpuperiod"`  //nolint
		CpuQuota    int64    `schema:"cpuquota"`   //nolint
		BuildArgs   string   `schema:"buildargs"`
		ShmSize     int      `schema:"shmsize"`
		Squash      bool     `schema:"squash"`
		Labels      string   `schema:"labels"`
		NetworkMode string   `schema:"networkmode"`
		Platform    string   `schema:"platform"`
		Target      string   `schema:"target"`
		Outputs     string   `schema:"outputs"`
		Registry    string   `schema:"registry"`
		IIDFile     string   `schema:"iidfile"`
	}{
		Dockerfile:  "Dockerfile",
		Tag:         []string{},
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
		output          string
		additionalNames []string
	)
	if len(query.Tag) > 0 {
		output = query.Tag[0]
	}
	if len(query.Tag) > 1 {
		additionalNames = query.Tag[1:]
	}

	if _, found := r.URL.Query()["target"]; found {
		output = query.Target
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
		Output:                         output,
		AdditionalTags:                 additionalNames,
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
		IIDFile:                 query.IIDFile,
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
	stream := progress.String() + "\n" + strings.Join(buildEvents, "\n") + "\n"
	// Docker prepends a message to the ID that was built.  Podman does not
	if !utils.IsLibpodRequest(r) {
		stream += "Successfully built "
	}
	stream += fmt.Sprintf("%s\n", id)
	utils.WriteResponse(w, http.StatusOK,
		struct {
			Stream string `json:"stream"`
		}{
			Stream: stream,
		})
}

func extractTarFile(r *http.Request) (string, error) {
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
		return "", fmt.Errorf("failed Request: Unable to copy tar file from request body %s", r.RequestURI)
	}

	_, _ = tarBall.Seek(0, 0)
	if err := archive.Untar(tarBall, buildDir, &archive.TarOptions{}); err != nil {
		return "", err
	}
	return anchorDir, nil
}
