package hooks

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestMonitorGood(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	manager, err := New(ctx, []string{dir})
	if err != nil {
		t.Fatal(err)
	}

	sync := make(chan error, 2)
	go manager.Monitor(ctx, sync)
	err = <-sync
	if err != nil {
		t.Fatal(err)
	}

	jsonPath := filepath.Join(dir, "a.json")

	t.Run("good-addition", func(t *testing.T) {
		err = ioutil.WriteFile(jsonPath, []byte(fmt.Sprintf("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"%s\"}, \"when\": {\"always\": true}, \"stages\": [\"prestart\", \"poststart\", \"poststop\"]}", path)), 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

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
	})

	t.Run("good-removal", func(t *testing.T) {
		err = os.Remove(jsonPath)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		expected := config.Hooks
		err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expected, config.Hooks)
	})

	t.Run("bad-addition", func(t *testing.T) {
		err = ioutil.WriteFile(jsonPath, []byte("{\"version\": \"-1\"]}"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		expected := config.Hooks
		err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expected, config.Hooks)

		err = os.Remove(jsonPath)
		if err != nil {
			t.Fatal(err)
		}
	})

	cancel()
	err = <-sync
	assert.Equal(t, context.Canceled, err)
}

func TestMonitorBadWatcher(t *testing.T) {
	ctx := context.Background()
	manager, err := New(ctx, []string{})
	if err != nil {
		t.Fatal(err)
	}
	manager.directories = []string{"/does/not/exist"}

	sync := make(chan error, 2)
	go manager.Monitor(ctx, sync)
	err = <-sync
	if !os.IsNotExist(err) {
		t.Fatal("opaque wrapping for not-exist errors")
	}
}
