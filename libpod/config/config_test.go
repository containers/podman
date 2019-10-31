package config

import (
	"reflect"
	"testing"

	"github.com/containers/libpod/libpod/define"
	"github.com/containers/storage"
	"github.com/stretchr/testify/assert"
)

func TestEmptyConfig(t *testing.T) {
	// Make sure that we can read empty configs
	config, err := readConfigFromFile("testdata/empty.conf")
	assert.NotNil(t, config)
	assert.Nil(t, err)
}

func TestDefaultLibpodConf(t *testing.T) {
	// Make sure that we can read the default libpod.conf
	config, err := readConfigFromFile("testdata/libpod.conf")
	assert.NotNil(t, config)
	assert.Nil(t, err)
}

func TestMergeEmptyAndDefaultMemoryConfig(t *testing.T) {
	// Make sure that when we merge the default config into an empty one that we
	// effectively get the default config.
	defaultConfig, err := defaultConfigFromMemory()
	assert.NotNil(t, defaultConfig)
	assert.Nil(t, err)
	defaultConfig.StateType = define.InvalidStateStore
	defaultConfig.StorageConfig = storage.StoreOptions{}

	emptyConfig, err := readConfigFromFile("testdata/empty.conf")
	assert.NotNil(t, emptyConfig)
	assert.Nil(t, err)

	err = emptyConfig.mergeConfig(defaultConfig)
	assert.Nil(t, err)

	equal := reflect.DeepEqual(emptyConfig, defaultConfig)
	assert.True(t, equal)
}

func TestMergeEmptyAndLibpodConfig(t *testing.T) {
	// Make sure that when we merge the default config into an empty one that we
	// effectively get the default config.
	libpodConfig, err := readConfigFromFile("testdata/libpod.conf")
	assert.NotNil(t, libpodConfig)
	assert.Nil(t, err)
	libpodConfig.StateType = define.InvalidStateStore
	libpodConfig.StorageConfig = storage.StoreOptions{}

	emptyConfig, err := readConfigFromFile("testdata/empty.conf")
	assert.NotNil(t, emptyConfig)
	assert.Nil(t, err)

	err = emptyConfig.mergeConfig(libpodConfig)
	assert.Nil(t, err)

	equal := reflect.DeepEqual(emptyConfig, libpodConfig)
	assert.True(t, equal)
}
