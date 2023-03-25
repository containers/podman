package abi

import (
	"strings"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/util"
)

// getSdNotifyMode returns the `sdNotifyAnnotation/$name` for the specified
// name. If name is empty, it'll only look for `sdNotifyAnnotation`.
func getSdNotifyMode(annotations map[string]string, name string) (string, error) {
	var mode string
	switch len(name) {
	case 0:
		mode = annotations[sdNotifyAnnotation]
	default:
		mode = annotations[sdNotifyAnnotation+"/"+name]
	}
	return mode, define.ValidateSdNotifyMode(mode)
}

// getBuildArgs returns a map of `arg=value` for any annotation that starts
// with `BuildArgumentsAnnotationPrefix/`, as long as the resulting `arg` is not empty.
func getBuildArgs(annotations map[string]string, name string) map[string]string {
	buildArgs := make(map[string]string)

	for k, v := range annotations {
		if strings.HasPrefix(k, util.BuildArgumentsAnnotationPrefix+".") && (strings.HasSuffix(k, "/"+name) || !strings.Contains(k, "/")) {
			arg := strings.TrimPrefix(k, util.BuildArgumentsAnnotationPrefix+".")
			arg = strings.TrimSuffix(arg, "/"+name)
			if len(arg) > 0 {
				buildArgs[arg] = v
			}
		}
	}

	return buildArgs
}
