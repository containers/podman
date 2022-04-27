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

func TestMonitorOneDirGood(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	manager, err := New(ctx, []string{dir}, []string{})
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
		_, err = manager.Hooks(config, map[string]string{}, false)
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
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expected, config.Hooks)
	})

	t.Run("bad-addition", func(t *testing.T) {
		err = ioutil.WriteFile(jsonPath, []byte("{\"version\": \"-1\"}"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		expected := config.Hooks
		_, err = manager.Hooks(config, map[string]string{}, false)
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

func TestMonitorTwoDirGood(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	primaryDir, err := ioutil.TempDir("", "hooks-test-primary-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(primaryDir)

	fallbackDir, err := ioutil.TempDir("", "hooks-test-fallback-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fallbackDir)

	manager, err := New(ctx, []string{fallbackDir, primaryDir}, []string{})
	if err != nil {
		t.Fatal(err)
	}

	sync := make(chan error, 2)
	go manager.Monitor(ctx, sync)
	err = <-sync
	if err != nil {
		t.Fatal(err)
	}

	fallbackPath := filepath.Join(fallbackDir, "a.json")
	fallbackJSON := []byte(fmt.Sprintf("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"%s\"}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"]}", path))
	fallbackInjected := &rspec.Hooks{
		Prestart: []rspec.Hook{
			{
				Path: path,
			},
		},
	}

	t.Run("good-fallback-addition", func(t *testing.T) {
		err = ioutil.WriteFile(fallbackPath, fallbackJSON, 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, fallbackInjected, config.Hooks)
	})

	primaryPath := filepath.Join(primaryDir, "a.json")
	primaryJSON := []byte(fmt.Sprintf("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"%s\", \"timeout\": 1}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"]}", path))
	one := 1
	primaryInjected := &rspec.Hooks{
		Prestart: []rspec.Hook{
			{
				Path:    path,
				Timeout: &one,
			},
		},
	}

	t.Run("good-primary-override", func(t *testing.T) {
		err = ioutil.WriteFile(primaryPath, primaryJSON, 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, primaryInjected, config.Hooks)
	})

	t.Run("good-fallback-removal", func(t *testing.T) {
		err = os.Remove(fallbackPath)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, primaryInjected, config.Hooks) // masked by primary
	})

	t.Run("good-fallback-restore", func(t *testing.T) {
		err = ioutil.WriteFile(fallbackPath, fallbackJSON, 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, primaryInjected, config.Hooks) // masked by primary
	})

	primaryPath2 := filepath.Join(primaryDir, "0a.json") // 0a because it will be before a.json alphabetically

	t.Run("bad-primary-new-addition", func(t *testing.T) {
		err = ioutil.WriteFile(primaryPath2, []byte("{\"version\": \"-1\"}"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		fmt.Println("expected: ", config.Hooks)
		expected := primaryInjected // 0a.json is bad, a.json is still good
		_, err = manager.Hooks(config, map[string]string{}, false)
		fmt.Println("actual: ", config.Hooks)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expected, config.Hooks)
	})

	t.Run("bad-primary-same-addition", func(t *testing.T) {
		err = ioutil.WriteFile(primaryPath, []byte("{\"version\": \"-1\"}"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		expected := fallbackInjected
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expected, config.Hooks)
	})

	t.Run("good-primary-removal", func(t *testing.T) {
		err = os.Remove(primaryPath)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, fallbackInjected, config.Hooks)
	})

	t.Run("good-non-json-addition", func(t *testing.T) {
		err = ioutil.WriteFile(filepath.Join(fallbackDir, "README"), []byte("Hello"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, fallbackInjected, config.Hooks)
	})

	t.Run("good-fallback-removal", func(t *testing.T) {
		err = os.Remove(fallbackPath)
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond) // wait for monitor to notice

		config := &rspec.Spec{}
		expected := config.Hooks
		_, err = manager.Hooks(config, map[string]string{}, false)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expected, config.Hooks)
	})

	cancel()
	err = <-sync
	assert.Equal(t, context.Canceled, err)
}

func TestMonitorBadWatcher(t *testing.T) {
	ctx := context.Background()

	manager, err := New(ctx, []string{}, []string{})
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
