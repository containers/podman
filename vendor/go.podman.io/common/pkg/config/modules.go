package config

// LoadedModules returns absolute paths to loaded containers.conf modules.
func (c *Config) LoadedModules() []string {
	// Required for conmon's callback to Podman's cleanup.
	// Absolute paths make loading the modules a bit faster.
	return c.loadedModules
}

// ModuleDirectories return the directories to load modules from:
// 1) XDG_CONFIG_HOME/HOME if rootless
// 2) /etc/
// 3) /usr/share.
func ModuleDirectories() ([]string, error) { // Public API for shell completions in Podman
	// FIXME: expose this again
	return nil, nil
}
