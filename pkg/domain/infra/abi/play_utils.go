package abi

import "github.com/containers/podman/v4/libpod/define"

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
