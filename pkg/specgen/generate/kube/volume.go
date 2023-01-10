package kube

import (
	"errors"
	"fmt"
	"os"

	"github.com/containers/common/pkg/parse"
	"github.com/containers/common/pkg/secrets"
	"github.com/containers/podman/v4/libpod"
	v1 "github.com/containers/podman/v4/pkg/k8s.io/api/core/v1"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
)

const (
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	kubeDirectoryPermission = 0755
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	kubeFilePermission = 0644
)

//nolint:revive
type KubeVolumeType int

const (
	KubeVolumeTypeBindMount KubeVolumeType = iota
	KubeVolumeTypeNamed
	KubeVolumeTypeConfigMap
	KubeVolumeTypeBlockDevice
	KubeVolumeTypeCharDevice
	KubeVolumeTypeSecret
	KubeVolumeTypeEmptyDir
)

//nolint:revive
type KubeVolume struct {
	// Type of volume to create
	Type KubeVolumeType
	// Path for bind mount or volume name for named volume
	Source string
	// Items to add to a named volume created where the key is the file name and the value is the data
	// This is only used when there are volumes in the yaml that refer to a configmap
	// Example: if configmap has data "SPECIAL_LEVEL: very" then the file name is "SPECIAL_LEVEL" and the
	// data in that file is "very".
	Items map[string][]byte
	// If the volume is optional, we can move on if it is not found
	// Only used when there are volumes in a yaml that refer to a configmap
	Optional bool
}

// Create a KubeVolume from an HostPathVolumeSource
func VolumeFromHostPath(hostPath *v1.HostPathVolumeSource) (*KubeVolume, error) {
	if hostPath.Type != nil {
		switch *hostPath.Type {
		case v1.HostPathDirectoryOrCreate:
			if err := os.MkdirAll(hostPath.Path, kubeDirectoryPermission); err != nil {
				return nil, err
			}
			// Label a newly created volume
			if err := libpod.LabelVolumePath(hostPath.Path); err != nil {
				return nil, fmt.Errorf("giving %s a label: %w", hostPath.Path, err)
			}
		case v1.HostPathFileOrCreate:
			if _, err := os.Stat(hostPath.Path); os.IsNotExist(err) {
				f, err := os.OpenFile(hostPath.Path, os.O_RDONLY|os.O_CREATE, kubeFilePermission)
				if err != nil {
					return nil, fmt.Errorf("creating HostPath: %w", err)
				}
				if err := f.Close(); err != nil {
					logrus.Warnf("Error in closing newly created HostPath file: %v", err)
				}
			}
			// unconditionally label a newly created volume
			if err := libpod.LabelVolumePath(hostPath.Path); err != nil {
				return nil, fmt.Errorf("giving %s a label: %w", hostPath.Path, err)
			}
		case v1.HostPathSocket:
			st, err := os.Stat(hostPath.Path)
			if err != nil {
				return nil, fmt.Errorf("checking HostPathSocket: %w", err)
			}
			if st.Mode()&os.ModeSocket != os.ModeSocket {
				return nil, fmt.Errorf("checking HostPathSocket: path %s is not a socket", hostPath.Path)
			}
		case v1.HostPathBlockDev:
			dev, err := os.Stat(hostPath.Path)
			if err != nil {
				return nil, fmt.Errorf("checking HostPathBlockDevice: %w", err)
			}
			if dev.Mode()&os.ModeCharDevice == os.ModeCharDevice {
				return nil, fmt.Errorf("checking HostPathDevice: path %s is not a block device", hostPath.Path)
			}
			return &KubeVolume{
				Type:   KubeVolumeTypeBlockDevice,
				Source: hostPath.Path,
			}, nil
		case v1.HostPathCharDev:
			dev, err := os.Stat(hostPath.Path)
			if err != nil {
				return nil, fmt.Errorf("checking HostPathCharDevice: %w", err)
			}
			if dev.Mode()&os.ModeCharDevice != os.ModeCharDevice {
				return nil, fmt.Errorf("checking HostPathCharDevice: path %s is not a character device", hostPath.Path)
			}
			return &KubeVolume{
				Type:   KubeVolumeTypeCharDevice,
				Source: hostPath.Path,
			}, nil
		case v1.HostPathDirectory:
		case v1.HostPathFile:
		case v1.HostPathUnset:
			// do nothing here because we will verify the path exists in validateVolumeHostDir
			break
		default:
			return nil, fmt.Errorf("invalid HostPath type %v", hostPath.Type)
		}
	}

	if err := parse.ValidateVolumeHostDir(hostPath.Path); err != nil {
		return nil, fmt.Errorf("in parsing HostPath in YAML: %w", err)
	}

	return &KubeVolume{
		Type:   KubeVolumeTypeBindMount,
		Source: hostPath.Path,
	}, nil
}

