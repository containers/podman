package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetImageConfigStopSignal(t *testing.T) {
	// Linux-only because parsing signal names is not supported on non-Linux systems by
	// pkg/signal.
	stopSignalValidInt, err := GetImageConfig([]string{"STOPSIGNAL 9"})
	require.Nil(t, err)
	assert.Equal(t, stopSignalValidInt.StopSignal, "9")

	stopSignalValidString, err := GetImageConfig([]string{"STOPSIGNAL SIGKILL"})
	require.Nil(t, err)
	assert.Equal(t, stopSignalValidString.StopSignal, "9")

	_, err = GetImageConfig([]string{"STOPSIGNAL 0"})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"STOPSIGNAL garbage"})
	assert.NotNil(t, err)

	_, err = GetImageConfig([]string{"STOPSIGNAL "})
	assert.NotNil(t, err)
}
