package util

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"

	v1 "github.com/containers/podman/v5/pkg/k8s.io/api/core/v1"
	metav1 "github.com/containers/podman/v5/pkg/k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
	yamlv3 "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

const (
	// Kube annotation for podman volume driver.
	VolumeDriverAnnotation = "volume.podman.io/driver"
	// Kube annotation for podman volume type.
	VolumeTypeAnnotation = "volume.podman.io/type"
	// Kube annotation for podman volume device.
	VolumeDeviceAnnotation = "volume.podman.io/device"
	// Kube annotation for podman volume UID.
	VolumeUIDAnnotation = "volume.podman.io/uid"
	// Kube annotation for podman volume GID.
	VolumeGIDAnnotation = "volume.podman.io/gid"
	// Kube annotation for podman volume mount options.
	VolumeMountOptsAnnotation = "volume.podman.io/mount-options"
	// Kube annotation for podman volume import source.
	VolumeImportSourceAnnotation = "volume.podman.io/import-source"
	// Kube annotation for podman volume image.
	VolumeImageAnnotation = "volume.podman.io/image"
)

// SplitMultiDocYAML reads multiple documents in a YAML file and
// returns them as a list.
func SplitMultiDocYAML(yamlContent []byte) ([][]byte, error) {
	var documentList [][]byte

	d := yamlv3.NewDecoder(bytes.NewReader(yamlContent))
	for {
		var o interface{}
		// read individual document
		err := d.Decode(&o)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("multi doc yaml could not be split: %w", err)
		}

		if o == nil {
			continue
		}

		// back to bytes
		document, err := yamlv3.Marshal(o)
		if err != nil {
			return nil, fmt.Errorf("individual doc yaml could not be marshalled: %w", err)
		}

		kind, err := GetKubeKind(document)
		if err != nil {
			return nil, fmt.Errorf("couldn't get object kind: %w", err)
		}

		// The items in a document of kind "List" are fully qualified resources
		// So, they can be treated as separate documents
		if kind == "List" {
			var kubeList metav1.List
			if err := yaml.Unmarshal(document, &kubeList); err != nil {
				return nil, err
			}
			for _, item := range kubeList.Items {
				itemDocument, err := yamlv3.Marshal(item)
				if err != nil {
					return nil, fmt.Errorf("individual doc yaml could not be marshalled: %w", err)
				}

				documentList = append(documentList, itemDocument)
			}
		} else {
			documentList = append(documentList, document)
		}
	}

	return documentList, nil
}

// GetKubeKind unmarshals a kube YAML document and returns its kind.
func GetKubeKind(obj []byte) (string, error) {
	var kubeObject v1.ObjectReference

	if err := yaml.Unmarshal(obj, &kubeObject); err != nil {
		return "", err
	}

	return kubeObject.Kind, nil
}

func imageNamePrefix(imageName string) string {
	prefix := imageName
	s := strings.Split(prefix, ":")
	if len(s) > 0 {
		prefix = s[0]
	}
	s = strings.Split(prefix, "/")
	if len(s) > 0 {
		prefix = s[len(s)-1]
	}
	s = strings.Split(prefix, "@")
	if len(s) > 0 {
		prefix = s[0]
	}
	return prefix
}

func GetBuildFile(imageName string, cwd string) (string, error) {
	buildDirName := imageNamePrefix(imageName)
	containerfilePath := filepath.Join(cwd, buildDirName, "Containerfile")
	dockerfilePath := filepath.Join(cwd, buildDirName, "Dockerfile")

	err := fileutils.Exists(containerfilePath)
	if err == nil {
		logrus.Debugf("Building %s with %s", imageName, containerfilePath)
		return containerfilePath, nil
	}
	// If the error is not because the file does not exist, take
	// a mulligan and try Dockerfile.  If that also fails, return that
	// error
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		logrus.Error(err.Error())
	}

	err = fileutils.Exists(dockerfilePath)
	if err == nil {
		logrus.Debugf("Building %s with %s", imageName, dockerfilePath)
		return dockerfilePath, nil
	}
	// Strike two
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return "", err
}
