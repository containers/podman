/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package release

import (
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/promo-tools/v3/image"
	"sigs.k8s.io/release-sdk/git"
	"sigs.k8s.io/release-sdk/github"
	"sigs.k8s.io/release-sdk/object"
	"sigs.k8s.io/release-utils/command"
	"sigs.k8s.io/release-utils/env"
	rhash "sigs.k8s.io/release-utils/hash"
	"sigs.k8s.io/release-utils/tar"
	"sigs.k8s.io/release-utils/util"
)

const (
	DefaultToolRepo = "release"
	DefaultToolRef  = git.DefaultBranch
	DefaultToolOrg  = git.DefaultGithubOrg

	DefaultK8sOrg  = git.DefaultGithubOrg
	DefaultK8sRepo = git.DefaultGithubRepo
	DefaultK8sRef  = git.DefaultRef

	// TODO(vdf): Need to reference K8s Infra project here
	DefaultKubernetesStagingProject = "kubernetes-release-test"
	DefaultRelengStagingTestProject = "k8s-staging-releng-test"
	DefaultRelengStagingProject     = "k8s-staging-releng"
	DefaultDiskSize                 = "500"
	BucketPrefix                    = "kubernetes-release-"
	BucketPrefixK8sInfra            = "k8s-release-"

	versionReleaseRE   = `v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[a-zA-Z0-9]+)*\.*(0|[1-9][0-9]*)?`
	versionBuildRE     = `([0-9]{1,})\+([0-9a-f]{5,40})`
	versionWorkspaceRE = `gitVersion ([^\n]+)`
	versionDirtyRE     = `(-dirty)`

	KubernetesTar = "kubernetes.tar.gz"

	// Staged source code tarball of Kubernetes
	SourcesTar = "src.tar.gz"

	// Root path on the bucket for staged artifacts
	StagePath = "stage"

	// Path where the release container images are stored
	ImagesPath = "release-images"

	// GCSStagePath is the directory where release artifacts are staged before
	// push to GCS.
	GCSStagePath = "gcs-stage"

	// ReleaseStagePath is the directory where releases are staged.
	ReleaseStagePath = "release-stage"

	// GCEPath is the directory where GCE scripts are created.
	GCEPath = ReleaseStagePath + "/full/kubernetes/cluster/gce"

	// GCIPath is the path for the container optimized OS for gcli.
	GCIPath = ReleaseStagePath + "/full/kubernetes/cluster/gce/gci"

	// ReleaseTarsPath is the directory where release artifacts are created.
	ReleaseTarsPath = "release-tars"

	// WindowsLocalPath is the directory where Windows GCE scripts are created.
	WindowsLocalPath = ReleaseStagePath + "/full/kubernetes/cluster/gce/windows"

	// CIBucketLegacy is the default bucket for Kubernetes CI releases
	CIBucketLegacy = "kubernetes-release-dev"

	// CIBucketK8sInfra is the community infra bucket for Kubernetes CI releases
	CIBucketK8sInfra = "k8s-release-dev"

	// TestBucket is the default bucket for mocked Kubernetes releases
	TestBucket = "kubernetes-release-gcb"

	// ProductionBucket is the default bucket for Kubernetes releases
	ProductionBucket = "kubernetes-release"

	// ProductionBucketURL is the url for the ProductionBucket
	ProductionBucketURL = "https://dl.k8s.io"

	// Production registry root URL
	GCRIOPathProd = image.ProdRegistry

	// Staging registry root URL prefix
	GCRIOPathStagingPrefix = image.StagingRepoPrefix

	// Staging registry root URL
	GCRIOPathStaging = GCRIOPathStagingPrefix + image.StagingRepoSuffix

	// Mock staging registry root URL
	GCRIOPathMock = GCRIOPathStaging + "/mock"

	// BuildDir is the default build output directory.
	BuildDir = "_output"

	// The default bazel build directory.
	BazelBuildDir = "bazel-bin/build"

	// Archive path is the root path in the bucket where releases are archived
	ArchivePath = "archive"

	// Publishing bot issue repository
	PubBotRepoOrg  = "k8s-release-robot"
	PubBotRepoName = "sig-release"

	DockerHubEnvKey   = "DOCKERHUB_TOKEN" // Env var containing the docker key
	DockerHubUserName = "k8sreleng"       // Docker Hub username

	ProvenanceFilename = "provenance.json" // Name of the SLSA provenance file (used in stage and release)
)

