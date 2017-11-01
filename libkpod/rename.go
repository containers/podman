package libkpod

import (
	"encoding/json"
	"path/filepath"

	"k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/kubernetes-incubator/cri-o/pkg/annotations"
	"github.com/opencontainers/runtime-tools/generate"
)

const configFile = "config.json"

// ContainerRename renames the given container
func (c *ContainerServer) ContainerRename(container, name string) error {
	ctr, err := c.LookupContainer(container)
	if err != nil {
		return err
	}

	oldName := ctr.Name()
	_, err = c.ReserveContainerName(ctr.ID(), name)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			c.ReleaseContainerName(name)
		} else {
			c.ReleaseContainerName(oldName)
		}
	}()

	// Update state.json
	if err = c.updateStateName(ctr, name); err != nil {
		return err
	}

	// Update config.json
	configRuntimePath := filepath.Join(ctr.BundlePath(), configFile)
	if err = updateConfigName(configRuntimePath, name); err != nil {
		return err
	}
	configStoragePath := filepath.Join(ctr.Dir(), configFile)
	if err = updateConfigName(configStoragePath, name); err != nil {
		return err
	}

	// Update containers.json
	if err = c.store.SetNames(ctr.ID(), []string{name}); err != nil {
		return err
	}
	return nil
}

func updateConfigName(configPath, name string) error {
	specgen, err := generate.NewFromFile(configPath)
	if err != nil {
		return err
	}
	specgen.AddAnnotation(annotations.Name, name)
	specgen.AddAnnotation(annotations.Metadata, updateMetadata(specgen.Spec().Annotations, name))

	return specgen.SaveToFile(configPath, generate.ExportOptions{})
}

func (c *ContainerServer) updateStateName(ctr *oci.Container, name string) error {
	ctr.State().Annotations[annotations.Name] = name
	ctr.State().Annotations[annotations.Metadata] = updateMetadata(ctr.State().Annotations, name)
	// This is taken directly from c.ContainerStateToDisk(), which can't be used because of the call to UpdateStatus() in the first line
	jsonSource, err := ioutils.NewAtomicFileWriter(ctr.StatePath(), 0644)
	if err != nil {
		return err
	}
	defer jsonSource.Close()
	enc := json.NewEncoder(jsonSource)
	return enc.Encode(c.runtime.ContainerStatus(ctr))
}

// Attempts to update a metadata annotation
func updateMetadata(specAnnotations map[string]string, name string) string {
	oldMetadata := specAnnotations[annotations.Metadata]
	containerType := specAnnotations[annotations.ContainerType]
	if containerType == "container" {
		metadata := runtime.ContainerMetadata{}
		err := json.Unmarshal([]byte(oldMetadata), metadata)
		if err != nil {
			return oldMetadata
		}
		metadata.Name = name
		m, err := json.Marshal(metadata)
		if err != nil {
			return oldMetadata
		}
		return string(m)
	} else if containerType == "sandbox" {
		metadata := runtime.PodSandboxMetadata{}
		err := json.Unmarshal([]byte(oldMetadata), metadata)
		if err != nil {
			return oldMetadata
		}
		metadata.Name = name
		m, err := json.Marshal(metadata)
		if err != nil {
			return oldMetadata
		}
		return string(m)
	} else {
		return specAnnotations[annotations.Metadata]
	}
}
