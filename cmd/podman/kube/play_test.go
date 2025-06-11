package kube

import (
	"io"
	"os"
	"strings"
	"testing"
)

// createTempFile writes content to a temp file and returns its path.
func createTempFile(t *testing.T, content string) string {
	t.Helper()

	tmp, err := os.CreateTemp(t.TempDir(), "testfile-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}

	if err := tmp.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	return tmp.Name()
}

func TestReaderFromArgs(t *testing.T) {
	tests := []struct {
		name     string
		files    []string // file contents
		expected string   // expected concatenated output
	}{
		{
			name: "single file",
			files: []string{
				`apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config`,
			},
			expected: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config`,
		},
		{
			name: "two files",
			files: []string{
				`apiVersion: v1
kind: Pod
metadata:
  name: my-pod`,
				`apiVersion: v1
kind: Service
metadata:
  name: my-service`,
			},
			expected: `apiVersion: v1
kind: Pod
metadata:
  name: my-pod
---
apiVersion: v1
kind: Service
metadata:
  name: my-service`,
		},
		{
			name: "empty file and normal file",
			files: []string{
				``,
				`apiVersion: v1
kind: Secret
metadata:
  name: my-secret`,
			},
			expected: `---
apiVersion: v1
kind: Secret
metadata:
  name: my-secret`,
		},
		{
			name: "files with only whitespace",
			files: []string{
				"\n  \n",
				`apiVersion: v1
kind: Namespace
metadata:
  name: test-ns`,
			},
			expected: `

---
apiVersion: v1
kind: Namespace
metadata:
  name: test-ns`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paths []string
			for _, content := range tt.files {
				path := createTempFile(t, content)
				defer os.Remove(path)
				paths = append(paths, path)
			}

			reader, err := readerFromArgs(paths)
			if err != nil {
				t.Fatalf("readerFromArgs failed: %v", err)
			}

			output, err := io.ReadAll(reader)
			if err != nil {
				t.Fatalf("failed to read result: %v", err)
			}

			got := strings.TrimSpace(string(output))
			want := strings.TrimSpace(tt.expected)

			if got != want {
				t.Errorf("unexpected output:\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

func TestReaderFromArgs_Stdin(t *testing.T) {
	const input = `apiVersion: v1
kind: Namespace
metadata:
  name: from-stdin`

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	_, _ = w.WriteString(input)
	_ = w.Close()
	os.Stdin = r

	reader, err := readerFromArgs([]string{"-"})
	if err != nil {
		t.Fatalf("readerFromArgs failed: %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read from stdin: %v", err)
	}

	if got := string(data); got != input {
		t.Errorf("unexpected stdin result:\n--- got ---\n%s\n--- want ---\n%s", got, input)
	}
}
