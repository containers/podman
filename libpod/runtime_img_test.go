package libpod

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	registry = `[registries.search]
registries = ['one']

[registries.insecure]
registries = ['two']`
)

func createTmpFile(content []byte) (string, error) {
	tmpfile, err := ioutil.TempFile(os.TempDir(), "unittest")
	if err != nil {
		return "", err
	}

	if _, err := tmpfile.Write(content); err != nil {
		return "", err

	}
	if err := tmpfile.Close(); err != nil {
		return "", err
	}
	return tmpfile.Name(), nil
}

func TestGetRegistries(t *testing.T) {
	registryPath, err := createTmpFile([]byte(registry))
	assert.NoError(t, err)
	defer os.Remove(registryPath)
	os.Setenv("REGISTRIES_CONFIG_PATH", registryPath)
	registries, err := GetRegistries()
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(registries, []string{"one"}))
}

func TestGetInsecureRegistries(t *testing.T) {
	registryPath, err := createTmpFile([]byte(registry))
	assert.NoError(t, err)
	os.Setenv("REGISTRIES_CONFIG_PATH", registryPath)
	defer os.Remove(registryPath)
	registries, err := GetInsecureRegistries()
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(registries, []string{"two"}))
}