var (
	ManifestImages = []string{
		"conformance",
		"kube-apiserver",
		"kube-controller-manager",
		"kube-proxy",
		"kube-scheduler",
	}

	SupportedArchitectures = []string{
		"amd64",
		"arm",
		"arm64",
		"ppc64le",
		"s390x",
	}

	FastArchitectures = []string{
		"amd64",
	}
)

// GetToolOrg checks if the 'TOOL_ORG' environment variable is set.
// If 'TOOL_ORG' is non-empty, it returns the value. Otherwise, it returns DefaultToolOrg.
func GetToolOrg() string {
	return env.Default("TOOL_ORG", DefaultToolOrg)
}

// GetToolRepo checks if the 'TOOL_REPO' environment variable is set.
// If 'TOOL_REPO' is non-empty, it returns the value. Otherwise, it returns DefaultToolRepo.
func GetToolRepo() string {
	return env.Default("TOOL_REPO", DefaultToolRepo)
}

// GetToolRef checks if the 'TOOL_REF' environment variable is set.
// If 'TOOL_REF' is non-empty, it returns the value. Otherwise, it returns DefaultToolRef.
func GetToolRef() string {
	return env.Default("TOOL_REF", DefaultToolRef)
}

// GetK8sOrg checks if the 'K8S_ORG' environment variable is set.
// If 'K8S_ORG' is non-empty, it returns the value. Otherwise, it returns DefaultK8sOrg.
func GetK8sOrg() string {
	return env.Default("K8S_ORG", DefaultK8sOrg)
}

// GetK8sRepo checks if the 'K8S_REPO' environment variable is set.
// If 'K8S_REPO' is non-empty, it returns the value. Otherwise, it returns DefaultK8sRepo.
func GetK8sRepo() string {
	return env.Default("K8S_REPO", DefaultK8sRepo)
}

// GetK8sRef checks if the 'K8S_REF' environment variable is set.
// If 'K8S_REF' is non-empty, it returns the value. Otherwise, it returns DefaultK8sRef.
func GetK8sRef() string {
	return env.Default("K8S_REF", DefaultK8sRef)
}

// IsDefaultK8sUpstream returns true if GetK8sOrg(), GetK8sRepo() and
// GetK8sRef() point to their default values.
func IsDefaultK8sUpstream() bool {
	return GetK8sOrg() == DefaultK8sOrg &&
		GetK8sRepo() == DefaultK8sRepo &&
		GetK8sRef() == DefaultK8sRef
}

// BuiltWithBazel determines whether the most recent Kubernetes release was built with Bazel.
func BuiltWithBazel(workDir string) (bool, error) {
	bazelBuild := filepath.Join(workDir, BazelBuildDir, ReleaseTarsPath, KubernetesTar)
	dockerBuild := filepath.Join(workDir, BuildDir, ReleaseTarsPath, KubernetesTar)
	return util.MoreRecent(bazelBuild, dockerBuild)
}

// ReadBazelVersion reads the version from a Bazel build.
func ReadBazelVersion(workDir string) (string, error) {
	version, err := os.ReadFile(filepath.Join(workDir, "bazel-bin", "version"))
	if os.IsNotExist(err) {
		// The check for version in bazel-genfiles can be removed once everyone is
		// off of versions before 0.25.0.
		// https://github.com/bazelbuild/bazel/issues/8651
		version, err = os.ReadFile(filepath.Join(workDir, "bazel-genfiles/version"))
	}
	return string(version), err
}

// ReadDockerizedVersion reads the version from a Dockerized Kubernetes build.
func ReadDockerizedVersion(workDir string) (string, error) {
	dockerTarball := filepath.Join(workDir, BuildDir, ReleaseTarsPath, KubernetesTar)
	reader, err := tar.ReadFileFromGzippedTar(
		dockerTarball, filepath.Join("kubernetes", "version"),
	)
	if err != nil {
		return "", err
	}
	file, err := io.ReadAll(reader)
	return strings.TrimSpace(string(file)), err
}

// IsValidReleaseBuild checks if build version is valid for release.
func IsValidReleaseBuild(build string) (bool, error) {
	// If the tag has a plus sign, then we force the versionBuildRe to match
	if strings.Contains(build, "+") {
		return regexp.MatchString("("+versionReleaseRE+`(\.`+versionBuildRE+")"+versionDirtyRE+"?)", build)
	}
	return regexp.MatchString("("+versionReleaseRE+`(\.`+versionBuildRE+")?"+versionDirtyRE+"?)", build)
}

// IsDirtyBuild checks if build version is dirty.
func IsDirtyBuild(build string) bool {
	return strings.Contains(build, "dirty")
}

