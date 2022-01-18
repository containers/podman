package hooks

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	current "github.com/containers/podman/v4/pkg/hooks/1.0.0"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

// path is the path to an example hook executable.
var path string

func TestGoodNew(t *testing.T) {
	ctx := context.Background()

	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for i, name := range []string{
		"01-my-hook.json",
		"01-UPPERCASE.json",
		"02-another-hook.json",
	} {
		jsonPath := filepath.Join(dir, name)
		var extraStages string
		if i == 0 {
			extraStages = ", \"poststart\", \"poststop\""
		}
		err = ioutil.WriteFile(jsonPath, []byte(fmt.Sprintf("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"%s\", \"timeout\": %d}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"%s]}", path, i+1, extraStages)), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	manager, err := New(ctx, []string{dir}, []string{})
	if err != nil {
		t.Fatal(err)
	}

	config := &rspec.Spec{}
	extensionStageHooks, err := manager.Hooks(config, map[string]string{}, false)
	if err != nil {
		t.Fatal(err)
	}

	one := 1
	two := 2
	three := 3
	assert.Equal(t, &rspec.Hooks{
		Prestart: []rspec.Hook{
			{
				Path:    path,
				Timeout: &one,
			},
			{
				Path:    path,
				Timeout: &two,
			},
			{
				Path:    path,
				Timeout: &three,
			},
		},
		Poststart: []rspec.Hook{
			{
				Path:    path,
				Timeout: &one,
			},
		},
		Poststop: []rspec.Hook{
			{
				Path:    path,
				Timeout: &one,
			},
		},
	}, config.Hooks)

	var nilExtensionStageHooks map[string][]rspec.Hook
	assert.Equal(t, nilExtensionStageHooks, extensionStageHooks)
}

func TestBadNew(t *testing.T) {
	ctx := context.Background()

	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	jsonPath := filepath.Join(dir, "a.json")
	err = ioutil.WriteFile(jsonPath, []byte("{\"version\": \"-1\"}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = New(ctx, []string{dir}, []string{})
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^parsing hook \"[^\"]*a.json\": unrecognized hook version: \"-1\"$", err.Error())
}

func TestBrokenMatch(t *testing.T) {
	manager := Manager{
		hooks: map[string]*current.Hook{
			"a.json": {
				Version: current.Version,
				Hook: rspec.Hook{
					Path: "/a/b/c",
				},
				When: current.When{
					Commands: []string{"["},
				},
				Stages: []string{"prestart"},
			},
		},
	}
	config := &rspec.Spec{
		Process: &rspec.Process{
			Args: []string{"/bin/sh"},
		},
	}
	extensionStageHooks, err := manager.Hooks(config, map[string]string{}, false)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^matching hook \"a\\.json\": command: error parsing regexp: .*", err.Error())

	var nilExtensionStageHooks map[string][]rspec.Hook
	assert.Equal(t, nilExtensionStageHooks, extensionStageHooks)
}

func TestInvalidStage(t *testing.T) {
	always := true
	manager := Manager{
		hooks: map[string]*current.Hook{
			"a.json": {
				Version: current.Version,
				Hook: rspec.Hook{
					Path: "/a/b/c",
				},
				When: current.When{
					Always: &always,
				},
				Stages: []string{"does-not-exist"},
			},
		},
	}
	extensionStageHooks, err := manager.Hooks(&rspec.Spec{}, map[string]string{}, false)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^hook \"a\\.json\": unknown stage \"does-not-exist\"$", err.Error())

	var nilExtensionStageHooks map[string][]rspec.Hook
	assert.Equal(t, nilExtensionStageHooks, extensionStageHooks)
}

func TestExtensionStage(t *testing.T) {
	always := true
	manager := Manager{
		hooks: map[string]*current.Hook{
			"a.json": {
				Version: current.Version,
				Hook: rspec.Hook{
					Path: "/a/b/c",
				},
				When: current.When{
					Always: &always,
				},
				Stages: []string{"prestart", "poststop", "a", "b"},
			},
		},
		extensionStages: []string{"poststop", "a", "b", "c"},
	}

	config := &rspec.Spec{}
	extensionStageHooks, err := manager.Hooks(config, map[string]string{}, false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, &rspec.Hooks{
		Prestart: []rspec.Hook{
			{
				Path: "/a/b/c",
			},
		},
	}, config.Hooks)

	assert.Equal(t, map[string][]rspec.Hook{
		"poststop": {
			{
				Path: "/a/b/c",
			},
		},
		"a": {
			{
				Path: "/a/b/c",
			},
		},
		"b": {
			{
				Path: "/a/b/c",
			},
		},
	}, extensionStageHooks)
}

func init() {
	if runtime.GOOS != "windows" {
		path = "/bin/sh"
	} else {
		panic("we need a reliable executable path on Windows")
	}
}
