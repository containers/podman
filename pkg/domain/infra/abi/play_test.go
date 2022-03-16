package abi

import (
	"bytes"
	"testing"

	v1 "github.com/containers/podman/v4/pkg/k8s.io/api/core/v1"
	v12 "github.com/containers/podman/v4/pkg/k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
)

func TestReadConfigMapFromFile(t *testing.T) {
	tests := []struct {
		name             string
		configMapContent string
		expectError      bool
		expectedErrorMsg string
		expected         v1.ConfigMap
	}{
		{
			"ValidConfigMap",
			`
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myvar: foo
`,
			false,
			"",
			v1.ConfigMap{
				TypeMeta: v12.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: v12.ObjectMeta{
					Name: "foo",
				},
				Data: map[string]string{
					"myvar": "foo",
				},
			},
		},
		{
			"InvalidYAML",
			`
Invalid YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myvar: foo
`,
			true,
			"unable to read YAML as Kube ConfigMap",
			v1.ConfigMap{},
		},
		{
			"InvalidKind",
			`
apiVersion: v1
kind: InvalidKind
metadata:
  name: foo
data:
  myvar: foo
`,
			true,
			"invalid YAML kind",
			v1.ConfigMap{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.NewBufferString(test.configMapContent)
			cm, err := readConfigMapFromFile(buf)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, cm)
			}
		})
	}
}

func TestGetKubeKind(t *testing.T) {
	tests := []struct {
		name             string
		kubeYAML         string
		expectError      bool
		expectedErrorMsg string
		expected         string
	}{
		{
			"ValidKubeYAML",
			`
apiVersion: v1
kind: Pod
`,
			false,
			"",
			"Pod",
		},
		{
			"InvalidKubeYAML",
			"InvalidKubeYAML",
			true,
			"cannot unmarshal",
			"",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			kind, err := getKubeKind([]byte(test.kubeYAML))
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, kind)
			}
		})
	}
}

func TestSplitMultiDocYAML(t *testing.T) {
	tests := []struct {
		name             string
		kubeYAML         string
		expectError      bool
		expectedErrorMsg string
		expected         int
	}{
		{
			"ValidNumberOfDocs",
			`
apiVersion: v1
kind: Pod
---
apiVersion: v1
kind: Pod
---
apiVersion: v1
kind: Pod
`,
			false,
			"",
			3,
		},
		{
			"InvalidMultiDocYAML",
			`
apiVersion: v1
kind: Pod
---
apiVersion: v1
kind: Pod
-
`,
			true,
			"multi doc yaml could not be split",
			0,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			docs, err := splitMultiDocYAML([]byte(test.kubeYAML))
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, len(docs))
			}
		})
	}
}
