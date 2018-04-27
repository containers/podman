// Package hooks implements CRI-O's hook handling.
package hooks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	current "github.com/projectatomic/libpod/pkg/hooks/1.0.0"
)

type reader func(content []byte) (*current.Hook, error)

var (
	// ErrNoJSONSuffix represents hook-add attempts where the filename
	// does not end in '.json'.
	ErrNoJSONSuffix = errors.New("hook filename does not end in '.json'")

	// Readers registers per-version hook readers.
	Readers = map[string]reader{}
)

// Read reads a hook JSON file, verifies it, and returns the hook configuration.
func Read(path string) (*current.Hook, error) {
	if !strings.HasSuffix(path, ".json") {
		return nil, ErrNoJSONSuffix
	}
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hook, err := read(content)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing hook %q", path)
	}
	err = hook.Validate()
	return hook, err
}

func read(content []byte) (hook *current.Hook, err error) {
	var ver version
	if err := json.Unmarshal(content, &ver); err != nil {
		return nil, errors.Wrap(err, "version check")
	}
	reader, ok := Readers[ver.Version]
	if !ok {
		return nil, fmt.Errorf("unrecognized hook version: %q", ver.Version)
	}

	hook, err = reader(content)
	if err != nil {
		return hook, errors.Wrap(err, ver.Version)
	}
	return hook, err
}

// ReadDir reads hook JSON files from a directory into the given map,
// clobbering any previous entries with the same filenames.
func ReadDir(path string, hooks map[string]*current.Hook) error {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, file := range files {
		hook, err := Read(filepath.Join(path, file.Name()))
		if err != nil {
			if err == ErrNoJSONSuffix {
				continue
			}
			return err
		}
		hooks[file.Name()] = hook
	}
	return nil
}

func init() {
	Readers[current.Version] = current.Read
}
