package hooks

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	current "github.com/projectatomic/libpod/pkg/hooks/1.0.0"
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

	jsonPath := filepath.Join(dir, "a.json")
	err = ioutil.WriteFile(jsonPath, []byte(fmt.Sprintf("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"%s\"}, \"when\": {\"always\": true}, \"stages\": [\"prestart\", \"poststart\", \"poststop\"]}", path)), 0644)
	if err != nil {
		t.Fatal(err)
	}

	manager, err := New(ctx, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	config := &rspec.Spec{}
	err = manager.Hooks(config, map[string]string{}, false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, &rspec.Hooks{
		Prestart: []rspec.Hook{
			{
				Path: path,
			},
		},
		Poststart: []rspec.Hook{
			{
				Path: path,
			},
		},
		Poststop: []rspec.Hook{
			{
				Path: path,
			},
		},
	}, config.Hooks)
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

	_, err = New(ctx, []string{dir})
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
	err := manager.Hooks(config, map[string]string{}, false)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^matching hook \"a\\.json\": command: error parsing regexp: .*", err.Error())
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
	err := manager.Hooks(&rspec.Spec{}, map[string]string{}, false)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^hook \"a\\.json\": unknown stage \"does-not-exist\"$", err.Error())
}

func init() {
	if runtime.GOOS != "windows" {
		path = "/bin/sh"
	} else {
		panic("we need a reliable executable path on Windows")
	}
}