// VolumeFromSecret creates a new kube volume from a kube secret.
func VolumeFromSecret(secretSource *v1.SecretVolumeSource, secretsManager *secrets.SecretsManager) (*KubeVolume, error) {
	kv := &KubeVolume{
		Type:   KubeVolumeTypeSecret,
		Source: secretSource.SecretName,
		Items:  map[string][]byte{},
	}

	// returns a byte array of a kube secret data, meaning this needs to go into a string map
	_, secretByte, err := secretsManager.LookupSecretData(secretSource.SecretName)
	if err != nil {
		if errors.Is(err, secrets.ErrNoSuchSecret) && secretSource.Optional != nil && *secretSource.Optional {
			kv.Optional = true
			return kv, nil
		}
		return nil, err
	}

	data := &v1.Secret{}

	err = yaml.Unmarshal(secretByte, data)
	if err != nil {
		return nil, err
	}

	// add key: value pairs to the items array
	for key, entry := range data.Data {
		kv.Items[key] = entry
	}

	for key, entry := range data.StringData {
		kv.Items[key] = []byte(entry)
	}

	return kv, nil
}

// Create a KubeVolume from a PersistentVolumeClaimVolumeSource
func VolumeFromPersistentVolumeClaim(claim *v1.PersistentVolumeClaimVolumeSource) (*KubeVolume, error) {
	return &KubeVolume{
		Type:   KubeVolumeTypeNamed,
		Source: claim.ClaimName,
	}, nil
}

func VolumeFromConfigMap(configMapVolumeSource *v1.ConfigMapVolumeSource, configMaps []v1.ConfigMap) (*KubeVolume, error) {
	var configMap *v1.ConfigMap
	kv := &KubeVolume{
		Type:  KubeVolumeTypeConfigMap,
		Items: map[string][]byte{},
	}
	for _, cm := range configMaps {
		if cm.Name == configMapVolumeSource.Name {
			matchedCM := cm
			// Set the source to the config map name
			kv.Source = cm.Name
			configMap = &matchedCM
			break
		}
	}

	if configMap == nil {
		// If the volumeSource was optional, move on even if a matching configmap wasn't found
		if configMapVolumeSource.Optional != nil && *configMapVolumeSource.Optional {
			kv.Source = configMapVolumeSource.Name
			kv.Optional = *configMapVolumeSource.Optional
			return kv, nil
		}
		return nil, fmt.Errorf("no such ConfigMap %q", configMapVolumeSource.Name)
	}

	// don't allow keys from "data" and "binaryData" to overlap
	for k := range configMap.Data {
		if _, ok := configMap.BinaryData[k]; ok {
			return nil, fmt.Errorf("the ConfigMap %q is invalid: duplicate key %q present in data and binaryData", configMap.Name, k)
		}
	}

	// If there are Items specified in the volumeSource, that overwrites the Data from the configmap
	if len(configMapVolumeSource.Items) > 0 {
		for _, item := range configMapVolumeSource.Items {
			if val, ok := configMap.Data[item.Key]; ok {
				kv.Items[item.Path] = []byte(val)
			} else if val, ok := configMap.BinaryData[item.Key]; ok {
				kv.Items[item.Path] = val
			}
		}
	} else {
		for k, v := range configMap.Data {
			kv.Items[k] = []byte(v)
		}
		for k, v := range configMap.BinaryData {
			kv.Items[k] = v
		}
	}
	return kv, nil
}

// Create a kubeVolume for an emptyDir volume
func VolumeFromEmptyDir(emptyDirVolumeSource *v1.EmptyDirVolumeSource, name string) (*KubeVolume, error) {
	return &KubeVolume{Type: KubeVolumeTypeEmptyDir, Source: name}, nil
}

// Create a KubeVolume from one of the supported VolumeSource
func VolumeFromSource(volumeSource v1.VolumeSource, configMaps []v1.ConfigMap, secretsManager *secrets.SecretsManager, volName string) (*KubeVolume, error) {
	switch {
	case volumeSource.HostPath != nil:
		return VolumeFromHostPath(volumeSource.HostPath)
	case volumeSource.PersistentVolumeClaim != nil:
		return VolumeFromPersistentVolumeClaim(volumeSource.PersistentVolumeClaim)
	case volumeSource.ConfigMap != nil:
		return VolumeFromConfigMap(volumeSource.ConfigMap, configMaps)
	case volumeSource.Secret != nil:
		return VolumeFromSecret(volumeSource.Secret, secretsManager)
	case volumeSource.EmptyDir != nil:
		return VolumeFromEmptyDir(volumeSource.EmptyDir, volName)
	default:
		return nil, errors.New("HostPath, ConfigMap, EmptyDir, Secret, and PersistentVolumeClaim are currently the only supported VolumeSource")
	}
}

// Create a map of volume name to KubeVolume
func InitializeVolumes(specVolumes []v1.Volume, configMaps []v1.ConfigMap, secretsManager *secrets.SecretsManager) (map[string]*KubeVolume, error) {
	volumes := make(map[string]*KubeVolume)

	for _, specVolume := range specVolumes {
		volume, err := VolumeFromSource(specVolume.VolumeSource, configMaps, secretsManager, specVolume.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create volume %q: %w", specVolume.Name, err)
		}

		volumes[specVolume.Name] = volume
	}

	return volumes, nil
}
