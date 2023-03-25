package abi

import (
	"testing"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestGetSdNotifyMode(t *testing.T) {
	tests := []struct {
		key, value, name, result string
		mustError                bool
	}{
		{sdNotifyAnnotation, define.SdNotifyModeConmon, "", define.SdNotifyModeConmon, false},
		{sdNotifyAnnotation + "/container-a", define.SdNotifyModeContainer, "container-a", define.SdNotifyModeContainer, false},
		{sdNotifyAnnotation + "/container-b", define.SdNotifyModeIgnore, "container-b", define.SdNotifyModeIgnore, false},
		{sdNotifyAnnotation + "/container-c", "", "container-c", "", false},
		{sdNotifyAnnotation + "-/wrong-key", "xxx", "wrong-key", "", false},
		{sdNotifyAnnotation + "/container-error", "invalid", "container-error", "", true},
	}

	annotations := make(map[string]string)
	// Populate the annotations
	for _, test := range tests {
		annotations[test.key] = test.value
	}
	// Run the tests
	for _, test := range tests {
		result, err := getSdNotifyMode(annotations, test.name)
		if test.mustError {
			require.Error(t, err, "%v", test)
			continue
		}
		require.NoError(t, err, "%v", test)
		require.Equal(t, test.result, result, "%v", test)
	}
}

func TestGetBuildArgs(t *testing.T) {
	prefix := util.BuildArgumentsAnnotationPrefix
	name := "test-pod"
	annotations := map[string]string{
		// Good cases
		prefix + ".ARG/" + name:         "Test1",
		prefix + ".another-arg/" + name: "Test2",
		prefix + ".empty-value/" + name: "",
		prefix + ".ARG2":                "Test4",

		// Bad cases
		"some-prefix-" + prefix + ".another-arg": "BadTest1",
		"pfx." + prefix + ".another-arg":         "BadTest2",
		prefix + "/":                             "BadTest3",
		prefix:                                   "BadTest4",
		"incorrect-prefix":                       "BadTest5",
		"/":                                      "BadTest6",
		prefix + ".ARG/another-pod":              "BadTest7",
		prefix + ".ARG/test-pod2":                "BadTest8",
		prefix + ".ARG/pfx-test-pod":             "BadTest9",
	}

	expected := map[string]string{
		"ARG":         "Test1",
		"another-arg": "Test2",
		"empty-value": "",
		"ARG2":        "Test4",
	}

	args := getBuildArgs(annotations, name)
	require.Equal(t, args, expected)
}