func GetWorkspaceVersion() (string, error) {
	workspaceStatusScript := "hack/print-workspace-status.sh"
	_, workspaceStatusScriptStatErr := os.Stat(workspaceStatusScript)
	if os.IsNotExist(workspaceStatusScriptStatErr) {
		return "", errors.Wrapf(workspaceStatusScriptStatErr,
			"checking for workspace status script",
		)
	}

	logrus.Info("Getting workspace status")
	workspaceStatusStream, getWorkspaceStatusErr := command.New(workspaceStatusScript).RunSuccessOutput()
	if getWorkspaceStatusErr != nil {
		return "", errors.Wrapf(getWorkspaceStatusErr, "getting workspace status")
	}

	workspaceStatus := workspaceStatusStream.Output()

	re := regexp.MustCompile(versionWorkspaceRE)
	submatch := re.FindStringSubmatch(workspaceStatus)

	version := submatch[1]

	logrus.Infof("Found workspace version: %s", version)
	return version, nil
}

// URLPrefixForBucket returns the URL prefix for the provided bucket string
func URLPrefixForBucket(bucket string) string {
	bucket = strings.TrimPrefix(bucket, object.GcsPrefix)
	urlPrefix := fmt.Sprintf("https://storage.googleapis.com/%s", bucket)
	if bucket == ProductionBucket {
		urlPrefix = ProductionBucketURL
	}
	return urlPrefix
}

// CopyBinaries takes the provided `rootPath` and copies the binaries sorted by
// their platform into the `targetPath`.
func CopyBinaries(rootPath, targetPath string) error {
	platformsPath := filepath.Join(rootPath, "client")
	platformsAndArches, err := os.ReadDir(platformsPath)
	if err != nil {
		return errors.Wrapf(err, "retrieve platforms from %s", platformsPath)
	}

	for _, platformArch := range platformsAndArches {
		if !platformArch.IsDir() {
			logrus.Warnf(
				"Skipping platform and arch %q because it's not a directory",
				platformArch.Name(),
			)
			continue
		}

		split := strings.Split(platformArch.Name(), "-")
		if len(split) != 2 {
			return errors.Errorf(
				"expected `platform-arch` format for %s", platformArch.Name(),
			)
		}

		platform := split[0]
		arch := split[1]
		logrus.Infof(
			"Copying binaries for %s platform on %s arch", platform, arch,
		)

		src := filepath.Join(
			rootPath, "client", platformArch.Name(), "kubernetes", "client", "bin",
		)

		// We assume here the "server package" is a superset of the "client
		// package"
		serverSrc := filepath.Join(rootPath, "server", platformArch.Name())
		if util.Exists(serverSrc) {
			logrus.Infof("Server source found in %s, copying them", serverSrc)
			src = filepath.Join(serverSrc, "kubernetes", "server", "bin")
		}

		dst := filepath.Join(targetPath, "bin", platform, arch)
		logrus.Infof("Copying server binaries from %s to %s", src, dst)
		if err := util.CopyDirContentsLocal(src, dst); err != nil {
			return errors.Wrapf(err,
				"copy server binaries from %s to %s", src, dst,
			)
		}

		// Copy node binaries if they exist and this isn't a 'server' platform
		nodeSrc := filepath.Join(rootPath, "node", platformArch.Name())
		if !util.Exists(serverSrc) && util.Exists(nodeSrc) {
			src = filepath.Join(nodeSrc, "kubernetes", "node", "bin")

			logrus.Infof("Copying node binaries from %s to %s", src, dst)
			if err := util.CopyDirContentsLocal(src, dst); err != nil {
				return errors.Wrapf(err,
					"copy node binaries from %s to %s", src, dst,
				)
			}
		}
	}
	return nil
}

