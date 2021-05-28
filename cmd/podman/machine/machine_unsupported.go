// +build !amd64 amd64,windows

package machine

// init do not register _podman machine_ command on unsupported platforms
func init() {}
