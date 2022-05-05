package libpod

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

// hookPath is the path to an example hook executable.
var hookPath string

func TestPostDeleteHooks(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	statePath := filepath.Join(dir, "state")
	copyPath := filepath.Join(dir, "copy")
	c := Container{
		config: &ContainerConfig{
			ID: "123abc",
			Spec: &rspec.Spec{
				Annotations: map[string]string{
					"a": "b",
				},
			},
			ContainerRootFSConfig: ContainerRootFSConfig{
				StaticDir: dir, // not the bundle, but good enough for this test
			},
		},
		state: &ContainerState{
			ExtensionStageHooks: map[string][]rspec.Hook{
				"poststop": {
					rspec.Hook{
						Path: hookPath,
						Args: []string{"sh", "-c", fmt.Sprintf("cat >%s", statePath)},
					},
					rspec.Hook{
						Path: "/does/not/exist",
					},
					rspec.Hook{
						Path: hookPath,
						Args: []string{"sh", "-c", fmt.Sprintf("cp %s %s", statePath, copyPath)},
					},
				},
			},
		},
	}
	err := c.postDeleteHooks(ctx)
	if err != nil {
		t.Fatal(err)
	}

	stateRegexp := `{"ociVersion":"1\.0\.2-dev","id":"123abc","status":"stopped","bundle":"` + dir + `","annotations":{"a":"b"}}`
	for _, p := range []string{statePath, copyPath} {
		path := p
		t.Run(path, func(t *testing.T) {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			assert.Regexp(t, stateRegexp, string(content))
		})
	}
}

func init() {
	if runtime.GOOS != "windows" {
		hookPath = "/bin/sh"
	} else {
		panic("we need a reliable executable path on Windows")
	}
}