// WriteChecksums writes the SHA256SUMS/SHA512SUMS files (contains all
// checksums) as well as a sepearete *.sha[256|512] file containing only the
// SHA for the corresponding file name.
func WriteChecksums(rootPath string) error {
	logrus.Info("Writing artifact hashes to SHA256SUMS/SHA512SUMS files")

	createSHASums := func(hasher hash.Hash) (string, error) {
		fileName := fmt.Sprintf("SHA%dSUMS", hasher.Size()*8)
		files := []string{}

		if err := filepath.Walk(rootPath,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}

				sha, err := rhash.ForFile(path, hasher)
				if err != nil {
					return errors.Wrap(err, "get hash from file")
				}

				files = append(files, fmt.Sprintf("%s  %s", sha, path))
				return nil
			},
		); err != nil {
			return "", errors.Wrapf(err, "traversing root path %s", rootPath)
		}

		file, err := os.Create(fileName)
		if err != nil {
			return "", errors.Wrapf(err, "create file %s", fileName)
		}
		if _, err := file.WriteString(strings.Join(files, "\n")); err != nil {
			return "", errors.Wrapf(err, "write to file %s", fileName)
		}

		return file.Name(), nil
	}

	// Write the release checksum files.
	// We checksum everything except our checksum files, which we do next.
	sha256SumsFile, err := createSHASums(sha256.New())
	if err != nil {
		return errors.Wrap(err, "create SHA256 sums")
	}
	sha512SumsFile, err := createSHASums(sha512.New())
	if err != nil {
		return errors.Wrap(err, "create SHA512 sums")
	}

	// After all the checksum files are generated, move them into the bucket
	// staging area
	moveFile := func(file string) error {
		if err := util.CopyFileLocal(
			file, filepath.Join(rootPath, file), true,
		); err != nil {
			return errors.Wrapf(err, "move %s sums file to %s", file, rootPath)
		}
		if err := os.RemoveAll(file); err != nil {
			return errors.Wrapf(err, "remove file %s", file)
		}
		return nil
	}
	if err := moveFile(sha256SumsFile); err != nil {
		return errors.Wrap(err, "move SHA256 sums")
	}
	if err := moveFile(sha512SumsFile); err != nil {
		return errors.Wrap(err, "move SHA512 sums")
	}

	logrus.Infof("Hashing files in %s", rootPath)

	writeSHAFile := func(fileName string, hasher hash.Hash) error {
		sha, err := rhash.ForFile(fileName, hasher)
		if err != nil {
			return errors.Wrap(err, "get hash from file")
		}
		shaFileName := fmt.Sprintf("%s.sha%d", fileName, hasher.Size()*8)

		return errors.Wrapf(
			os.WriteFile(shaFileName, []byte(sha), os.FileMode(0o644)),
			"write SHA to file %s", shaFileName,
		)
	}

	if err := filepath.Walk(rootPath,
		func(path string, file os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if file.IsDir() {
				return nil
			}

			if err := writeSHAFile(path, sha256.New()); err != nil {
				return errors.Wrapf(err, "write %s.sha256", file.Name())
			}

			if err := writeSHAFile(path, sha512.New()); err != nil {
				return errors.Wrapf(err, "write %s.sha512", file.Name())
			}
			return nil
		},
	); err != nil {
		return errors.Wrapf(err, "traversing root path %s", rootPath)
	}

	return nil
}

// CreatePubBotBranchIssue creates an issue on GitHub to notify
func CreatePubBotBranchIssue(branchName string) error {
	// Check the GH token is set
	if os.Getenv(github.TokenEnvKey) == "" {
		return errors.New("cannot file publishing bot issue as GitHub token is not set")
	}

	gh := github.New()

	// Create the body for the issue
	issueBody := fmt.Sprintf("The branch `%s` was just created.\n\n", branchName)
	issueBody += "Please update the publishing-bot's configuration to also publish this new branch.\n\n"
	issueBody += "/sig release\n"
	issueBody += "/area release-eng\n"
	issueBody += "/milestone v" + strings.TrimPrefix(branchName, "release-") + "\n"

	// Create the issue on GitHub
	issue, err := gh.CreateIssue(
		PubBotRepoOrg, PubBotRepoName,
		"Update publishing-bot for "+branchName,
		issueBody,
		&github.NewIssueOptions{},
	)
	if err != nil {
		return errors.Wrap(err, "creating publishing bot issue")
	}
	logrus.Infof("Publishing bot issue created #%d!", issue.GetNumber())
	return nil
}

// Calls docker login to log into docker hub using a token from the environment
func DockerHubLogin() error {
	// Check the environment  variable is set
	if os.Getenv(DockerHubEnvKey) == "" {
		return errors.New("Unable to find docker token in the environment")
	}
	// Pipe the token into docker login
	cmd := command.New(
		"docker", "login", fmt.Sprintf("--username=%s", DockerHubUserName),
		"--password", os.Getenv(DockerHubEnvKey),
	)
	// Run docker login:
	if err := cmd.RunSuccess(); err != nil {
		errStr := strings.ReplaceAll(err.Error(), os.Getenv(DockerHubEnvKey), "**********")
		return errors.Wrap(errors.New(errStr), "logging into Docker Hub")
	}
	logrus.Infof("User %s successfully logged into Docker Hub", DockerHubUserName)
	return nil
}
