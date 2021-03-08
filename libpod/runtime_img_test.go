package libpod

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	sysreg "github.com/containers/podman/v3/pkg/registries"
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
	os.Setenv("CONTAINERS_REGISTRIES_CONF", registryPath)
	registries, err := sysreg.GetRegistries()
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(registries, []string{"one"}))
}

func TestGetInsecureRegistries(t *testing.T) {
	registryPath, err := createTmpFile([]byte(registry))
	assert.NoError(t, err)
	os.Setenv("CONTAINERS_REGISTRIES_CONF", registryPath)
	defer os.Remove(registryPath)
	registries, err := sysreg.GetInsecureRegistries()
	assert.NoError(t, err)
	assert.True(t, reflect.DeepEqual(registries, []string{"two"}))
}
