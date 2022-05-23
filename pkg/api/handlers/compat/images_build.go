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
	"strings"
	"time"

	"github.com/containers/buildah"
	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/api/handlers/utils"
	api "github.com/containers/podman/v4/pkg/api/types"
	"github.com/containers/podman/v4/pkg/auth"
	"github.com/containers/podman/v4/pkg/channel"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/gorilla/schema"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func BuildImage(w http.ResponseWriter, r *http.Request) {
	if hdr, found := r.Header["Content-Type"]; found && len(hdr) > 0 {
		contentType := hdr[0]
		switch contentType {
		case "application/tar":
			logrus.Infof("tar file content type is  %s, should use \"application/x-tar\" content type", contentType)
		case "application/x-tar":
			break
		default:
			if utils.IsLibpodRequest(r) {
				utils.BadRequest(w, "Content-Type", hdr[0],
					fmt.Errorf("Content-Type: %s is not supported. Should be \"application/x-tar\"", hdr[0]))
				return
			}
			logrus.Infof("tar file content type is  %s, should use \"application/x-tar\" content type", contentType)
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
		AddHosts               string   `schema:"extrahosts"`
		AdditionalCapabilities string   `schema:"addcaps"`
		AllPlatforms           bool     `schema:"allplatforms"`
		Annotations            string   `schema:"annotations"`
		AppArmor               string   `schema:"apparmor"`
		BuildArgs              string   `schema:"buildargs"`
		CacheFrom              string   `schema:"cachefrom"`
		CgroupParent           string   `schema:"cgroupparent"` // nolint
		Compression            uint64   `schema:"compression"`
		ConfigureNetwork       string   `schema:"networkmode"`
		CpuPeriod              uint64   `schema:"cpuperiod"`  // nolint
		CpuQuota               int64    `schema:"cpuquota"`   // nolint
		CpuSetCpus             string   `schema:"cpusetcpus"` // nolint
		CpuSetMems             string   `schema:"cpusetmems"` // nolint
		CpuShares              uint64   `schema:"cpushares"`  // nolint
		DNSOptions             string   `schema:"dnsoptions"`
		DNSSearch              string   `schema:"dnssearch"`
		DNSServers             string   `schema:"dnsservers"`
		Devices                string   `schema:"devices"`
		Dockerfile             string   `schema:"dockerfile"`
		DropCapabilities       string   `schema:"dropcaps"`
		Envs                   []string `schema:"setenv"`
		Excludes               string   `schema:"excludes"`
		ForceRm                bool     `schema:"forcerm"`
		From                   string   `schema:"from"`
		HTTPProxy              bool     `schema:"httpproxy"`
		IdentityLabel          bool     `schema:"identitylabel"`
		Ignore                 bool     `schema:"ignore"`
		Isolation              string   `schema:"isolation"`
		Jobs                   int      `schema:"jobs"` // nolint
		LabelOpts              string   `schema:"labelopts"`
		Labels                 string   `schema:"labels"`
		Layers                 bool     `schema:"layers"`
		LogRusage              bool     `schema:"rusage"`
		Manifest               string   `schema:"manifest"`
		MemSwap                int64    `schema:"memswap"`
		Memory                 int64    `schema:"memory"`
		NamespaceOptions       string   `schema:"nsoptions"`
		NoCache                bool     `schema:"nocache"`
		OSFeatures             []string `schema:"osfeature"`
		OSVersion              string   `schema:"osversion"`
		OutputFormat           string   `schema:"outputformat"`
		Platform               []string `schema:"platform"`
		Pull                   bool     `schema:"pull"`
		PullPolicy             string   `schema:"pullpolicy"`
		Quiet                  bool     `schema:"q"`
		Registry               string   `schema:"registry"`
		Rm                     bool     `schema:"rm"`
		RusageLogFile          string   `schema:"rusagelogfile"`
		Seccomp                string   `schema:"seccomp"`
		Secrets                string   `schema:"secrets"`
		SecurityOpt            string   `schema:"securityopt"`
		ShmSize                int      `schema:"shmsize"`
		Squash                 bool     `schema:"squash"`
		TLSVerify              bool     `schema:"tlsVerify"`
		Tags                   []string `schema:"t"`
		Target                 string   `schema:"target"`
		Timestamp              int64    `schema:"timestamp"`
		Ulimits                string   `schema:"ulimits"`
		UnsetEnvs              []string `schema:"unsetenv"`
	}{
		Dockerfile:    "Dockerfile",
		IdentityLabel: true,
		Registry:      "docker.io",
		Rm:            true,
		ShmSize:       64 * 1024 * 1024,
	}

	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	// if layers field not set assume its not from a valid podman-client
	// could be a docker client, set `layers=true` since that is the default
	// expected behaviour
	if !utils.IsLibpodRequest(r) {
		if _, found := r.URL.Query()["layers"]; !found {
			query.Layers = true
		}
	}

	// convert tag formats
	tags := query.Tags

	// convert addcaps formats
	var addCaps = []string{}
	if _, found := r.URL.Query()["addcaps"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.AdditionalCapabilities), &m); err != nil {
			utils.BadRequest(w, "addcaps", query.AdditionalCapabilities, err)
			return
		}
		addCaps = m
	}

	// convert addcaps formats
	containerFiles := []string{}
	if _, found := r.URL.Query()["dockerfile"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.Dockerfile), &m); err != nil {
			// it's not json, assume just a string
			m = []string{filepath.Join(contextDirectory, query.Dockerfile)}
		}
		containerFiles = m
	} else {
		containerFiles = []string{filepath.Join(contextDirectory, "Dockerfile")}
		if utils.IsLibpodRequest(r) {
			containerFiles = []string{filepath.Join(contextDirectory, "Containerfile")}
			if _, err = os.Stat(containerFiles[0]); err != nil {
				containerFiles = []string{filepath.Join(contextDirectory, "Dockerfile")}
				if _, err1 := os.Stat(containerFiles[0]); err1 != nil {
					utils.BadRequest(w, "dockerfile", query.Dockerfile, err)
				}
			}
		}
	}

	addhosts := []string{}
	if _, found := r.URL.Query()["extrahosts"]; found {
		if err := json.Unmarshal([]byte(query.AddHosts), &addhosts); err != nil {
			utils.BadRequest(w, "extrahosts", query.AddHosts, err)
			return
		}
	}

	compression := archive.Compression(query.Compression)

	// convert dropcaps formats
	var dropCaps = []string{}
	if _, found := r.URL.Query()["dropcaps"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.DropCapabilities), &m); err != nil {
			utils.BadRequest(w, "dropcaps", query.DropCapabilities, err)
			return
		}
		dropCaps = m
	}

	// convert devices formats
	var devices = []string{}
	if _, found := r.URL.Query()["devices"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.Devices), &m); err != nil {
			utils.BadRequest(w, "devices", query.Devices, err)
			return
		}
		devices = m
	}

	var dnsservers = []string{}
	if _, found := r.URL.Query()["dnsservers"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.DNSServers), &m); err != nil {
			utils.BadRequest(w, "dnsservers", query.DNSServers, err)
			return
		}
		dnsservers = m
	}

	var dnsoptions = []string{}
	if _, found := r.URL.Query()["dnsoptions"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.DNSOptions), &m); err != nil {
			utils.BadRequest(w, "dnsoptions", query.DNSOptions, err)
			return
		}
		dnsoptions = m
	}

	var dnssearch = []string{}
	if _, found := r.URL.Query()["dnssearch"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.DNSSearch), &m); err != nil {
			utils.BadRequest(w, "dnssearches", query.DNSSearch, err)
			return
		}
		dnssearch = m
	}

	var secrets = []string{}
	if _, found := r.URL.Query()["secrets"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.Secrets), &m); err != nil {
			utils.BadRequest(w, "secrets", query.Secrets, err)
			return
		}

		// for podman-remote all secrets must be picked from context director
		// hence modify src so contextdir is added as prefix

		for _, secret := range m {
			secretOpt := strings.Split(secret, ",")
			if len(secretOpt) > 0 {
				modifiedOpt := []string{}
				for _, token := range secretOpt {
					arr := strings.SplitN(token, "=", 2)
					if len(arr) > 1 {
						if arr[0] == "src" {
							/* move secret away from contextDir */
							/* to make sure we dont accidentally commit temporary secrets to image*/
							builderDirectory, _ := filepath.Split(contextDirectory)
							// following path is outside build context
							newSecretPath := filepath.Join(builderDirectory, arr[1])
							oldSecretPath := filepath.Join(contextDirectory, arr[1])
							err := os.Rename(oldSecretPath, newSecretPath)
							if err != nil {
								utils.BadRequest(w, "secrets", query.Secrets, err)
								return
							}

							modifiedSrc := fmt.Sprintf("src=%s", newSecretPath)
							modifiedOpt = append(modifiedOpt, modifiedSrc)
						} else {
							modifiedOpt = append(modifiedOpt, token)
						}
					}
				}
				secrets = append(secrets, strings.Join(modifiedOpt, ","))
			}
		}
	}

	var output string
	if len(tags) > 0 {
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, tags[0])
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error normalizing image"))
			return
		}
		output = possiblyNormalizedName
	}
	format := buildah.Dockerv2ImageManifest
	registry := query.Registry
	isolation := buildah.IsolationDefault
	if utils.IsLibpodRequest(r) {
		var err error
		isolation, err = parseLibPodIsolation(query.Isolation)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "failed to parse isolation"))
			return
		}

		// make sure to force rootless as rootless otherwise buildah runs code which is intended to be run only as root.
		if isolation == buildah.IsolationOCI && rootless.IsRootless() {
			isolation = buildah.IsolationOCIRootless
		}
		registry = ""
		format = query.OutputFormat
	} else {
		if _, found := r.URL.Query()["isolation"]; found {
			if query.Isolation != "" && query.Isolation != "default" {
				logrus.Debugf("invalid `isolation` parameter: %q", query.Isolation)
			}
		}
	}
	var additionalTags []string // nolint
	for i := 1; i < len(tags); i++ {
		possiblyNormalizedTag, err := utils.NormalizeToDockerHub(r, tags[i])
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error normalizing image"))
			return
		}
		additionalTags = append(additionalTags, possiblyNormalizedTag)
	}

	var buildArgs = map[string]string{}
	if _, found := r.URL.Query()["buildargs"]; found {
		if err := json.Unmarshal([]byte(query.BuildArgs), &buildArgs); err != nil {
			utils.BadRequest(w, "buildargs", query.BuildArgs, err)
			return
		}
	}

	var excludes = []string{}
	if _, found := r.URL.Query()["excludes"]; found {
		if err := json.Unmarshal([]byte(query.Excludes), &excludes); err != nil {
			utils.BadRequest(w, "excludes", query.Excludes, err)
			return
		}
	}

	// convert annotations formats
	var annotations = []string{}
	if _, found := r.URL.Query()["annotations"]; found {
		if err := json.Unmarshal([]byte(query.Annotations), &annotations); err != nil {
			utils.BadRequest(w, "annotations", query.Annotations, err)
			return
		}
	}

	// convert nsoptions formats
	nsoptions := buildah.NamespaceOptions{}
	if _, found := r.URL.Query()["nsoptions"]; found {
		if err := json.Unmarshal([]byte(query.NamespaceOptions), &nsoptions); err != nil {
			utils.BadRequest(w, "nsoptions", query.NamespaceOptions, err)
			return
		}
	} else {
		nsoptions = append(nsoptions, buildah.NamespaceOption{
			Name: string(specs.NetworkNamespace),
			Host: true,
		})
	}
	// convert label formats
	var labels = []string{}
	if _, found := r.URL.Query()["labels"]; found {
		makeLabels := make(map[string]string)
		err := json.Unmarshal([]byte(query.Labels), &makeLabels)
		if err == nil {
			for k, v := range makeLabels {
				labels = append(labels, k+"="+v)
			}
		} else {
			if err := json.Unmarshal([]byte(query.Labels), &labels); err != nil {
				utils.BadRequest(w, "labels", query.Labels, err)
				return
			}
		}
	}

	jobs := 1
	if _, found := r.URL.Query()["jobs"]; found {
		jobs = query.Jobs
	}

	var (
		labelOpts = []string{}
		seccomp   string
		apparmor  string
	)

	if utils.IsLibpodRequest(r) {
		seccomp = query.Seccomp
		apparmor = query.AppArmor
		// convert labelopts formats
		if _, found := r.URL.Query()["labelopts"]; found {
			var m = []string{}
			if err := json.Unmarshal([]byte(query.LabelOpts), &m); err != nil {
				utils.BadRequest(w, "labelopts", query.LabelOpts, err)
				return
			}
			labelOpts = m
		}
	} else {
		// handle security-opt
		if _, found := r.URL.Query()["securityopt"]; found {
			var securityOpts = []string{}
			if err := json.Unmarshal([]byte(query.SecurityOpt), &securityOpts); err != nil {
				utils.BadRequest(w, "securityopt", query.SecurityOpt, err)
				return
			}
			for _, opt := range securityOpts {
				if opt == "no-new-privileges" {
					utils.BadRequest(w, "securityopt", query.SecurityOpt, errors.New("no-new-privileges is not supported"))
					return
				}
				con := strings.SplitN(opt, "=", 2)
				if len(con) != 2 {
					utils.BadRequest(w, "securityopt", query.SecurityOpt, errors.Errorf("Invalid --security-opt name=value pair: %q", opt))
					return
				}

				switch con[0] {
				case "label":
					labelOpts = append(labelOpts, con[1])
				case "apparmor":
					apparmor = con[1]
				case "seccomp":
					seccomp = con[1]
				default:
					utils.BadRequest(w, "securityopt", query.SecurityOpt, errors.Errorf("Invalid --security-opt 2: %q", opt))
					return
				}
			}
		}
	}

	// convert ulimits formats
	var ulimits = []string{}
	if _, found := r.URL.Query()["ulimits"]; found {
		var m = []string{}
		if err := json.Unmarshal([]byte(query.Ulimits), &m); err != nil {
			utils.BadRequest(w, "ulimits", query.Ulimits, err)
			return
		}
		ulimits = m
	}

	pullPolicy := buildahDefine.PullIfMissing
	if utils.IsLibpodRequest(r) {
		pullPolicy = buildahDefine.PolicyMap[query.PullPolicy]
	} else {
		if _, found := r.URL.Query()["pull"]; found {
			if query.Pull {
				pullPolicy = buildahDefine.PullAlways
			}
		}
	}

	creds, authfile, err := auth.GetCredentials(r)
	if err != nil {
		// Credential value(s) not returned as their value is not human readable
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	fromImage := query.From
	if fromImage != "" {
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, fromImage)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, errors.Wrap(err, "error normalizing image"))
			return
		}
		fromImage = possiblyNormalizedName
	}

	systemContext := &types.SystemContext{
		AuthFilePath:     authfile,
		DockerAuthConfig: creds,
	}
	utils.PossiblyEnforceDockerHub(r, systemContext)

	if _, found := r.URL.Query()["tlsVerify"]; found {
		systemContext.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
		systemContext.OCIInsecureSkipTLSVerify = !query.TLSVerify
		systemContext.DockerDaemonInsecureSkipTLSVerify = !query.TLSVerify
	}
	// Channels all mux'ed in select{} below to follow API build protocol
	stdout := channel.NewWriter(make(chan []byte))
	defer stdout.Close()

	auxout := channel.NewWriter(make(chan []byte))
	defer auxout.Close()

	stderr := channel.NewWriter(make(chan []byte))
	defer stderr.Close()

	reporter := channel.NewWriter(make(chan []byte))
	defer reporter.Close()

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	buildOptions := buildahDefine.BuildOptions{
		AddCapabilities: addCaps,
		AdditionalTags:  additionalTags,
		Annotations:     annotations,
		Args:            buildArgs,
		AllPlatforms:    query.AllPlatforms,
		CommonBuildOpts: &buildah.CommonBuildOptions{
			AddHost:            addhosts,
			ApparmorProfile:    apparmor,
			CPUPeriod:          query.CpuPeriod,
			CPUQuota:           query.CpuQuota,
			CPUSetCPUs:         query.CpuSetCpus,
			CPUSetMems:         query.CpuSetMems,
			CPUShares:          query.CpuShares,
			CgroupParent:       query.CgroupParent,
			DNSOptions:         dnsoptions,
			DNSSearch:          dnssearch,
			DNSServers:         dnsservers,
			HTTPProxy:          query.HTTPProxy,
			IdentityLabel:      types.NewOptionalBool(query.IdentityLabel),
			LabelOpts:          labelOpts,
			Memory:             query.Memory,
			MemorySwap:         query.MemSwap,
			SeccompProfilePath: seccomp,
			ShmSize:            strconv.Itoa(query.ShmSize),
			Ulimit:             ulimits,
			Secrets:            secrets,
		},
		Compression:                    compression,
		ConfigureNetwork:               parseNetworkConfigurationPolicy(query.ConfigureNetwork),
		ContextDirectory:               contextDirectory,
		Devices:                        devices,
		DropCapabilities:               dropCaps,
		Envs:                           query.Envs,
		Err:                            auxout,
		Excludes:                       excludes,
		ForceRmIntermediateCtrs:        query.ForceRm,
		From:                           fromImage,
		IgnoreUnrecognizedInstructions: query.Ignore,
		Isolation:                      isolation,
		Jobs:                           &jobs,
		Labels:                         labels,
		Layers:                         query.Layers,
		LogRusage:                      query.LogRusage,
		Manifest:                       query.Manifest,
		MaxPullPushRetries:             3,
		NamespaceOptions:               nsoptions,
		NoCache:                        query.NoCache,
		OSFeatures:                     query.OSFeatures,
		OSVersion:                      query.OSVersion,
		Out:                            stdout,
		Output:                         output,
		OutputFormat:                   format,
		PullPolicy:                     pullPolicy,
		PullPushRetryDelay:             2 * time.Second,
		Quiet:                          query.Quiet,
		Registry:                       registry,
		RemoveIntermediateCtrs:         query.Rm,
		ReportWriter:                   reporter,
		RusageLogFile:                  query.RusageLogFile,
		Squash:                         query.Squash,
		SystemContext:                  systemContext,
		Target:                         query.Target,
		UnsetEnvs:                      query.UnsetEnvs,
	}

	for _, platformSpec := range query.Platform {
		os, arch, variant, err := parse.Platform(platformSpec)
		if err != nil {
			utils.BadRequest(w, "platform", platformSpec, err)
			return
		}
		buildOptions.Platforms = append(buildOptions.Platforms, struct{ OS, Arch, Variant string }{
			OS:      os,
			Arch:    arch,
			Variant: variant,
		})
	}
	if _, found := r.URL.Query()["timestamp"]; found {
		ts := time.Unix(query.Timestamp, 0)
		buildOptions.Timestamp = &ts
	}

	var (
		imageID string
		success bool
	)

	runCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		imageID, _, err = runtime.Build(r.Context(), buildOptions, containerFiles...)
		if err == nil {
			success = true
		} else {
			stderr.Write([]byte(err.Error() + "\n"))
		}
	}()

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	// Send headers and prime client for stream to come
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	flush()

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
	var stepErrors []string

	for {
		type BuildResponse struct {
			Stream string                 `json:"stream,omitempty"`
			Error  *jsonmessage.JSONError `json:"errorDetail,omitempty"`
			// NOTE: `error` is being deprecated check https://github.com/moby/moby/blob/master/pkg/jsonmessage/jsonmessage.go#L148
			ErrorMessage string          `json:"error,omitempty"` // deprecate this slowly
			Aux          json.RawMessage `json:"aux,omitempty"`
		}
		m := BuildResponse{}

		select {
		case e := <-stdout.Chan():
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
		case e := <-auxout.Chan():
			if !query.Quiet {
				m.Stream = string(e)
				if err := enc.Encode(m); err != nil {
					stderr.Write([]byte(err.Error()))
				}
				flush()
			} else {
				stepErrors = append(stepErrors, string(e))
			}
		case e := <-stderr.Chan():
			// Docker-API Compat parity : Build failed so
			// output all step errors irrespective of quiet
			// flag.
			for _, stepError := range stepErrors {
				t := BuildResponse{}
				t.Stream = stepError
				if err := enc.Encode(t); err != nil {
					stderr.Write([]byte(err.Error()))
				}
				flush()
			}
			m.ErrorMessage = string(e)
			m.Error = &jsonmessage.JSONError{
				Message: m.ErrorMessage,
			}
			if err := enc.Encode(m); err != nil {
				logrus.Warnf("Failed to json encode error %v", err)
			}
			flush()
			return
		case <-runCtx.Done():
			if success {
				if !utils.IsLibpodRequest(r) && !query.Quiet {
					m.Aux = []byte(fmt.Sprintf(`{"ID":"sha256:%s"}`, imageID))
					if err := enc.Encode(m); err != nil {
						logrus.Warnf("failed to json encode error %v", err)
					}
					m.Aux = nil
					m.Stream = fmt.Sprintf("Successfully built %12.12s\n", imageID)
					if err := enc.Encode(m); err != nil {
						logrus.Warnf("Failed to json encode error %v", err)
					}
					flush()
					for _, tag := range tags {
						m.Stream = fmt.Sprintf("Successfully tagged %s\n", tag)
						if err := enc.Encode(m); err != nil {
							logrus.Warnf("Failed to json encode error %v", err)
						}
						flush()
					}
				}
			}
			flush()
			return
		case <-r.Context().Done():
			cancel()
			logrus.Infof("Client disconnect reported for build %q / %q.", registry, query.Dockerfile)
			return
		}
	}
}

func parseNetworkConfigurationPolicy(network string) buildah.NetworkConfigurationPolicy {
	if val, err := strconv.Atoi(network); err == nil {
		return buildah.NetworkConfigurationPolicy(val)
	}
	switch network {
	case "NetworkDefault":
		return buildah.NetworkDefault
	case "NetworkDisabled":
		return buildah.NetworkDisabled
	case "NetworkEnabled":
		return buildah.NetworkEnabled
	default:
		return buildah.NetworkDefault
	}
}

func parseLibPodIsolation(isolation string) (buildah.Isolation, error) { // nolint
	if val, err := strconv.Atoi(isolation); err == nil {
		return buildah.Isolation(val), nil
	}
	return parse.IsolationOption(isolation)
}

func extractTarFile(r *http.Request) (string, error) {
	// build a home for the request body
	anchorDir, err := ioutil.TempDir("", "libpod_builder")
	if err != nil {
		return "", err
	}

	path := filepath.Join(anchorDir, "tarBall")
	tarBall, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
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
	err = os.Mkdir(buildDir, 0o700)
	if err != nil {
		return "", err
	}

	_, _ = tarBall.Seek(0, 0)
	err = archive.Untar(tarBall, buildDir, nil)
	return buildDir, err
}
