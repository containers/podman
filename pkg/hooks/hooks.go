package hooks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// DefaultHooksDir Default directory containing hooks config files
	DefaultHooksDir = "/usr/share/containers/oci/hooks.d"
	// OverrideHooksDir Directory where admin can override the default configuration
	OverrideHooksDir = "/etc/containers/oci/hooks.d"
)

// HookParams is the structure returned from read the hooks configuration
type HookParams struct {
	Hook          string   `json:"hook"`
	Stage         []string `json:"stage"`
	Cmds          []string `json:"cmd"`
	Annotations   []string `json:"annotation"`
	HasBindMounts bool     `json:"hasbindmounts"`
	Arguments     []string `json:"arguments"`
}

// readHook reads hooks json files, verifies it and returns the json config
func readHook(hookPath string) (HookParams, error) {
	var hook HookParams
	raw, err := ioutil.ReadFile(hookPath)
	if err != nil {
		return hook, errors.Wrapf(err, "error Reading hook %q", hookPath)
	}
	if err := json.Unmarshal(raw, &hook); err != nil {
		return hook, errors.Wrapf(err, "error Unmarshalling JSON for %q", hookPath)
	}
	if _, err := os.Stat(hook.Hook); err != nil {
		return hook, errors.Wrapf(err, "unable to stat hook %q in hook config %q", hook.Hook, hookPath)
	}
	validStage := map[string]bool{"prestart": true, "poststart": true, "poststop": true}
	for _, cmd := range hook.Cmds {
		if _, err = regexp.Compile(cmd); err != nil {
			return hook, errors.Wrapf(err, "invalid cmd regular expression %q defined in hook config %q", cmd, hookPath)
		}
	}
	for _, cmd := range hook.Annotations {
		if _, err = regexp.Compile(cmd); err != nil {
			return hook, errors.Wrapf(err, "invalid cmd regular expression %q defined in hook config %q", cmd, hookPath)
		}
	}
	for _, stage := range hook.Stage {
		if !validStage[stage] {
			return hook, errors.Wrapf(err, "unknown stage %q defined in hook config %q", stage, hookPath)
		}
	}
	return hook, nil
}

// readHooks reads hooks json files in directory to setup OCI Hooks
// adding hooks to the passedin hooks map.
func readHooks(hooksPath string, hooks map[string]HookParams) error {
	if _, err := os.Stat(hooksPath); err != nil {
		if os.IsNotExist(err) {
			logrus.Warnf("hooks path: %q does not exist", hooksPath)
			return nil
		}
		return errors.Wrapf(err, "unable to stat hooks path %q", hooksPath)
	}

	files, err := ioutil.ReadDir(hooksPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		hook, err := readHook(filepath.Join(hooksPath, file.Name()))
		if err != nil {
			return err
		}
		for key, h := range hooks {
			// hook.Hook can only be defined in one hook file, unless it has the
			// same name in the override path.
			if hook.Hook == h.Hook && key != file.Name() {
				return errors.Wrapf(syscall.EINVAL, "duplicate path,  hook %q from %q already defined in %q", hook.Hook, hooksPath, key)
			}
		}
		hooks[file.Name()] = hook
	}
	return nil
}

// SetupHooks takes a hookspath and reads all of the hooks in that directory.
// returning a map of the configured hooks
func SetupHooks(hooksPath string) (map[string]HookParams, error) {
	hooksMap := make(map[string]HookParams)
	if err := readHooks(hooksPath, hooksMap); err != nil {
		return nil, err
	}
	if hooksPath == DefaultHooksDir {
		if err := readHooks(OverrideHooksDir, hooksMap); err != nil {
			return nil, err
		}
	}

	return hooksMap, nil
}

// AddOCIHook generates OCI specification using the included hook
func AddOCIHook(g *generate.Generator, hook HookParams) error {
	for _, stage := range hook.Stage {
		h := spec.Hook{
			Path: hook.Hook,
			Args: append([]string{hook.Hook}, hook.Arguments...),
			Env:  []string{fmt.Sprintf("stage=%s", stage)},
		}
		logrus.Debugf("AddOCIHook", h)
		switch stage {
		case "prestart":
			g.AddPreStartHook(h)

		case "poststart":
			g.AddPostStartHook(h)

		case "poststop":
			g.AddPostStopHook(h)
		}
	}
	return nil
}
