package util

import (
	"github.com/containers/buildah/define"
)

const (
	// DefaultRuntime if containers.conf fails.
	DefaultRuntime = define.DefaultRuntime
)

var (
	// DefaultCapabilities is the list of capabilities which we grant by
	// default to containers which are running under UID 0.
	DefaultCapabilities = define.DefaultCapabilities

	// DefaultNetworkSysctl is the list of Kernel parameters which we
	// grant by default to containers which are running under UID 0.
	DefaultNetworkSysctl = define.DefaultNetworkSysctl
)
