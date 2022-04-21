// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type CheckpointedPod struct {
	PodUID                 string                  `json:"io.kubernetes.pod.uid,omitempty"`
	ID                     string                  `json:"SandboxID,omitempty"`
	Name                   string                  `json:"io.kubernetes.pod.name,omitempty"`
	TerminationGracePeriod int64                   `json:"io.kubernetes.pod.terminationGracePeriod,omitempty"`
	Namespace              string                  `json:"io.kubernetes.pod.namespace,omitempty"`
	ConfigSource           string                  `json:"kubernetes.io/config.source,omitempty"`
	ConfigSeen             string                  `json:"kubernetes.io/config.seen,omitempty"`
	Manager                string                  `json:"io.container.manager,omitempty"`
	Containers             []CheckpointedContainer `json:"Containers"`
	HostIP                 string                  `json:"hostIP,omitempty"`
	PodIP                  string                  `json:"podIP,omitempty"`
	PodIPs                 []string                `json:"podIPs,omitempty"`
}

type CheckpointedContainer struct {
	Name                      string `json:"io.kubernetes.container.name,omitempty"`
	ID                        string `json:"id,omitempty"`
	TerminationMessagePath    string `json:"io.kubernetes.container.terminationMessagePath,omitempty"`
	TerminationMessagePolicy  string `json:"io.kubernetes.container.terminationMessagePolicy,omitempty"`
	RestartCounter            int32  `json:"io.kubernetes.container.restartCount,omitempty"`
	TerminationMessagePathUID string `json:"terminationMessagePathUID,omitempty"`
	Image                     string `json:"Image"`
}

type CheckpointMetadata struct {
	Version          int `json:"version"`
	CheckpointedPods []CheckpointedPod
}

const (
	// kubelet archive
	CheckpointedPodsFile = "checkpointed.pods"
	// container archive
	ConfigDumpFile             = "config.dump"
	SpecDumpFile               = "spec.dump"
	NetworkStatusFile          = "network.status"
	CheckpointDirectory        = "checkpoint"
	CheckpointVolumesDirectory = "volumes"
	DevShmCheckpointTar        = "devshm-checkpoint.tar"
	RootFsDiffTar              = "rootfs-diff.tar"
	DeletedFilesFile           = "deleted.files"
	DumpLogFile                = "dump.log"
	RestoreLogFile             = "restore.log"
	// pod archive
	PodOptionsFile = "pod.options"
	PodDumpFile    = "pod.dump"
)

type CheckpointType int

const (
	// The checkpoint archive contains a kubelet checkpoint
	// One or multiple pods and kubelet metadata (checkpointed.pods)
	Kubelet CheckpointType = iota
	// The checkpoint archive contains one pod including one or multiple containers
	Pod
	// The checkpoint archive contains a single container
	Container
	Unknown
)

// This is a reduced copy of what Podman uses to store checkpoint metadata
type ContainerConfig struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	RootfsImageName string    `json:"rootfsImageName,omitempty"`
	OCIRuntime      string    `json:"runtime,omitempty"`
	CreatedTime     time.Time `json:"createdTime"`
}

// This is metadata stored inside of a Pod checkpoint archive
type CheckpointedPodOptions struct {
	Version      int      `json:"version"`
	Containers   []string `json:"containers,omitempty"`
	MountLabel   string   `json:"mountLabel"`
	ProcessLabel string   `json:"processLabel"`
}

// This is metadata stored inside of Pod checkpoint archive
type PodSandboxConfig struct {
	Metadata SandboxMetadta `json:"metadata"`
	Hostname string         `json:"hostname"`
}

type SandboxMetadta struct {
	Name      string `json:"name"`
	UID       string `json:"uid"`
	Namespace string `json:"namespace"`
}

