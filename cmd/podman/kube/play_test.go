package kube

import (
	"context"
	"io"
	"testing"

	v1 "github.com/containers/podman/v5/pkg/k8s.io/api/core/v1"
	"github.com/stretchr/testify/assert"
)

func TestPodBuild(t *testing.T) {
	tests := []struct {
		name             string
		pod              v1.Pod
		contextDir       string
		expectError      bool
		expectedErrorMsg string
		expectedImages   []string
	}{
		{
			"pod without containers should no raise any error",
			v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{},
				},
			},
			"",
			false,
			"",
			[]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()

			pod, err := podBuild(nil, ctx, test.pod, test.contextDir)
			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				for i, container := range pod.Spec.Containers {
					assert.Equal(t, test.expectedImages[i], container.Image)
				}
			}
		})
	}
}

// TestBytesArrayToReader tests the bytesArrayToReader function
func TestBytesArrayToReader(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]byte
		expected string
	}{
		{
			name: "single document",
			input: [][]byte{
				[]byte("document1"),
			},
			expected: "document1",
		},
		{
			name: "two documents",
			input: [][]byte{
				[]byte("document1"),
				[]byte("document2"),
			},
			expected: "document1\n---\ndocument2",
		},
		{
			name: "three documents",
			input: [][]byte{
				[]byte("document1"),
				[]byte("document2"),
				[]byte("document3"),
			},
			expected: "document1\n---\ndocument2\n---\ndocument3",
		},
		{
			name:     "empty input",
			input:    [][]byte{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytesArrayToReader(tt.input)
			result, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assert.Equal(t, tt.expected, string(result))
		})
	}
}
