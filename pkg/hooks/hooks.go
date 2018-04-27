// Package hooks implements CRI-O's hook handling.
package hooks

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	current "github.com/projectatomic/libpod/pkg/hooks/1.0.0"
)

// Version is the current hook configuration version.
const Version = current.Version

const (
	// DefaultDir is the default directory containing system hook configuration files.
	DefaultDir = "/usr/share/containers/oci/hooks.d"

	// OverrideDir is the directory for hook configuration files overriding the default entries.
	OverrideDir = "/etc/containers/oci/hooks.d"
)

// Manager provides an opaque interface for managing CRI-O hooks.
type Manager struct {
	hooks       map[string]*current.Hook
	directories []string
	lock        sync.Mutex
}

// New creates a new hook manager.  Directories are ordered by
// increasing preference (hook configurations in later directories
// override configurations with the same filename from earlier
// directories).
func New(ctx context.Context, directories []string) (manager *Manager, err error) {
	manager = &Manager{
		hooks:       map[string]*current.Hook{},
		directories: directories,
	}

	for _, dir := range directories {
		err = ReadDir(dir, manager.hooks)
		if err != nil {
			return nil, err
		}
	}

	return manager, nil
}

// Hooks injects OCI runtime hooks for a given container configuration.
func (m *Manager) Hooks(config *rspec.Spec, annotations map[string]string, hasBindMounts bool) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	for name, hook := range m.hooks {
		match, err := hook.When.Match(config, annotations, hasBindMounts)
		if err != nil {
			return errors.Wrapf(err, "matching hook %q", name)
		}
		if match {
			if config.Hooks == nil {
				config.Hooks = &rspec.Hooks{}
			}
			for _, stage := range hook.Stages {
				switch stage {
				case "prestart":
					config.Hooks.Prestart = append(config.Hooks.Prestart, hook.Hook)
				case "poststart":
					config.Hooks.Poststart = append(config.Hooks.Poststart, hook.Hook)
				case "poststop":
					config.Hooks.Poststop = append(config.Hooks.Poststop, hook.Hook)
				default:
					return fmt.Errorf("hook %q: unknown stage %q", name, stage)
				}
			}
		}
	}
	return nil
}

// remove remove a hook by name.
func (m *Manager) remove(hook string) (ok bool) {
	m.lock.Lock()
	defer m.lock.Unlock()
	_, ok = m.hooks[hook]
	if ok {
		delete(m.hooks, hook)
	}
	return ok
}

// add adds a hook by path
func (m *Manager) add(path string) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	hook, err := Read(path)
	if err != nil {
		return err
	}
	m.hooks[filepath.Base(path)] = hook
	return nil
}
