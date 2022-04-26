package registry

import (
	"strings"

	"github.com/containers/podman/v4/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	ImageKey = "PODMAN_REGISTRY_IMAGE"
	UserKey  = "PODMAN_REGISTRY_USER"
	PassKey  = "PODMAN_REGISTRY_PASS"
	PortKey  = "PODMAN_REGISTRY_PORT"
)

var binary = "podman-registry"

// Registry is locally running registry.
type Registry struct {
	// Image - container image of the registry.
	Image string
	// User - the user to authenticate against the registry.
	User string
	// Password - the accompanying password for the user.
	Password string
	// Port - the port the registry is listening to on the host.
	Port string
	// running indicates if the registry is running.
	running bool
}

// Start a new registry and return it along with it's image, user, password, and port.
func Start() (*Registry, error) {
	// Start a registry.
	out, err := utils.ExecCmd(binary, "start")
	if err != nil {
		return nil, errors.Wrapf(err, "error running %q: %s", binary, out)
	}

	// Parse the output.
	registry := Registry{}
	for _, s := range strings.Split(out, "\n") {
		if s == "" {
			continue
		}
		spl := strings.Split(s, "=")
		if len(spl) != 2 {
			return nil, errors.Errorf("unexpected output format %q: want 'PODMAN_...=...'", s)
		}
		key := spl[0]
		val := strings.TrimSuffix(strings.TrimPrefix(spl[1], "\""), "\"")
		switch key {
		case ImageKey:
			registry.Image = val
		case UserKey:
			registry.User = val
		case PassKey:
			registry.Password = val
		case PortKey:
			registry.Port = val
		default:
			logrus.Errorf("Unexpected podman-registry output: %q", s)
		}
	}

	// Extra sanity check.
	if registry.Image == "" {
		return nil, errors.Errorf("unexpected output %q: %q missing", out, ImageKey)
	}
	if registry.User == "" {
		return nil, errors.Errorf("unexpected output %q: %q missing", out, UserKey)
	}
	if registry.Password == "" {
		return nil, errors.Errorf("unexpected output %q: %q missing", out, PassKey)
	}
	if registry.Port == "" {
		return nil, errors.Errorf("unexpected output %q: %q missing", out, PortKey)
	}

	registry.running = true

	return &registry, nil
}

// Stop the registry.
func (r *Registry) Stop() error {
	// Stop a registry.
	if !r.running {
		return nil
	}
	if _, err := utils.ExecCmd(binary, "-P", r.Port, "stop"); err != nil {
		return errors.Wrapf(err, "error stopping registry (%v) with %q", *r, binary)
	}
	r.running = false
	return nil
}
