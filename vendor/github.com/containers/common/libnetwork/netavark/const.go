//go:build linux || freebsd
// +build linux freebsd

package netavark

const defaultBridgeName = "podman"

const defaultRootLockPath = "/run/lock/netavark.lock"
