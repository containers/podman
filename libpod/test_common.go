package libpod

import (
	"encoding/json"
	"net"
	"path/filepath"
	"reflect"
	"time"

	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	"github.com/opencontainers/runtime-tools/generate"
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
		id:     id,
		name:   name,
		labels: map[string]string{"a": "b", "c": "d"},
		valid:  true,
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
func testContainersEqual(a, b *Container) bool {
	if a == nil && b == nil {
		return true
	} else if a == nil || b == nil {
		return false
	}

	if a.valid != b.valid {
		return false
	}

	aConfigJSON, err := json.Marshal(a.config)
	if err != nil {
		return false
	}

	bConfigJSON, err := json.Marshal(b.config)
	if err != nil {
		return false
	}

	if !reflect.DeepEqual(aConfigJSON, bConfigJSON) {
		return false
	}

	aStateJSON, err := json.Marshal(a.state)
	if err != nil {
		return false
	}

	bStateJSON, err := json.Marshal(b.state)
	if err != nil {
		return false
	}

	return reflect.DeepEqual(aStateJSON, bStateJSON)
}

// This tests pod equality
// We cannot guarantee equality in lockfile objects so we can't simply compare
// nolint
func testPodsEqual(a, b *Pod) bool {
	if a == nil && b == nil {
		return true
	} else if a == nil || b == nil {
		return false
	}

	if a.id != b.id {
		return false
	}
	if a.name != b.name {
		return false
	}
	if a.valid != b.valid {
		return false
	}

	return reflect.DeepEqual(a.labels, b.labels)
}
