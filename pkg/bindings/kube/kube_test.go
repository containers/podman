package kube

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/containers/storage/pkg/archive"
	"github.com/stretchr/testify/assert"
)

// Helper function to untar the resulting tarball
func untar(tarStream io.Reader, destDir string) error {
	return archive.Untar(tarStream, destDir, nil)
}

func TestGetTarKubePlayContext(t *testing.T) {
	testCases := []struct {
		description   string
		yamlContent   string
		setup         func(t *testing.T) string
		expectedFiles map[string]string
		expectError   bool
	}{
		{
			description: "Basic pod definition with a single container",
			yamlContent: `
apiVersion: v1
kind: Pod
metadata:
  name: demo-build-remote
spec:
  containers:
    - name: container
      image: foobar
`,
			setup: func(t *testing.T) string {
				// Create a temporary context directory with a Dockerfile for the "foobar" image
				tmpDir := t.TempDir()
				fooBarDir := filepath.Join(tmpDir, "foobar")
				err := os.Mkdir(fooBarDir, 0755)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(fooBarDir, "Containerfile"), []byte("FROM busybox"), 0644)
				assert.NoError(t, err)
				return tmpDir
			},
			expectedFiles: map[string]string{
				"play.yaml": `
apiVersion: v1
kind: Pod
metadata:
  name: demo-build-remote
spec:
  containers:
    - name: container
      image: foobar
`, "foobar/Containerfile": "FROM busybox",
			},
		},
		{
			description: "Pod with multiple containers",
			yamlContent: `
apiVersion: v1
kind: Pod
metadata:
  name: demo-build-remote
spec:
  containers:
    - name: container1
      image: foobar
    - name: container2
      image: barfoo
`,
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				fooBarDir := filepath.Join(tmpDir, "foobar")
				err := os.Mkdir(fooBarDir, 0755)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(fooBarDir, "Containerfile"), []byte("FROM busybox:1"), 0644)
				assert.NoError(t, err)

				barfooDir := filepath.Join(tmpDir, "barfoo")
				err = os.Mkdir(barfooDir, 0755)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(barfooDir, "Containerfile"), []byte("FROM busybox:2"), 0644)
				assert.NoError(t, err)
				return tmpDir
			},
			expectedFiles: map[string]string{
				"play.yaml": `
apiVersion: v1
kind: Pod
metadata:
  name: demo-build-remote
spec:
  containers:
    - name: container1
      image: foobar
    - name: container2
      image: barfoo
`,
				"foobar/Containerfile": "FROM busybox:1",
				"barfoo/Containerfile": "FROM busybox:2",
			},
		},
		{
			description: "Non-pod resources are ignored",
			yamlContent: `
apiVersion: v1
kind: Service
metadata:
  name: demo-service
spec:
  ports:
    - port: 80
`,
			setup: func(t *testing.T) string {
				return t.TempDir() // No Dockerfile needed since no Pod exists
			},
			expectedFiles: map[string]string{
				"play.yaml": `
apiVersion: v1
kind: Service
metadata:
  name: demo-service
spec:
  ports:
    - port: 80
`,
			},
		},
		{
			description: "Missing build file for container",
			yamlContent: `
apiVersion: v1
kind: Pod
metadata:
  name: demo-build-remote
spec:
  containers:
    - name: container
      image: missing-image
`,
			setup: func(t *testing.T) string {
				return t.TempDir() // No Dockerfile for the missing image
			},
			expectedFiles: map[string]string{
				"play.yaml": `
apiVersion: v1
kind: Pod
metadata:
  name: demo-build-remote
spec:
  containers:
    - name: container
      image: missing-image
`,
			},
		},
		{
			description: "Multiple YAML documents",
			yamlContent: `
apiVersion: v1
kind: Pod
metadata:
  name: pod-1
spec:
  containers:
    - name: container1
      image: foobar
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-2
spec:
  containers:
    - name: container1
      image: barfoo
`,
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				fooBarDir := filepath.Join(tmpDir, "foobar")
				err := os.Mkdir(fooBarDir, 0755)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(fooBarDir, "Containerfile"), []byte("FROM busybox:1"), 0644)
				assert.NoError(t, err)

				barfooDir := filepath.Join(tmpDir, "barfoo")
				err = os.Mkdir(barfooDir, 0755)
				assert.NoError(t, err)
				err = os.WriteFile(filepath.Join(barfooDir, "Containerfile"), []byte("FROM busybox:2"), 0644)
				assert.NoError(t, err)
				return tmpDir
			},
			expectedFiles: map[string]string{
				"play.yaml": `
apiVersion: v1
kind: Pod
metadata:
  name: pod-1
spec:
  containers:
    - name: container1
      image: foobar
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-2
spec:
  containers:
    - name: container1
      image: barfoo
`,
				"foobar/Containerfile": "FROM busybox:1",
				"barfoo/Containerfile": "FROM busybox:2",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.description, func(t *testing.T) {
			// Create the context directory based on the setup
			contextDir := testCase.setup(t)

			// Convert the YAML content to a reader
			reader := bytes.NewReader([]byte(testCase.yamlContent))

			// Call getTarKubePlayContext
			tarStream, err := getTarKubePlayContext(reader, contextDir)
			if testCase.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			defer tarStream.Close()

			// Create a temporary directory to untar the result
			destDir := t.TempDir()

			// Untar the resulting tarball
			err = untar(tarStream, destDir)
			assert.NoError(t, err)

			// Validate the contents of the tarball
			for expectedPath, expectedContent := range testCase.expectedFiles {
				actualContent, err := os.ReadFile(filepath.Join(destDir, expectedPath))
				assert.NoError(t, err)
				assert.Equal(t, expectedContent, string(actualContent))
			}
		})
	}
}
