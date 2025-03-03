package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		t.Run(test.name, func(t *testing.T) {
			kind, err := GetKubeKind([]byte(test.kubeYAML))
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
		{
			"DocWithList",
			`
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Pod
- apiVersion: v1
  kind: Pod
- apiVersion: v1
  kind: Pod
`,
			false,
			"",
			3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			docs, err := SplitMultiDocYAML([]byte(test.kubeYAML))
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
