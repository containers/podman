// Package hooks implements hook configuration and handling for CRI-O and libpod.
package hooks

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	current "github.com/projectatomic/libpod/pkg/hooks/1.0.0"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
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
	hooks           map[string]*current.Hook
	directories     []string
	extensionStages []string
	language        language.Tag
	lock            sync.Mutex
}

type namedHook struct {
	name string
	hook *current.Hook
}

type namedHooks []*namedHook

// New creates a new hook manager.  Directories are ordered by
// increasing preference (hook configurations in later directories
// override configurations with the same filename from earlier
// directories).
func New(ctx context.Context, directories []string, extensionStages []string, lang language.Tag) (manager *Manager, err error) {
	manager = &Manager{
		hooks:           map[string]*current.Hook{},
		directories:     directories,
		extensionStages: extensionStages,
		language:        lang,
	}

	for _, dir := range directories {
		err = ReadDir(dir, manager.extensionStages, manager.hooks)
		if err != nil {
			return nil, err
		}
	}

	return manager, nil
}

// filenames returns sorted hook entries.
func (m *Manager) namedHooks() (hooks []*namedHook) {
	m.lock.Lock()
	defer m.lock.Unlock()

	hooks = make([]*namedHook, len(m.hooks))
	i := 0
	for name, hook := range m.hooks {
		hooks[i] = &namedHook{
			name: name,
			hook: hook,
		}
		i++
	}

	return hooks
}

// Hooks injects OCI runtime hooks for a given container configuration.
func (m *Manager) Hooks(config *rspec.Spec, annotations map[string]string, hasBindMounts bool) (extensionStages map[string][]rspec.Hook, err error) {
	hooks := m.namedHooks()
	collator := collate.New(m.language, collate.IgnoreCase, collate.IgnoreWidth)
	collator.Sort(namedHooks(hooks))
	validStages := map[string]bool{} // beyond the OCI stages
	for _, stage := range m.extensionStages {
		validStages[stage] = true
	}
	for _, namedHook := range hooks {
		match, err := namedHook.hook.When.Match(config, annotations, hasBindMounts)
		if err != nil {
			return extensionStages, errors.Wrapf(err, "matching hook %q", namedHook.name)
		}
		if match {
			if config.Hooks == nil {
				config.Hooks = &rspec.Hooks{}
			}
			for _, stage := range namedHook.hook.Stages {
				switch stage {
				case "prestart":
					config.Hooks.Prestart = append(config.Hooks.Prestart, namedHook.hook.Hook)
				case "poststart":
					config.Hooks.Poststart = append(config.Hooks.Poststart, namedHook.hook.Hook)
				case "poststop":
					config.Hooks.Poststop = append(config.Hooks.Poststop, namedHook.hook.Hook)
				default:
					if !validStages[stage] {
						return extensionStages, fmt.Errorf("hook %q: unknown stage %q", namedHook.name, stage)
					}
					if extensionStages == nil {
						extensionStages = map[string][]rspec.Hook{}
					}
					extensionStages[stage] = append(extensionStages[stage], namedHook.hook.Hook)
				}
			}
		}
	}

	return extensionStages, nil
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
	hook, err := Read(path, m.extensionStages)
	if err != nil {
		return err
	}
	m.hooks[filepath.Base(path)] = hook
	return nil
}

// Len is part of the collate.Lister interface.
func (hooks namedHooks) Len() int {
	return len(hooks)
}

// Swap is part of the collate.Lister interface.
func (hooks namedHooks) Swap(i, j int) {
	hooks[i], hooks[j] = hooks[j], hooks[i]
}

// Bytes is part of the collate.Lister interface.
func (hooks namedHooks) Bytes(i int) []byte {
	return []byte(hooks[i].name)
}
