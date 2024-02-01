//go:build !remote

package kube

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containers/podman/v5/libpod"
	v1 "github.com/containers/podman/v5/pkg/k8s.io/api/core/v1"
)

// KubeSeccompPaths holds information about a pod YAML's seccomp configuration
// it holds both container and pod seccomp paths
//
//nolint:revive
type KubeSeccompPaths struct {
	containerPaths map[string]string
	podPath        string
}

// FindForContainer checks whether a container has a seccomp path configured for it
// if not, it returns the podPath, which should always have a value
func (k *KubeSeccompPaths) FindForContainer(ctrName string) string {
	if path, ok := k.containerPaths[ctrName]; ok {
		return path
	}
	return k.podPath
}

// InitializeSeccompPaths takes annotations from the pod object metadata and finds annotations pertaining to seccomp
// it parses both pod and container level
// if the annotation is of the form "localhost/%s", the seccomp profile will be set to profileRoot/%s
func InitializeSeccompPaths(annotations map[string]string, profileRoot string) (*KubeSeccompPaths, error) {
	seccompPaths := &KubeSeccompPaths{containerPaths: make(map[string]string)}
	var err error
	if annotations != nil {
		for annKeyValue, seccomp := range annotations {
			// check if it is prefaced with container.seccomp.security.alpha.kubernetes.io/
			prefixAndCtr := strings.Split(annKeyValue, "/")
			if prefixAndCtr[0]+"/" != v1.SeccompContainerAnnotationKeyPrefix {
				continue
			} else if len(prefixAndCtr) != 2 {
				// this could be caused by a user inputting either of
				// container.seccomp.security.alpha.kubernetes.io{,/}
				// both of which are invalid
				return nil, fmt.Errorf("invalid seccomp path: %s", prefixAndCtr[0])
			}

			path, err := verifySeccompPath(seccomp, profileRoot)
			if err != nil {
				return nil, err
			}
			seccompPaths.containerPaths[prefixAndCtr[1]] = path
		}

		podSeccomp, ok := annotations[v1.SeccompPodAnnotationKey]
		if ok {
			seccompPaths.podPath, err = verifySeccompPath(podSeccomp, profileRoot)
		} else {
			seccompPaths.podPath, err = libpod.DefaultSeccompPath()
		}
		if err != nil {
			return nil, err
		}
	}
	return seccompPaths, nil
}

// verifySeccompPath takes a path and checks whether it is a default, unconfined, or a path
// the available options are parsed as defined in https://kubernetes.io/docs/concepts/policy/pod-security-policy/#seccomp
func verifySeccompPath(path string, profileRoot string) (string, error) {
	switch path {
	case v1.DeprecatedSeccompProfileDockerDefault:
		fallthrough
	case v1.SeccompProfileRuntimeDefault:
		return libpod.DefaultSeccompPath()
	case "unconfined":
		return path, nil
	default:
		parts := strings.Split(path, "/")
		if parts[0] == "localhost" {
			return filepath.Join(profileRoot, parts[1]), nil
		}
		return "", fmt.Errorf("invalid seccomp path: %s", path)
	}
}