func checkForFile(checkpointDirectory, file string) (bool, error) {
	_, err := os.Stat(filepath.Join(checkpointDirectory, file))
	if err != nil && !os.IsNotExist(err) {
		return false, errors.Wrapf(err, "Failed to access %q\n", file)
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

func DetectCheckpointArchiveType(checkpointDirectory string) (CheckpointType, error) {
	kubelet, err := checkForFile(checkpointDirectory, CheckpointedPodsFile)
	if os.IsNotExist(err) {
		return Unknown, err
	}

	container, err := checkForFile(checkpointDirectory, ConfigDumpFile)
	if os.IsNotExist(err) {
		return Unknown, err
	}

	pod, err := checkForFile(checkpointDirectory, PodDumpFile)
	if os.IsNotExist(err) {
		return Unknown, err
	}

	if pod && !container && !kubelet {
		return Pod, nil
	}

	if !pod && container && !kubelet {
		return Container, nil
	}

	if !pod && !container && kubelet {
		return Kubelet, nil
	}

	return Unknown, nil
}

func ReadPodCheckpointDumpFile(checkpointDirectory string) (*PodSandboxConfig, string, error) {
	var podSandboxConfig PodSandboxConfig
	podDumpFile, err := ReadJSONFile(&podSandboxConfig, checkpointDirectory, PodDumpFile)

	return &podSandboxConfig, podDumpFile, err
}

func ReadPodCheckpointOptionsFile(checkpointDirectory string) (*CheckpointedPodOptions, string, error) {
	var checkpointedPodOptions CheckpointedPodOptions
	podOptionsFile, err := ReadJSONFile(&checkpointedPodOptions, checkpointDirectory, PodOptionsFile)

	return &checkpointedPodOptions, podOptionsFile, err
}

func ReadContainerCheckpointSpecDump(checkpointDirectory string) (*spec.Spec, string, error) {
	var specDump spec.Spec
	specDumpFile, err := ReadJSONFile(&specDump, checkpointDirectory, SpecDumpFile)

	return &specDump, specDumpFile, err
}

func ReadContainerCheckpointConfigDump(checkpointDirectory string) (*ContainerConfig, string, error) {
	var containerConfig ContainerConfig
	configDumpFile, err := ReadJSONFile(&containerConfig, checkpointDirectory, ConfigDumpFile)

	return &containerConfig, configDumpFile, err
}

func ReadContainerCheckpointDeletedFiles(checkpointDirectory string) ([]string, string, error) {
	var deletedFiles []string
	deletedFilesFile, err := ReadJSONFile(&deletedFiles, checkpointDirectory, DeletedFilesFile)

	return deletedFiles, deletedFilesFile, err
}

func ReadKubeletCheckpoints(checkpointsDirectory string) (*CheckpointMetadata, string, error) {
	var checkpointMetadata CheckpointMetadata
	checkpointMetadataPath, err := ReadJSONFile(&checkpointMetadata, checkpointsDirectory, CheckpointedPodsFile)

	return &checkpointMetadata, checkpointMetadataPath, err
}

// WriteJSONFile marshalls and writes the given data to a JSON file
func WriteJSONFile(v interface{}, dir, file string) (string, error) {
	fileJSON, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", errors.Wrapf(err, "Error marshalling JSON")
	}
	file = filepath.Join(dir, file)
	if err := ioutil.WriteFile(file, fileJSON, 0o600); err != nil {
		return "", errors.Wrapf(err, "Error writing to %q", file)
	}

	return file, nil
}

func ReadJSONFile(v interface{}, dir, file string) (string, error) {
	file = filepath.Join(dir, file)
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read %s", file)
	}
	if err = json.Unmarshal(content, v); err != nil {
		return "", errors.Wrapf(err, "failed to unmarshal %s", file)
	}

	return file, nil
}

func WriteKubeletCheckpointsMetadata(checkpointMetadata *CheckpointMetadata, dir string) error {
	_, err := WriteJSONFile(checkpointMetadata, dir, CheckpointedPodsFile)

	return err
}

func ByteToString(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
