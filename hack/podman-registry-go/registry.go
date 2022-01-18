package registry

import (
	"strings"

	"github.com/containers/podman/v4/utils"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	imageKey = "PODMAN_REGISTRY_IMAGE"
	userKey  = "PODMAN_REGISTRY_USER"
	passKey  = "PODMAN_REGISTRY_PASS"
	portKey  = "PODMAN_REGISTRY_PORT"
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
		case imageKey:
			registry.Image = val
		case userKey:
			registry.User = val
		case passKey:
			registry.Password = val
		case portKey:
			registry.Port = val
		default:
			logrus.Errorf("Unexpected podman-registry output: %q", s)
		}
	}

	// Extra sanity check.
	if registry.Image == "" {
		return nil, errors.Errorf("unexpected output %q: %q missing", out, imageKey)
	}
	if registry.User == "" {
		return nil, errors.Errorf("unexpected output %q: %q missing", out, userKey)
	}
	if registry.Password == "" {
		return nil, errors.Errorf("unexpected output %q: %q missing", out, passKey)
	}
	if registry.Port == "" {
		return nil, errors.Errorf("unexpected output %q: %q missing", out, portKey)
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
