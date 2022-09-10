package registry

import (
	"fmt"
	"strings"

	"github.com/containers/podman/v4/utils"
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

// Options allows for customizing a registry.
type Options struct {
	// Image - custom registry image.
	Image string
}

// Start a new registry and return it along with it's image, user, password, and port.
func Start() (*Registry, error) {
	return StartWithOptions(nil)
}

// StartWithOptions a new registry and return it along with it's image, user, password, and port.
func StartWithOptions(options *Options) (*Registry, error) {
	if options == nil {
		options = &Options{}
	}

	var args []string
	if options.Image != "" {
		args = append(args, "-i", options.Image)
	}
	args = append(args, "start")

	// Start a registry.
	out, err := utils.ExecCmd(binary, args...)
	if err != nil {
		return nil, fmt.Errorf("running %q: %s: %w", binary, out, err)
	}

	// Parse the output.
	registry := Registry{}
	for _, s := range strings.Split(out, "\n") {
		if s == "" {
			continue
		}
		spl := strings.Split(s, "=")
		if len(spl) != 2 {
			return nil, fmt.Errorf("unexpected output format %q: want 'PODMAN_...=...'", s)
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
		return nil, fmt.Errorf("unexpected output %q: %q missing", out, ImageKey)
	}
	if registry.User == "" {
		return nil, fmt.Errorf("unexpected output %q: %q missing", out, UserKey)
	}
	if registry.Password == "" {
		return nil, fmt.Errorf("unexpected output %q: %q missing", out, PassKey)
	}
	if registry.Port == "" {
		return nil, fmt.Errorf("unexpected output %q: %q missing", out, PortKey)
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
		return fmt.Errorf("stopping registry (%v) with %q: %w", *r, binary, err)
	}
	r.running = false
	return nil
}
