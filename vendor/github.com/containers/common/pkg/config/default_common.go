//go:build !freebsd
// +build !freebsd

package config

// DefaultInitPath is the default path to the container-init binary.
var DefaultInitPath = "/usr/libexec/podman/catatonit"
