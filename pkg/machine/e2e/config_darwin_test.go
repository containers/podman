package e2e_test

import "github.com/containers/podman/v5/pkg/machine/define"

const podmanBinary = "../../../bin/darwin/podman"

func getOtherProvider() string {
	if isVmtype(define.AppleHvVirt) {
		return "libkrun"
	} else if isVmtype(define.LibKrun) {
		return "applehv"
	}
	return ""
}
