// Package hooks implements hook configuration and handling for CRI-O and libpod.
package hooks

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	current "github.com/containers/libpod/pkg/hooks/1.0.0"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
//
// extensionStages allows callers to add additional stages beyond
// those specified in the OCI Runtime Specification and to control
// OCI-defined stages instead of delagating to the OCI runtime.  See
// Hooks() for more information.
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
//
// If extensionStages was set when initializing the Manager,
// matching hooks requesting those stages will be returned in
// extensionStageHooks.  This takes precedence over their inclusion in
// the OCI configuration.  For example:
//
//   manager, err := New(ctx, []string{DefaultDir}, []string{"poststop"}, lang)
//   extensionStageHooks, err := manager.Hooks(config, annotations, hasBindMounts)
//
// will have any matching post-stop hooks in extensionStageHooks and
// will not insert them into config.Hooks.Poststop.
func (m *Manager) Hooks(config *rspec.Spec, annotations map[string]string, hasBindMounts bool) (extensionStageHooks map[string][]rspec.Hook, err error) {
	hooks := m.namedHooks()
	collator := collate.New(m.language, collate.IgnoreCase, collate.IgnoreWidth)
	collator.Sort(namedHooks(hooks))
	localStages := map[string]bool{} // stages destined for extensionStageHooks
	for _, stage := range m.extensionStages {
		localStages[stage] = true
	}
	for _, namedHook := range hooks {
		match, err := namedHook.hook.When.Match(config, annotations, hasBindMounts)
		if err != nil {
			return extensionStageHooks, errors.Wrapf(err, "matching hook %q", namedHook.name)
		}
		if match {
			logrus.Debugf("hook %s matched; adding to stages %v", namedHook.name, namedHook.hook.Stages)
			if config.Hooks == nil {
				config.Hooks = &rspec.Hooks{}
			}
			for _, stage := range namedHook.hook.Stages {
				if _, ok := localStages[stage]; ok {
					if extensionStageHooks == nil {
						extensionStageHooks = map[string][]rspec.Hook{}
					}
					extensionStageHooks[stage] = append(extensionStageHooks[stage], namedHook.hook.Hook)
				} else {
					switch stage {
					case "prestart":
						config.Hooks.Prestart = append(config.Hooks.Prestart, namedHook.hook.Hook)
					case "poststart":
						config.Hooks.Poststart = append(config.Hooks.Poststart, namedHook.hook.Hook)
					case "poststop":
						config.Hooks.Poststop = append(config.Hooks.Poststop, namedHook.hook.Hook)
					default:
						return extensionStageHooks, fmt.Errorf("hook %q: unknown stage %q", namedHook.name, stage)
					}
				}
			}
		} else {
			logrus.Debugf("hook %s did not match", namedHook.name)
		}
	}

	return extensionStageHooks, nil
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
