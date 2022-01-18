package hook

import (
	"testing"

	current "github.com/containers/podman/v4/pkg/hooks/1.0.0"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestGood(t *testing.T) {
	hook, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stages\": [\"prestart\"], \"cmds\": [\"sh\"]}"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
		},
		When: current.When{
			Commands: []string{"sh"},
			Or:       true,
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

func TestArguments(t *testing.T) {
	hook, err := Read([]byte("{\"hook\": \"/a/b/c\", \"arguments\": [\"d\", \"e\"], \"stages\": [\"prestart\"], \"cmds\": [\"sh\"]}"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
			Args: []string{"/a/b/c", "d", "e"},
		},
		When: current.When{
			Commands: []string{"sh"},
			Or:       true,
		},
		Stages: []string{"prestart"},
	}, hook)
}

func TestEmptyObject(t *testing.T) {
	_, err := Read([]byte("{}"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^missing required property: hook$", err.Error())
}

func TestNoStages(t *testing.T) {
	_, err := Read([]byte("{\"hook\": \"/a/b/c\"}"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^missing required property: stages$", err.Error())
}

func TestStage(t *testing.T) {
	hook, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stage\": [\"prestart\"]}"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
		},
		When:   current.When{Or: true},
		Stages: []string{"prestart"},
	}, hook)
}

func TestStagesAndStage(t *testing.T) {
	_, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stages\": [\"prestart\"], \"stage\": [\"prestart\"]}"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^cannot set both 'stage' and 'stages'$", err.Error())
}

func TestCmd(t *testing.T) {
	hook, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stage\": [\"prestart\"], \"cmd\": [\"sh\"]}"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
		},
		When: current.When{
			Commands: []string{"sh"},
			Or:       true,
		},
		Stages: []string{"prestart"},
	}, hook)
}

func TestCmdsAndCmd(t *testing.T) {
	_, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stages\": [\"prestart\"], \"cmds\": [\"sh\"], \"cmd\": [\"true\"]}"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^cannot set both 'cmd' and 'cmds'$", err.Error())
}

func TestAnnotations(t *testing.T) {
	hook, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stage\": [\"prestart\"], \"annotations\": [\"a\", \"b\"]}"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
		},
		When: current.When{
			Annotations: map[string]string{".*": "a|b"},
			Or:          true,
		},
		Stages: []string{"prestart"},
	}, hook)
}

func TestAnnotation(t *testing.T) {
	hook, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stage\": [\"prestart\"], \"annotation\": [\"a\", \"b\"]}"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
		},
		When: current.When{
			Annotations: map[string]string{".*": "a|b"},
			Or:          true,
		},
		Stages: []string{"prestart"},
	}, hook)
}

func TestAnnotationsAndAnnotation(t *testing.T) {
	_, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stages\": [\"prestart\"], \"annotations\": [\"a\"], \"annotation\": [\"b\"]}"))
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^cannot set both 'annotation' and 'annotations'$", err.Error())
}

func TestHasBindMounts(t *testing.T) {
	hook, err := Read([]byte("{\"hook\": \"/a/b/c\", \"stage\": [\"prestart\"], \"hasbindmounts\": true}"))
	if err != nil {
		t.Fatal(err)
	}
	hasBindMounts := true
	assert.Equal(t, &current.Hook{
		Version: current.Version,
		Hook: rspec.Hook{
			Path: "/a/b/c",
		},
		When: current.When{
			HasBindMounts: &hasBindMounts,
			Or:            true,
		},
		Stages: []string{"prestart"},
	}, hook)
}
