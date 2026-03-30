package kube

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containers/podman/v6/pkg/specgen"
)

// expectedPath mirrors the exact decision tree of resolveHostPathsInNode:
//   - if path is already absolute on this OS, return it unchanged;
//   - otherwise join it with base and run ConvertWinMountPath so the result
//     matches the actual output on every platform (e.g. C:\foo→/mnt/c/foo on
//     Windows/WSL, no-op on Linux/macOS).
func expectedPath(base, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	abs := filepath.Join(base, path)
	if converted, err := specgen.ConvertWinMountPath(abs); err == nil {
		return converted
	}
	return abs
}

var configMapYAML = strings.Join([]string{
	"apiVersion: v1",
	"kind: ConfigMap",
	"metadata:",
	"  name: my-config",
	"data:",
	"  key: value",
}, "\n")

var podYAML = strings.Join([]string{
	"apiVersion: v1",
	"kind: Pod",
	"metadata:",
	"  name: my-pod",
}, "\n")

var serviceYAML = strings.Join([]string{
	"apiVersion: v1",
	"kind: Service",
	"metadata:",
	"  name: my-service",
}, "\n")

var secretYAML = strings.Join([]string{
	"apiVersion: v1",
	"kind: Secret",
	"metadata:",
	"  name: my-secret",
}, "\n")

var namespaceYAML = strings.Join([]string{
	"apiVersion: v1",
	"kind: Namespace",
	"metadata:",
	"  name: my-namespace",
}, "\n")

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
			name:     "single file",
			files:    []string{configMapYAML},
			expected: configMapYAML,
		},
		{
			name: "two files",
			files: []string{
				podYAML,
				serviceYAML,
			},
			expected: podYAML + "\n---\n" + serviceYAML,
		},
		{
			name: "empty file and normal file",
			files: []string{
				"",
				secretYAML,
			},
			expected: "---\n" + secretYAML,
		},
		{
			name: "files with only whitespace",
			files: []string{
				"\n  \n",
				namespaceYAML,
			},
			expected: "---\n" + namespaceYAML,
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

			reader, err := readerFromArgsWithStdin(paths, nil)
			if err != nil {
				t.Fatalf("readerFromArgsWithStdin failed: %v", err)
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
	stdinReader := strings.NewReader(namespaceYAML)

	reader, err := readerFromArgsWithStdin([]string{"-"}, stdinReader)
	if err != nil {
		t.Fatalf("readerFromArgsWithStdin failed: %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read from stdin: %v", err)
	}

	if got := string(data); got != namespaceYAML {
		t.Errorf("unexpected stdin result:\n--- got ---\n%s\n--- want ---\n%s", got, namespaceYAML)
	}
}

func TestResolveRelativeHostPaths(t *testing.T) {
	baseDir := "/some/base/dir"

	tests := []struct {
		name     string
		input    string
		wantPath string // expected value of hostPath.path after resolution
	}{
		{
			name: "relative dot-slash path is resolved",
			input: strings.Join([]string{
				"apiVersion: v1",
				"kind: Pod",
				"spec:",
				"  volumes:",
				"  - name: html",
				"    hostPath:",
				"      path: ./html",
				"      type: Directory",
			}, "\n"),
			wantPath: expectedPath(baseDir, "html"),
		},
		{
			name: "relative bare path is resolved",
			input: strings.Join([]string{
				"apiVersion: v1",
				"kind: Pod",
				"spec:",
				"  volumes:",
				"  - name: data",
				"    hostPath:",
				"      path: data/subdir",
				"      type: Directory",
			}, "\n"),
			wantPath: expectedPath(baseDir, "data/subdir"),
		},
		{
			name: "absolute path is passed through",
			input: strings.Join([]string{
				"apiVersion: v1",
				"kind: Pod",
				"spec:",
				"  volumes:",
				"  - name: html",
				"    hostPath:",
				"      path: /absolute/path",
				"      type: Directory",
			}, "\n"),
			// On Linux/macOS filepath.IsAbs("/absolute/path") is true so the
			// path is not modified. On Windows it is false (no drive letter),
			// so ConvertWinMountPath converts it. expectedPath mirrors the
			// exact IsAbs check the production code uses.
			wantPath: expectedPath(baseDir, "/absolute/path"),
		},
		{
			name: "no hostPath volumes — content passes through",
			input: strings.Join([]string{
				"apiVersion: v1",
				"kind: Pod",
				"metadata:",
				"  name: my-pod",
			}, "\n"),
			wantPath: "", // no hostPath to check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := resolveRelativeHostPaths([]byte(tt.input), baseDir)
			if err != nil {
				t.Fatalf("resolveRelativeHostPaths returned error: %v", err)
			}
			if tt.wantPath == "" {
				return
			}
			outStr := string(out)
			if !strings.Contains(outStr, tt.wantPath) {
				t.Errorf("expected output to contain path %q\ngot:\n%s", tt.wantPath, outStr)
			}
		})
	}
}

func TestResolveRelativeHostPaths_MultiDoc(t *testing.T) {
	baseDir := "/base"
	input := strings.Join([]string{
		"apiVersion: v1",
		"kind: Pod",
		"spec:",
		"  volumes:",
		"  - name: vol1",
		"    hostPath:",
		"      path: ./vol1",
		"---",
		"apiVersion: v1",
		"kind: Pod",
		"spec:",
		"  volumes:",
		"  - name: vol2",
		"    hostPath:",
		"      path: ./vol2",
	}, "\n")

	out, err := resolveRelativeHostPaths([]byte(input), baseDir)
	if err != nil {
		t.Fatalf("resolveRelativeHostPaths returned error: %v", err)
	}
	outStr := string(out)
	for _, want := range []string{expectedPath(baseDir, "vol1"), expectedPath(baseDir, "vol2")} {
		if !strings.Contains(outStr, want) {
			t.Errorf("expected output to contain %q\ngot:\n%s", want, outStr)
		}
	}
}
