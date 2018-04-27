package hook

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

// path is the path to an example hook executable.
var path string

func TestGoodRead(t *testing.T) {
	hook, err := Read([]byte("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"/a/b/c\"}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"]}"))
	if err != nil {
		t.Fatal(err)
	}
	always := true
	assert.Equal(t, &Hook{
		Version: Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
		},
		When: When{
			Always: &always,
		},
		Stages: []string{"prestart"},
	}, hook)
}

func TestInvalidJSON(t *testing.T) {
	_, err := Read([]byte("{"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^unexpected end of JSON input$", err.Error())
}

func TestGoodValidate(t *testing.T) {
	always := true
	hook := &Hook{
		Version: Version,
		Hook: rspec.Hook{
			Path: path,
		},
		When: When{
			Always: &always,
		},
		Stages: []string{"prestart"},
	}
	err := hook.Validate()
	if err != nil {
		t.Fatal(err)
	}
}

func TestNilValidation(t *testing.T) {
	var hook *Hook
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^nil hook$", err.Error())
}

func TestWrongVersion(t *testing.T) {
	hook := Hook{Version: "0.1.0"}
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^unexpected hook version \"0.1.0\" \\(expecting 1.0.0\\)$", err.Error())
}

func TestNoHookPath(t *testing.T) {
	hook := Hook{
		Version: "1.0.0",
		Hook:    rspec.Hook{},
	}
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^missing required property: hook.path$", err.Error())
}

func TestUnknownHookPath(t *testing.T) {
	hook := Hook{
		Version: "1.0.0",
		Hook: rspec.Hook{
			Path: filepath.Join("does", "not", "exist"),
		},
	}
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^stat does/not/exist: no such file or directory$", err.Error())
	if !os.IsNotExist(err) {
		t.Fatal("opaque wrapping for not-exist errors")
	}
}

func TestNoStages(t *testing.T) {
	hook := Hook{
		Version: "1.0.0",
		Hook: rspec.Hook{
			Path: path,
		},
	}
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^missing required property: stages$", err.Error())
}

func TestInvalidStage(t *testing.T) {
	hook := Hook{
		Version: "1.0.0",
		Hook: rspec.Hook{
			Path: path,
		},
		Stages: []string{"does-not-exist"},
	}
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^unknown stage \"does-not-exist\"$", err.Error())
}

func TestInvalidAnnotationKey(t *testing.T) {
	hook := Hook{
		Version: "1.0.0",
		Hook: rspec.Hook{
			Path: path,
		},
		When: When{
			Annotations: map[string]string{
				"[": "a",
			},
		},
		Stages: []string{"prestart"},
	}
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^invalid annotation key \"\\[\": error parsing regexp: .*", err.Error())
}

func TestInvalidAnnotationValue(t *testing.T) {
	hook := Hook{
		Version: "1.0.0",
		Hook: rspec.Hook{
			Path: path,
		},
		When: When{
			Annotations: map[string]string{
				"a": "[",
			},
		},
		Stages: []string{"prestart"},
	}
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^invalid annotation value \"\\[\": error parsing regexp: .*", err.Error())
}

func TestInvalidCommand(t *testing.T) {
	hook := Hook{
		Version: "1.0.0",
		Hook: rspec.Hook{
			Path: path,
		},
		When: When{
			Commands: []string{"["},
		},
		Stages: []string{"prestart"},
	}
	err := hook.Validate()
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^invalid command \"\\[\": error parsing regexp: .*", err.Error())
}

func init() {
	if runtime.GOOS != "windows" {
		path = "/bin/sh"
	} else {
		panic("we need a reliable executable path on Windows")
	}
}
