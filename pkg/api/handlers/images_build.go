package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/buildah"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/libpod/pkg/api/handlers/utils"
	"github.com/containers/storage/pkg/archive"
	log "github.com/sirupsen/logrus"
)

func BuildImage(w http.ResponseWriter, r *http.Request) {
	// contentType := r.Header.Get("Content-Type")
	// if contentType != "application/x-tar" {
	// 	Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, errors.New("/build expects Content-Type of 'application/x-tar'"))
	// 	return
	// }

	authConfigs := map[string]AuthConfig{}
	if hasHeader(r, "X-Registry-Config") {
		registryHeader := getHeader(r, "X-Registry-Config")
		authConfigsJSON := base64.NewDecoder(base64.URLEncoding, strings.NewReader(registryHeader))
		if json.NewDecoder(authConfigsJSON).Decode(&authConfigs) != nil {
			utils.BadRequest(w, "X-Registry-Config", registryHeader, json.NewDecoder(authConfigsJSON).Decode(&authConfigs))
			return
		}
	}

	anchorDir, err := extractTarFile(r, w)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	// defer os.RemoveAll(anchorDir)

	query := struct {
		Dockerfile  string `json:"dockerfile"`
		Tag         string `json:"t"`
		ExtraHosts  string `json:"extrahosts"`
		Remote      string `json:"remote"`
		Quiet       bool   `json:"q"`
		NoCache     bool   `json:"nocache"`
		CacheFrom   string `json:"cachefrom"`
		Pull        string `json:"pull"`
		Rm          bool   `json:"rm"`
		ForceRm     bool   `json:"forcerm"`
		Memory      int    `json:"memory"`
		MemSwap     int    `json:"memswap"`
		CpuShares   int    `json:"cpushares"`
		CpuSetCpus  string `json:"cpusetcpus"`
		CpuPeriod   int    `json:"cpuperiod"`
		CpuQuota    int    `json:"cpuquota"`
		BuildArgs   string `json:"buildargs"`
		ShmSize     int    `json:"shmsize"`
		Squash      bool   `json:"squash"`
		Labels      string `json:"labels"`
		NetworkMode string `json:"networkmode"`
		Platform    string `json:"platform"`
		Target      string `json:"target"`
		Outputs     string `json:"outputs"`
	}{
		Dockerfile:  "Dockerfile",
		Tag:         "",
		ExtraHosts:  "",
		Remote:      "",
		Quiet:       false,
		NoCache:     false,
		CacheFrom:   "",
		Pull:        "",
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
	}

	if err := decodeQuery(r, &query); err != nil {
		utils.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest, err)
		return
	}

	// Tag is the name with optional tag...
	var name = query.Tag
	var tag string
	if strings.Contains(query.Tag, ":") {
		tokens := strings.SplitN(query.Tag, ":", 2)
		name = tokens[0]
		tag = tokens[1]
	}

	var buildArgs = map[string]string{}
	if found := hasVar(r, "buildargs"); found {
		if err := json.Unmarshal([]byte(query.BuildArgs), &buildArgs); err != nil {
			utils.BadRequest(w, "buildargs", query.BuildArgs, err)
			return
		}
	}

	// convert label formats
	var labels = []string{}
	if hasVar(r, "labels") {
		var m = map[string]string{}
		if err := json.Unmarshal([]byte(query.Labels), &m); err != nil {
			utils.BadRequest(w, "labels", query.Labels, err)
			return
		}

		for k, v := range m {
			labels = append(labels, fmt.Sprintf("%s=%v", k, v))
		}
	}

	buildOptions := imagebuildah.BuildOptions{
		ContextDirectory:               filepath.Join(anchorDir, "build"),
		PullPolicy:                     0,
		Registry:                       "",
		IgnoreUnrecognizedInstructions: false,
		Quiet:                          query.Quiet,
		Isolation:                      0,
		Runtime:                        "",
		RuntimeArgs:                    nil,
		TransientMounts:                nil,
		Compression:                    0,
		Args:                           buildArgs,
		Output:                         name,
		AdditionalTags:                 []string{tag},
		Log:                            nil,
		In:                             nil,
		Out:                            nil,
		Err:                            nil,
		SignaturePolicyPath:            "",
		ReportWriter:                   nil,
		OutputFormat:                   "",
		SystemContext:                  nil,
		NamespaceOptions:               nil,
		ConfigureNetwork:               0,
		CNIPluginPath:                  "",
		CNIConfigDir:                   "",
		IDMappingOptions:               nil,
		AddCapabilities:                nil,
		DropCapabilities:               nil,
		CommonBuildOpts:                &buildah.CommonBuildOptions{},
		DefaultMountsFilePath:          "",
		IIDFile:                        "",
		Squash:                         query.Squash,
		Labels:                         labels,
		Annotations:                    nil,
		OnBuild:                        nil,
		Layers:                         false,
		NoCache:                        query.NoCache,
		RemoveIntermediateCtrs:         query.Rm,
		ForceRmIntermediateCtrs:        query.ForceRm,
		BlobDirectory:                  "",
		Target:                         query.Target,
		Devices:                        nil,
	}

	id, _, err := getRuntime(r).Build(r.Context(), buildOptions, query.Dockerfile)
	if err != nil {
		utils.InternalServerError(w, err)
	}

	// Find image ID that was built...
	utils.WriteResponse(w, http.StatusOK,
		struct {
			Stream string `json:"stream"`
		}{
			Stream: fmt.Sprintf("Successfully built %s\n", id),
		})
}

func extractTarFile(r *http.Request, w http.ResponseWriter) (string, error) {
	var (
		// length  int64
		// n       int64
		copyErr error
	)

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

	// if hasHeader(r, "Content-Length") {
	// 	length, err := strconv.ParseInt(getHeader(r, "Content-Length"), 10, 64)
	// 	if err != nil {
	// 		return "", errors.New(fmt.Sprintf("Failed request: unable to parse Content-Length of '%s'", getHeader(r, "Content-Length")))
	// 	}
	// 	n, copyErr = io.CopyN(tarBall, r.Body, length+1)
	// } else {
	_, copyErr = io.Copy(tarBall, r.Body)
	// }
	r.Body.Close()

	if copyErr != nil {
		utils.InternalServerError(w,
			fmt.Errorf("failed Request: Unable to copy tar file from request body %s", r.RequestURI))
	}
	log.Debugf("Content-Length: %s", getVar(r, "Content-Length"))

	// if hasHeader(r, "Content-Length") && n != length {
	// 	return "", errors.New(fmt.Sprintf("Failed request: Given Content-Length does not match file size %d != %d", n, length))
	// }

	_, _ = tarBall.Seek(0, 0)
	if err := archive.Untar(tarBall, buildDir, &archive.TarOptions{}); err != nil {
		return "", err
	}
	return anchorDir, nil
}
