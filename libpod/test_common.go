package libpod

import (
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/stretchr/testify/assert"
)

// nolint
func getTestContainer(id, name, locksDir string) (*Container, error) {
	ctr := &Container{
		config: &ContainerConfig{
			ID:              id,
			Name:            name,
			RootfsImageID:   id,
			RootfsImageName: "testimg",
			ImageVolumes:    true,
			StaticDir:       "/does/not/exist/",
			LogPath:         "/does/not/exist/",
			Stdin:           true,
			Labels:          map[string]string{"a": "b", "c": "d"},
			StopSignal:      0,
			StopTimeout:     0,
			CreatedTime:     time.Now(),
			Privileged:      true,
			Mounts:          []string{"/does/not/exist"},
			DNSServer:       []net.IP{net.ParseIP("192.168.1.1"), net.ParseIP("192.168.2.2")},
			DNSSearch:       []string{"example.com", "example.example.com"},
			PortMappings: []ocicni.PortMapping{
				{
					HostPort:      80,
					ContainerPort: 90,
					Protocol:      "tcp",
					HostIP:        "192.168.3.3",
				},
				{
					HostPort:      100,
					ContainerPort: 110,
					Protocol:      "udp",
					HostIP:        "192.168.4.4",
				},
			},
		},
		state: &containerState{
			State:      ContainerStateRunning,
			ConfigPath: "/does/not/exist/specs/" + id,
			RunDir:     "/does/not/exist/tmp/",
			Mounted:    true,
			Mountpoint: "/does/not/exist/tmp/" + id,
			PID:        1234,
		},
		valid: true,
	}

	g := generate.New()
	ctr.config.Spec = g.Spec()

	ctr.config.Labels["test"] = "testing"

	// Must make lockfile or container will error on being retrieved from DB
	lockPath := filepath.Join(locksDir, id)
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, err
	}
	ctr.lock = lock

	return ctr, nil
}

// nolint
func getTestPod(id, name, locksDir string) (*Pod, error) {
	pod := &Pod{
		config: &PodConfig{
			ID:     id,
			Name:   name,
			Labels: map[string]string{"a": "b", "c": "d"},
		},
		valid: true,
	}

	lockPath := filepath.Join(locksDir, id)
	lock, err := storage.GetLockfile(lockPath)
	if err != nil {
		return nil, err
	}
	pod.lock = lock

	return pod, nil
}

// This horrible hack tests if containers are equal in a way that should handle
// empty arrays being dropped to nil pointers in the spec JSON
// nolint
func testContainersEqual(t *testing.T, a, b *Container) {
	if a == nil && b == nil {
		return
	}
	assert.NotNil(t, a)
	assert.NotNil(t, b)

	assert.NotNil(t, a.config)
	assert.NotNil(t, b.config)
	assert.NotNil(t, a.state)
	assert.NotNil(t, b.state)

	aConfig := new(ContainerConfig)
	bConfig := new(ContainerConfig)
	aState := new(containerState)
	bState := new(containerState)

	assert.Equal(t, a.valid, b.valid)

	aConfigJSON, err := json.Marshal(a.config)
	assert.NoError(t, err)
	err = json.Unmarshal(aConfigJSON, aConfig)
	assert.NoError(t, err)

	bConfigJSON, err := json.Marshal(b.config)
	assert.NoError(t, err)
	err = json.Unmarshal(bConfigJSON, bConfig)
	assert.NoError(t, err)

	assert.EqualValues(t, aConfig, bConfig)

	aStateJSON, err := json.Marshal(a.state)
	assert.NoError(t, err)
	err = json.Unmarshal(aStateJSON, aState)
	assert.NoError(t, err)

	bStateJSON, err := json.Marshal(b.state)
	assert.NoError(t, err)
	err = json.Unmarshal(bStateJSON, bState)
	assert.NoError(t, err)

	assert.EqualValues(t, aState, bState)
}
