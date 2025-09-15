//go:build !remote

package compat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/containers/buildah"
	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/libpod"
	"github.com/containers/podman/v5/pkg/api/handlers/utils"
	api "github.com/containers/podman/v5/pkg/api/types"
	"github.com/containers/podman/v5/pkg/auth"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/channel"
	"github.com/containers/podman/v5/pkg/rootless"
	"github.com/containers/podman/v5/pkg/util"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func genSpaceErr(err error) error {
	if errors.Is(err, syscall.ENOSPC) {
		return fmt.Errorf("context directory may be too large: %w", err)
	}
	return err
}

func BuildImage(w http.ResponseWriter, r *http.Request) {
	multipart := false
	if hdr, found := r.Header["Content-Type"]; found && len(hdr) > 0 {
		contentType, _, err := mime.ParseMediaType(hdr[0])
		if err != nil {
			utils.BadRequest(w, "Content-Type", hdr[0], fmt.Errorf("failed to parse content type: %w", err))
			return
		}

		switch contentType {
		case "application/tar":
			logrus.Infof("tar file content type is  %s, should use \"application/x-tar\" content type", contentType)
		case "application/x-tar":
			break
		case "multipart/form-data":
			logrus.Infof("Received %s", hdr[0])
			multipart = true
		default:
			if utils.IsLibpodRequest(r) {
				utils.BadRequest(w, "Content-Type", hdr[0],
					fmt.Errorf("Content-Type: %s is not supported. Should be \"application/x-tar\"", hdr[0]))
				return
			}
			logrus.Infof("tar file content type is  %s, should use \"application/x-tar\" content type", contentType)
		}
	}

	anchorDir, err := os.MkdirTemp("", "libpod_builder")
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
		err := os.RemoveAll(anchorDir)
		if err != nil {
			logrus.Warn(fmt.Errorf("failed to remove build scratch directory %q: %w", anchorDir, err))
		}
	}()

	contextDirectory, additionalBuildContexts, err := handleBuildContexts(anchorDir, r, multipart)
	if err != nil {
		utils.InternalServerError(w, genSpaceErr(err))
		return
	}

	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	conf, err := runtime.GetConfigNoCopy()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	query := struct {
		AddHosts                string             `schema:"extrahosts"`
		AdditionalCapabilities  string             `schema:"addcaps"`
		AdditionalBuildContexts string             `schema:"additionalbuildcontexts"`
		AllPlatforms            bool               `schema:"allplatforms"`
		Annotations             string             `schema:"annotations"`
		AppArmor                string             `schema:"apparmor"`
		BuildArgs               string             `schema:"buildargs"`
		CacheFrom               string             `schema:"cachefrom"`
		CacheTo                 string             `schema:"cacheto"`
		CacheTTL                string             `schema:"cachettl"`
		CgroupParent            string             `schema:"cgroupparent"`
		CompatVolumes           bool               `schema:"compatvolumes"`
		Compression             uint64             `schema:"compression"`
		ConfigureNetwork        string             `schema:"networkmode"`
		CPPFlags                string             `schema:"cppflags"`
		CpuPeriod               uint64             `schema:"cpuperiod"`
		CpuQuota                int64              `schema:"cpuquota"`
		CpuSetCpus              string             `schema:"cpusetcpus"`
		CpuSetMems              string             `schema:"cpusetmems"`
		CpuShares               uint64             `schema:"cpushares"`
		CreatedAnnotation       types.OptionalBool `schema:"createdannotation"`
		DNSOptions              string             `schema:"dnsoptions"`
		DNSSearch               string             `schema:"dnssearch"`
		DNSServers              string             `schema:"dnsservers"`
		Devices                 string             `schema:"devices"`
		Dockerfile              string             `schema:"dockerfile"`
		DropCapabilities        string             `schema:"dropcaps"`
		Envs                    []string           `schema:"setenv"`
		Excludes                string             `schema:"excludes"`
		ForceRm                 bool               `schema:"forcerm"`
		From                    string             `schema:"from"`
		GroupAdd                []string           `schema:"groupadd"`
		HTTPProxy               bool               `schema:"httpproxy"`
		IDMappingOptions        string             `schema:"idmappingoptions"`
		IdentityLabel           bool               `schema:"identitylabel"`
		Ignore                  bool               `schema:"ignore"`
		InheritLabels           types.OptionalBool `schema:"inheritlabels"`
		InheritAnnotations      types.OptionalBool `schema:"inheritannotations"`
		Isolation               string             `schema:"isolation"`
		Jobs                    int                `schema:"jobs"`
		LabelOpts               string             `schema:"labelopts"`
		Labels                  string             `schema:"labels"`
		LayerLabels             []string           `schema:"layerLabel"`
		Layers                  bool               `schema:"layers"`
		LogRusage               bool               `schema:"rusage"`
		Manifest                string             `schema:"manifest"`
		MemSwap                 int64              `schema:"memswap"`
		Memory                  int64              `schema:"memory"`
		NamespaceOptions        string             `schema:"nsoptions"`
		NoCache                 bool               `schema:"nocache"`
		NoHosts                 bool               `schema:"nohosts"`
		OmitHistory             bool               `schema:"omithistory"`
		OSFeatures              []string           `schema:"osfeature"`
		OSVersion               string             `schema:"osversion"`
		OutputFormat            string             `schema:"outputformat"`
		Platform                []string           `schema:"platform"`
		Pull                    bool               `schema:"pull"`
		PullPolicy              string             `schema:"pullpolicy"`
		Quiet                   bool               `schema:"q"`
		Registry                string             `schema:"registry"`
		Rm                      bool               `schema:"rm"`
		RusageLogFile           string             `schema:"rusagelogfile"`
		Remote                  string             `schema:"remote"`
		RewriteTimestamp        bool               `schema:"rewritetimestamp"`
		Retry                   int                `schema:"retry"`
		RetryDelay              string             `schema:"retry-delay"`
		Seccomp                 string             `schema:"seccomp"`
		Secrets                 string             `schema:"secrets"`
		SecurityOpt             string             `schema:"securityopt"`
		ShmSize                 int                `schema:"shmsize"`
		SkipUnusedStages        bool               `schema:"skipunusedstages"`
		SourceDateEpoch         int64              `schema:"sourcedateepoch"`
		Squash                  bool               `schema:"squash"`
		TLSVerify               bool               `schema:"tlsVerify"`
		Tags                    []string           `schema:"t"`
		Target                  string             `schema:"target"`
		Timestamp               int64              `schema:"timestamp"`
		Ulimits                 string             `schema:"ulimits"`
		UnsetEnvs               []string           `schema:"unsetenv"`
		UnsetLabels             []string           `schema:"unsetlabel"`
		UnsetAnnotations        []string           `schema:"unsetannotation"`
		Volumes                 []string           `schema:"volume"`
	}{
		Dockerfile: "Dockerfile",
		Registry:   "docker.io",
		Rm:         true,
		ShmSize:    64 * 1024 * 1024,
		TLSVerify:  true,
		Retry:      int(conf.Engine.Retry),
		RetryDelay: conf.Engine.RetryDelay,
	}

	decoder := utils.GetDecoder(r)
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	var identityLabel types.OptionalBool
	if _, found := r.URL.Query()["identitylabel"]; found {
		identityLabel = types.NewOptionalBool(query.IdentityLabel)
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
	// Tells if query parameter `dockerfile` is set or not.
	dockerFileSet := false
	if utils.IsLibpodRequest(r) && query.Remote != "" {
		// The context directory could be a URL.  Try to handle that.
		anchorDir, err := os.MkdirTemp(parse.GetTempDir(), "libpod_builder")
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		tempDir, subDir, err := buildahDefine.TempDirForURL(anchorDir, "buildah", query.Remote)
		if err != nil {
			utils.InternalServerError(w, genSpaceErr(err))
			return
		}
		if tempDir != "" {
			// We had to download it to a temporary directory.
			// Delete it later.
			defer func() {
				if err = os.RemoveAll(tempDir); err != nil {
					// We are deleting this on server so log on server end
					// client does not have to worry about server cleanup.
					logrus.Errorf("Cannot delete downloaded temp dir %q: %s", tempDir, err)
				}
			}()
			contextDirectory = filepath.Join(tempDir, subDir)
		} else {
			// Nope, it was local.  Use it as is.
			absDir, err := filepath.Abs(query.Remote)
			if err != nil {
				utils.BadRequest(w, "remote", query.Remote, err)
				return
			}
			contextDirectory = absDir
		}
	} else {
		if _, found := r.URL.Query()["dockerfile"]; found {
			var m = []string{}
			if err := json.Unmarshal([]byte(query.Dockerfile), &m); err != nil {
				// it's not json, assume just a string
				m = []string{query.Dockerfile}
			}

			for _, containerfile := range m {
				// Add path to containerfile iff it is not URL
				if !strings.HasPrefix(containerfile, "http://") && !strings.HasPrefix(containerfile, "https://") {
					containerfile = filepath.Join(contextDirectory,
						filepath.Clean(filepath.FromSlash(containerfile)))
				}
				containerFiles = append(containerFiles, containerfile)
			}
			dockerFileSet = true
		}
	}

	if !dockerFileSet {
		containerFiles = []string{filepath.Join(contextDirectory, "Dockerfile")}
		if utils.IsLibpodRequest(r) {
			containerFiles = []string{filepath.Join(contextDirectory, "Containerfile")}
			if err = fileutils.Exists(containerFiles[0]); err != nil {
				containerFiles = []string{filepath.Join(contextDirectory, "Dockerfile")}
				if err1 := fileutils.Exists(containerFiles[0]); err1 != nil {
					utils.BadRequest(w, "dockerfile", query.Dockerfile, err)
					return
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

	var compatVolumes types.OptionalBool
	if _, found := r.URL.Query()["compatvolumes"]; found {
		compatVolumes = types.NewOptionalBool(query.CompatVolumes)
	}

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
					key, val, hasVal := strings.Cut(token, "=")
					if hasVal {
						if key == "src" {
							/* move secret away from contextDir */
							/* to make sure we dont accidentally commit temporary secrets to image*/
							builderDirectory, _ := filepath.Split(contextDirectory)
							// following path is outside build context
							newSecretPath := filepath.Join(builderDirectory, val)
							oldSecretPath := filepath.Join(contextDirectory, val)
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
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
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
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse isolation: %w", err))
			return
		}

		// Make sure to force rootless as rootless otherwise buildah runs code which is intended to be run only as root.
		// Same the other way around: https://github.com/containers/podman/issues/22109
		switch isolation {
		case buildah.IsolationOCI:
			if rootless.IsRootless() {
				isolation = buildah.IsolationOCIRootless
			}
		case buildah.IsolationOCIRootless:
			if !rootless.IsRootless() {
				isolation = buildah.IsolationOCI
			}
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
	var additionalTags []string
	for i := 1; i < len(tags); i++ {
		possiblyNormalizedTag, err := utils.NormalizeToDockerHub(r, tags[i])
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
			return
		}
		additionalTags = append(additionalTags, possiblyNormalizedTag)
	}

	var idMappingOptions buildahDefine.IDMappingOptions
	if _, found := r.URL.Query()["idmappingoptions"]; found {
		if err := json.Unmarshal([]byte(query.IDMappingOptions), &idMappingOptions); err != nil {
			utils.BadRequest(w, "idmappingoptions", query.IDMappingOptions, err)
			return
		}
	}

	cacheFrom := []reference.Named{}
	if _, found := r.URL.Query()["cachefrom"]; found {
		var cacheFromSrcList []string
		if err := json.Unmarshal([]byte(query.CacheFrom), &cacheFromSrcList); err != nil {
			utils.BadRequest(w, "cacheFrom", query.CacheFrom, err)
			return
		}
		cacheFrom, err = parse.RepoNamesToNamedReferences(cacheFromSrcList)
		if err != nil {
			utils.BadRequest(w, "cacheFrom", query.CacheFrom, err)
			return
		}
	}
	cacheTo := []reference.Named{}
	if _, found := r.URL.Query()["cacheto"]; found {
		var cacheToDestList []string
		if err := json.Unmarshal([]byte(query.CacheTo), &cacheToDestList); err != nil {
			utils.BadRequest(w, "cacheTo", query.CacheTo, err)
			return
		}
		cacheTo, err = parse.RepoNamesToNamedReferences(cacheToDestList)
		if err != nil {
			utils.BadRequest(w, "cacheto", query.CacheTo, err)
			return
		}
	}
	var cacheTTL time.Duration
	if _, found := r.URL.Query()["cachettl"]; found {
		cacheTTL, err = time.ParseDuration(query.CacheTTL)
		if err != nil {
			utils.BadRequest(w, "cachettl", query.CacheTTL, err)
			return
		}
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

	// convert cppflags formats
	var cppflags = []string{}
	if _, found := r.URL.Query()["cppflags"]; found {
		if err := json.Unmarshal([]byte(query.CPPFlags), &cppflags); err != nil {
			utils.BadRequest(w, "cppflags", query.CPPFlags, err)
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
				name, value, hasValue := strings.Cut(opt, "=")
				if !hasValue {
					utils.BadRequest(w, "securityopt", query.SecurityOpt, fmt.Errorf("invalid --security-opt name=value pair: %q", opt))
					return
				}

				switch name {
				case "label":
					labelOpts = append(labelOpts, value)
				case "apparmor":
					apparmor = value
				case "seccomp":
					seccomp = value
				default:
					utils.BadRequest(w, "securityopt", query.SecurityOpt, fmt.Errorf("invalid --security-opt 2: %q", opt))
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
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
			return
		}
		fromImage = possiblyNormalizedName
	}

	systemContext := &types.SystemContext{
		AuthFilePath:     authfile,
		DockerAuthConfig: creds,
	}
	if err := utils.PossiblyEnforceDockerHub(r, systemContext); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("checking to enforce DockerHub: %w", err))
		return
	}

	var skipUnusedStages types.OptionalBool
	if _, found := r.URL.Query()["skipunusedstages"]; found {
		skipUnusedStages = types.NewOptionalBool(query.SkipUnusedStages)
	}

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

	_, ignoreFile, err := util.ParseDockerignore(containerFiles, contextDirectory)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("processing ignore file: %w", err))
		return
	}

	retryDelay := 2 * time.Second
	if query.RetryDelay != "" {
		retryDelay, err = time.ParseDuration(query.RetryDelay)
		if err != nil {
			utils.BadRequest(w, "retry-delay", query.RetryDelay, err)
			return
		}
	}

	// Note: avoid using types.NewOptionaBool() to initialize optional bool fields of this
	// struct without checking if the client supplied a value.  Skipping that step prevents
	// the builder from choosing/using its defaults.
	buildOptions := buildahDefine.BuildOptions{
		AddCapabilities:         addCaps,
		AdditionalBuildContexts: additionalBuildContexts,
		AdditionalTags:          additionalTags,
		Annotations:             annotations,
		CPPFlags:                cppflags,
		CacheFrom:               cacheFrom,
		CacheTo:                 cacheTo,
		CacheTTL:                cacheTTL,
		Args:                    buildArgs,
		AllPlatforms:            query.AllPlatforms,
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
			IdentityLabel:      identityLabel,
			LabelOpts:          labelOpts,
			Memory:             query.Memory,
			MemorySwap:         query.MemSwap,
			NoHosts:            query.NoHosts,
			OmitHistory:        query.OmitHistory,
			SeccompProfilePath: seccomp,
			ShmSize:            strconv.Itoa(query.ShmSize),
			Ulimit:             ulimits,
			Secrets:            secrets,
			Volumes:            query.Volumes,
		},
		CompatVolumes:                  compatVolumes,
		CreatedAnnotation:              query.CreatedAnnotation,
		Compression:                    compression,
		ConfigureNetwork:               parseNetworkConfigurationPolicy(query.ConfigureNetwork),
		ContextDirectory:               contextDirectory,
		Devices:                        devices,
		DropCapabilities:               dropCaps,
		Envs:                           query.Envs,
		Err:                            auxout,
		Excludes:                       excludes,
		ForceRmIntermediateCtrs:        query.ForceRm,
		GroupAdd:                       query.GroupAdd,
		From:                           fromImage,
		IDMappingOptions:               &idMappingOptions,
		IgnoreUnrecognizedInstructions: query.Ignore,
		IgnoreFile:                     ignoreFile,
		InheritLabels:                  query.InheritLabels,
		InheritAnnotations:             query.InheritAnnotations,
		Isolation:                      isolation,
		Jobs:                           &jobs,
		Labels:                         labels,
		LayerLabels:                    query.LayerLabels,
		Layers:                         query.Layers,
		LogRusage:                      query.LogRusage,
		Manifest:                       query.Manifest,
		MaxPullPushRetries:             query.Retry,
		NamespaceOptions:               nsoptions,
		NoCache:                        query.NoCache,
		OSFeatures:                     query.OSFeatures,
		OSVersion:                      query.OSVersion,
		Out:                            stdout,
		Output:                         output,
		OutputFormat:                   format,
		PullPolicy:                     pullPolicy,
		PullPushRetryDelay:             retryDelay,
		Quiet:                          query.Quiet,
		Registry:                       registry,
		RemoveIntermediateCtrs:         query.Rm,
		ReportWriter:                   reporter,
		RewriteTimestamp:               query.RewriteTimestamp,
		RusageLogFile:                  query.RusageLogFile,
		SkipUnusedStages:               skipUnusedStages,
		Squash:                         query.Squash,
		SystemContext:                  systemContext,
		Target:                         query.Target,
		UnsetEnvs:                      query.UnsetEnvs,
		UnsetLabels:                    query.UnsetLabels,
		UnsetAnnotations:               query.UnsetAnnotations,
	}

	platforms := query.Platform
	if len(platforms) == 1 {
		// Docker API uses comma separated platform arg so match this here
		platforms = strings.Split(query.Platform[0], ",")
	}
	for _, platformSpec := range platforms {
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
	if _, found := r.URL.Query()["sourcedateepoch"]; found {
		ts := time.Unix(query.SourceDateEpoch, 0)
		buildOptions.SourceDateEpoch = &ts
	}
	if _, found := r.URL.Query()["timestamp"]; found {
		ts := time.Unix(query.Timestamp, 0)
		buildOptions.Timestamp = &ts
	}

	var (
		imageID string
		success bool
	)

	runCtx, cancel := context.WithCancel(r.Context())
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
				t, _ := os.CreateTemp("", "build_*_server")
				defer t.Close()
				body = io.MultiWriter(t, w)
			}
		}
	}

	enc := json.NewEncoder(body)
	enc.SetEscapeHTML(true)
	var stepErrors []string

	for {
		m := images.BuildResponse{}

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
				t := images.BuildResponse{}
				t.Stream = stepError
				if err := enc.Encode(t); err != nil {
					stderr.Write([]byte(err.Error()))
				}
				flush()
			}
			m.ErrorMessage = string(e)
			m.Error = &jsonmessage.JSONError{
				Message: string(e),
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
					flush()
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

func handleBuildContexts(anchorDir string, r *http.Request, multipart bool) (contextDir string, additionalContexts map[string]*buildahDefine.AdditionalBuildContext, err error) {
	additionalContexts = make(map[string]*buildahDefine.AdditionalBuildContext)
	query := r.URL.Query()

	for _, url := range query["additionalbuildcontexts"] {
		name, value, found := strings.Cut(url, "=")
		if !found {
			return "", nil, fmt.Errorf("invalid additional build context format: %q", url)
		}

		logrus.Debugf("name: %q, context: %q", name, value)

		switch {
		case strings.HasPrefix(value, "url:"):
			value = strings.TrimPrefix(value, "url:")
			tempDir, subdir, err := buildahDefine.TempDirForURL(anchorDir, "buildah", value)
			if err != nil {
				return "", nil, fmt.Errorf("downloading URL %q: %w", name, err)
			}

			contextPath := filepath.Join(tempDir, subdir)
			additionalContexts[name] = &buildahDefine.AdditionalBuildContext{
				IsURL:           true,
				IsImage:         false,
				Value:           contextPath,
				DownloadedCache: contextPath,
			}

			logrus.Debugf("Downloaded URL context %q to %q", name, contextPath)
		case strings.HasPrefix(value, "image:"):
			value = strings.TrimPrefix(value, "image:")
			additionalContexts[name] = &buildahDefine.AdditionalBuildContext{
				IsURL:   false,
				IsImage: true,
				Value:   value,
			}

			logrus.Debugf("Using image context %q: %q", name, value)
		}
	}

	// If we have a multipart we use the operations, if not default extraction for main context
	if multipart {
		logrus.Debug("Multipart is needed")
		reader, err := r.MultipartReader()
		if err != nil {
			return "", nil, fmt.Errorf("failed to create multipart reader: %w", err)
		}

		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", nil, fmt.Errorf("failed to read multipart: %w", err)
			}

			fieldName := part.FormName()

			switch {
			case fieldName == "MainContext":
				mainDir, err := extractTarFile(anchorDir, part)
				if err != nil {
					part.Close()
					return "", nil, fmt.Errorf("extracting main context in multipart: %w", err)
				}
				if mainDir == "" {
					part.Close()
					return "", nil, fmt.Errorf("main context directory is empty")
				}
				contextDir = mainDir
				part.Close()

			case strings.HasPrefix(fieldName, "build-context-"):
				contextName := strings.TrimPrefix(fieldName, "build-context-")

				// Create temp directory directly under anchorDir
				additionalAnchor, err := os.MkdirTemp(anchorDir, contextName+"-*")
				if err != nil {
					part.Close()
					return "", nil, fmt.Errorf("creating temp directory for additional context %q: %w", contextName, err)
				}

				if err := chrootarchive.Untar(part, additionalAnchor, nil); err != nil {
					part.Close()
					return "", nil, fmt.Errorf("extracting additional context %q: %w", contextName, err)
				}

				var latestModTime time.Time
				fileCount := 0
				walkErr := filepath.Walk(additionalAnchor, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					// Skip the root directory itself since it's always going to have the latest timestamp
					if path == additionalAnchor {
						return nil
					}
					if !info.IsDir() {
						fileCount++
					}
					// Use any extracted content timestamp (files or subdirectories)
					if info.ModTime().After(latestModTime) {
						latestModTime = info.ModTime()
					}
					return nil
				})
				if walkErr != nil {
					part.Close()
					return "", nil, fmt.Errorf("error walking additional context: %w", walkErr)
				}

				// If we found any files, set the timestamp on the additional context directory
				// to the latest modified time found in the files.
				if !latestModTime.IsZero() {
					if err := os.Chtimes(additionalAnchor, latestModTime, latestModTime); err != nil {
						logrus.Warnf("Failed to set timestamp on additional context directory: %v", err)
					}
				}

				additionalContexts[contextName] = &buildahDefine.AdditionalBuildContext{
					IsURL:   false,
					IsImage: false,
					Value:   additionalAnchor,
				}
				part.Close()
			default:
				logrus.Debugf("Ignoring unknown multipart field: %s", fieldName)
				part.Close()
			}
		}
	} else {
		logrus.Debug("No multipart needed")
		contextDir, err = extractTarFile(anchorDir, r.Body)
		if err != nil {
			return "", nil, err
		}
	}

	return contextDir, additionalContexts, nil
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

func parseLibPodIsolation(isolation string) (buildah.Isolation, error) {
	if val, err := strconv.Atoi(isolation); err == nil {
		return buildah.Isolation(val), nil
	}
	return parse.IsolationOption(isolation)
}

func extractTarFile(anchorDir string, r io.ReadCloser) (string, error) {
	buildDir := filepath.Join(anchorDir, "build")
	err := os.Mkdir(buildDir, 0o700)
	if err != nil {
		return "", err
	}

	err = archive.Untar(r, buildDir, nil)
	return buildDir, err
}
