package kube

import (
	"os"

	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/podman/v2/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

const (
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	kubeDirectoryPermission = 0755
	// https://kubernetes.io/docs/concepts/storage/volumes/#hostpath
	kubeFilePermission = 0644
)

type KubeVolumeType int

const (
	KubeVolumeTypeBindMount KubeVolumeType = iota
	KubeVolumeTypeNamed     KubeVolumeType = iota
)

type KubeVolume struct {
	// Type of volume to create
	Type KubeVolumeType
	// Path for bind mount or volume name for named volume
	Source string
}

// Create a KubeVolume from an HostPathVolumeSource
func VolumeFromHostPath(hostPath *v1.HostPathVolumeSource) (*KubeVolume, error) {
	if hostPath.Type != nil {
		switch *hostPath.Type {
		case v1.HostPathDirectoryOrCreate:
			if _, err := os.Stat(hostPath.Path); os.IsNotExist(err) {
				if err := os.Mkdir(hostPath.Path, kubeDirectoryPermission); err != nil {
					return nil, err
				}
			}
			// Label a newly created volume
			if err := libpod.LabelVolumePath(hostPath.Path); err != nil {
				return nil, errors.Wrapf(err, "error giving %s a label", hostPath.Path)
			}
		case v1.HostPathFileOrCreate:
			if _, err := os.Stat(hostPath.Path); os.IsNotExist(err) {
				f, err := os.OpenFile(hostPath.Path, os.O_RDONLY|os.O_CREATE, kubeFilePermission)
				if err != nil {
					return nil, errors.Wrap(err, "error creating HostPath")
				}
				if err := f.Close(); err != nil {
					logrus.Warnf("Error in closing newly created HostPath file: %v", err)
				}
			}
			// unconditionally label a newly created volume
			if err := libpod.LabelVolumePath(hostPath.Path); err != nil {
				return nil, errors.Wrapf(err, "error giving %s a label", hostPath.Path)
			}
		case v1.HostPathSocket:
			st, err := os.Stat(hostPath.Path)
			if err != nil {
				return nil, errors.Wrap(err, "error checking HostPathSocket")
			}
			if st.Mode()&os.ModeSocket != os.ModeSocket {
				return nil, errors.Errorf("error checking HostPathSocket: path %s is not a socket", hostPath.Path)
			}

		case v1.HostPathDirectory:
		case v1.HostPathFile:
		case v1.HostPathUnset:
			// do nothing here because we will verify the path exists in validateVolumeHostDir
			break
		default:
			return nil, errors.Errorf("Invalid HostPath type %v", hostPath.Type)
		}
	}

	if err := parse.ValidateVolumeHostDir(hostPath.Path); err != nil {
		return nil, errors.Wrapf(err, "error in parsing HostPath in YAML")
	}

	return &KubeVolume{
		Type:   KubeVolumeTypeBindMount,
		Source: hostPath.Path,
	}, nil
}

// Create a KubeVolume from a PersistentVolumeClaimVolumeSource
func VolumeFromPersistentVolumeClaim(claim *v1.PersistentVolumeClaimVolumeSource) (*KubeVolume, error) {
	return &KubeVolume{
		Type:   KubeVolumeTypeNamed,
		Source: claim.ClaimName,
	}, nil
}

// Create a KubeVolume from one of the supported VolumeSource
func VolumeFromSource(volumeSource v1.VolumeSource) (*KubeVolume, error) {
	if volumeSource.HostPath != nil {
		return VolumeFromHostPath(volumeSource.HostPath)
	} else if volumeSource.PersistentVolumeClaim != nil {
		return VolumeFromPersistentVolumeClaim(volumeSource.PersistentVolumeClaim)
	} else {
		return nil, errors.Errorf("HostPath and PersistentVolumeClaim are currently the conly supported VolumeSource")
	}
}

// Create a map of volume name to KubeVolume
func InitializeVolumes(specVolumes []v1.Volume) (map[string]*KubeVolume, error) {
	volumes := make(map[string]*KubeVolume)

	for _, specVolume := range specVolumes {
		volume, err := VolumeFromSource(specVolume.VolumeSource)
		if err != nil {
			return nil, err
		}

		volumes[specVolume.Name] = volume
	}

	return volumes, nil
}
