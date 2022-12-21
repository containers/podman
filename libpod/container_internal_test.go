package libpod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/containers/storage/pkg/idtools"
	stypes "github.com/containers/storage/types"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

// hookPath is the path to an example hook executable.
var hookPath string

func TestParseOptionIDs(t *testing.T) {
	_, err := parseOptionIDs("uids=100-200-2")
	assert.NotNil(t, err)

	mappings, err := parseOptionIDs("100-200-2")
	assert.Nil(t, err)
	assert.NotNil(t, mappings)

	assert.Equal(t, len(mappings), 1)

	assert.Equal(t, mappings[0].ContainerID, 100)
	assert.Equal(t, mappings[0].HostID, 200)
	assert.Equal(t, mappings[0].Size, 2)

	mappings, err = parseOptionIDs("100-200-2#300-400-5")
	assert.Nil(t, err)
	assert.NotNil(t, mappings)

	assert.Equal(t, len(mappings), 2)

	assert.Equal(t, mappings[0].ContainerID, 100)
	assert.Equal(t, mappings[0].HostID, 200)
	assert.Equal(t, mappings[0].Size, 2)

	assert.Equal(t, mappings[1].ContainerID, 300)
	assert.Equal(t, mappings[1].HostID, 400)
	assert.Equal(t, mappings[1].Size, 5)
}

func TestParseIDMapMountOption(t *testing.T) {
	uidMap := []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      1000,
			Size:        10000,
		},
	}
	gidMap := []idtools.IDMap{
		{
			ContainerID: 0,
			HostID:      2000,
			Size:        10000,
		},
	}
	options := stypes.IDMappingOptions{
		UIDMap: uidMap,
		GIDMap: gidMap,
	}
	uids, gids, err := parseIDMapMountOption(options, "idmap")
	assert.Nil(t, err)
	assert.Equal(t, len(uids), 1)
	assert.Equal(t, len(gids), 1)

	assert.Equal(t, uids[0].ContainerID, uint32(1000))
	assert.Equal(t, uids[0].HostID, uint32(0))
	assert.Equal(t, uids[0].Size, uint32(10000))

	assert.Equal(t, gids[0].ContainerID, uint32(2000))
	assert.Equal(t, gids[0].HostID, uint32(0))
	assert.Equal(t, gids[0].Size, uint32(10000))

	uids, gids, err = parseIDMapMountOption(options, "idmap=uids=0-1-10#10-11-10;gids=0-3-10")
	assert.Nil(t, err)
	assert.Equal(t, len(uids), 2)
	assert.Equal(t, len(gids), 1)

	assert.Equal(t, uids[0].ContainerID, uint32(1))
	assert.Equal(t, uids[0].HostID, uint32(0))
	assert.Equal(t, uids[0].Size, uint32(10))

	assert.Equal(t, uids[1].ContainerID, uint32(11))
	assert.Equal(t, uids[1].HostID, uint32(10))
	assert.Equal(t, uids[1].Size, uint32(10))

	assert.Equal(t, gids[0].ContainerID, uint32(3))
	assert.Equal(t, gids[0].HostID, uint32(0))
	assert.Equal(t, gids[0].Size, uint32(10))

	_, _, err = parseIDMapMountOption(options, "idmap=uids=0-1-10#10-11-10;gids=0-3-10;foobar=bar")
	assert.NotNil(t, err)

	_, _, err = parseIDMapMountOption(options, "idmap=uids=0-1-10#10-11-10;gids=0-3-10#0-12")
	assert.NotNil(t, err)

	_, _, err = parseIDMapMountOption(options, "idmap=uids=0-1-10#10-11-10;gids=0-3-10#0-12--12")
	assert.NotNil(t, err)

	_, _, err = parseIDMapMountOption(options, "idmap=uids=0-1-10#10-11-10;gids=0-3-10#-1-12-12")
	assert.NotNil(t, err)

	_, _, err = parseIDMapMountOption(options, "idmap=uids=0-1-10#10-11-10;gids=0-3-10#0--12-0")
	assert.NotNil(t, err)
}

func TestPostDeleteHooks(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	statePath := filepath.Join(dir, "state")
	copyPath := filepath.Join(dir, "copy")
	c := Container{
		runtime: &Runtime{},
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
			content, err := os.ReadFile(path)
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
