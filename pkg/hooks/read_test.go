package hooks

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	current "github.com/containers/podman/v4/pkg/hooks/1.0.0"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestNoJSONSuffix(t *testing.T) {
	_, err := Read("abc", []string{})
	assert.Equal(t, err, ErrNoJSONSuffix)
}

func TestUnknownPath(t *testing.T) {
	_, err := Read(filepath.Join("does", "not", "exist.json"), []string{})
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^open does/not/exist.json: no such file or directory$", err.Error())
	if !os.IsNotExist(err) {
		t.Fatal("opaque wrapping for not-exist errors")
	}
}

func TestGoodFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	jsonPath := filepath.Join(dir, "hook.json")
	err = ioutil.WriteFile(jsonPath, []byte(fmt.Sprintf("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"%s\"}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"]}", path)), 0644)
	if err != nil {
		t.Fatal(err)
	}

	hook, err := Read(jsonPath, []string{})
	if err != nil {
		t.Fatal(err)
	}
	always := true
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: path,
		},
		When: current.When{
			Always: &always,
		},
		Stages: []string{"prestart"},
	}, hook)
}

func TestBadFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "hook.json")
	err = ioutil.WriteFile(path, []byte("{\"version\": \"1.0.0\", \"hook\": \"not-a-string\"}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Read(path, []string{})
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^parsing hook \"[^\"]*hook.json\": 1.0.0: json: cannot unmarshal string into Go struct field Hook.hook of type specs.Hook$", err.Error())
}

func TestGoodBytes(t *testing.T) {
	hook, err := read([]byte("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"/a/b/c\"}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"]}"))
	if err != nil {
		t.Fatal(err)
	}
	always := true
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
		},
		When: current.When{
			Always: &always,
		},
		Stages: []string{"prestart"},
	}, hook)
}

func TestInvalidJSON(t *testing.T) {
	_, err := read([]byte("{"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^version check: unexpected end of JSON input$", err.Error())
}

func TestInvalidVersion(t *testing.T) {
	_, err := read([]byte("{\"version\": \"-1\"}"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^unrecognized hook version: \"-1\"$", err.Error())
}

func TestInvalidCurrentJSON(t *testing.T) {
	_, err := read([]byte("{\"version\": \"1.0.0\", \"hook\": \"not-a-string\"}"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^1.0.0: json: cannot unmarshal string into Go struct field Hook.hook of type specs.Hook$", err.Error())
}

func TestGoodDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = ioutil.WriteFile(filepath.Join(dir, "README"), []byte("not a hook"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	jsonPath := filepath.Join(dir, "a.json")
	err = ioutil.WriteFile(jsonPath, []byte(fmt.Sprintf("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"%s\"}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"]}", path)), 0644)
	if err != nil {
		t.Fatal(err)
	}

	hooks := map[string]*current.Hook{}
	err = ReadDir(dir, []string{}, hooks)
	if err != nil {
		t.Fatal(err)
	}

	always := true
	assert.Equal(t, map[string]*current.Hook{
		"a.json": {
			Version: current.Version,
			Hook: rspec.Hook{
				Path: path,
			},
			When: current.When{
				Always: &always,
			},
			Stages: []string{"prestart"},
		},
	}, hooks)
}

func TestUnknownDir(t *testing.T) {
	hooks := map[string]*current.Hook{}
	err := ReadDir(filepath.Join("does", "not", "exist"), []string{}, hooks)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^open does/not/exist: no such file or directory$", err.Error())
	if !os.IsNotExist(err) {
		t.Fatal("opaque wrapping for not-exist errors")
	}
}

func TestBadDir(t *testing.T) {
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

	hooks := map[string]*current.Hook{}
	err = ReadDir(dir, []string{}, hooks)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^parsing hook \"[^\"]*a.json\": unrecognized hook version: \"-1\"$", err.Error())
}

func TestHookExecutableDoesNotExit(t *testing.T) {
	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	jsonPath := filepath.Join(dir, "hook.json")
	err = ioutil.WriteFile(jsonPath, []byte("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"/does/not/exist\"}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"]}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	hooks := map[string]*current.Hook{}
	err = ReadDir(dir, []string{}, hooks)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^stat /does/not/exist: no such file or directory$", err.Error())
}
