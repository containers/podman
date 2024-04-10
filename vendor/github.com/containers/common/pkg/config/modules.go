package config

import (
	"fmt"
	"path/filepath"

	"github.com/containers/storage/pkg/fileutils"
	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/unshare"
	"github.com/hashicorp/go-multierror"
)

// The subdirectory for looking up containers.conf modules.
const moduleSubdir = "containers/containers.conf.modules"

// Moving the base paths into variables allows for overriding them in units
// tests.
var (
	moduleBaseEtc = "/etc/"
	moduleBaseUsr = "/usr/share"
)

// LoadedModules returns absolute paths to loaded containers.conf modules.
func (c *Config) LoadedModules() []string {
	// Required for conmon's callback to Podman's cleanup.
	// Absolute paths make loading the modules a bit faster.
	return c.loadedModules
}

// Find the specified modules in the options.  Return an error if a specific
// module cannot be located on the host.
func (o *Options) modules() ([]string, error) {
	if len(o.Modules) == 0 {
		return nil, nil
	}

	dirs, err := ModuleDirectories()
	if err != nil {
		return nil, err
	}

	configs := make([]string, 0, len(o.Modules))
	for _, path := range o.Modules {
		resolved, err := resolveModule(path, dirs)
		if err != nil {
			return nil, fmt.Errorf("could not resolve module %q: %w", path, err)
		}
		configs = append(configs, resolved)
	}

	return configs, nil
}

// ModuleDirectories return the directories to load modules from:
// 1) XDG_CONFIG_HOME/HOME if rootless
// 2) /etc/
// 3) /usr/share
func ModuleDirectories() ([]string, error) { // Public API for shell completions in Podman
	modules := []string{
		filepath.Join(moduleBaseEtc, moduleSubdir),
		filepath.Join(moduleBaseUsr, moduleSubdir),
	}

	if !unshare.IsRootless() {
		return modules, nil
	}

	// Prepend the user modules dir.
	configHome, err := homedir.GetConfigHome()
	if err != nil {
		return nil, err
	}
	return append([]string{filepath.Join(configHome, moduleSubdir)}, modules...), nil
}

// Resolve the specified path to a module.
func resolveModule(path string, dirs []string) (string, error) {
	if filepath.IsAbs(path) {
		err := fileutils.Exists(path)
		return path, err
	}

	// Collect all errors to avoid suppressing important errors (e.g.,
	// permission errors).
	var multiErr error
	for _, d := range dirs {
		candidate := filepath.Join(d, path)
		err := fileutils.Exists(candidate)
		if err == nil {
			return candidate, nil
		}
		multiErr = multierror.Append(multiErr, err)
	}
	return "", multiErr
}
