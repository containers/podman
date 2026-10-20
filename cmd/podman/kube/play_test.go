package kube

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"go.podman.io/podman/v6/pkg/domain/entities"
)

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

func captureStd(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = outW
	os.Stderr = errW
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})
	fn()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	if err := outW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := errW.Close(); err != nil {
		t.Fatal(err)
	}

	stdout, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(errR)
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr)
}

func TestPrintPlayKubeDownSecretError(t *testing.T) {
	reports := &entities.PlayKubeReport{}
	reports.SecretRmReport = []*entities.SecretRmReport{{
		ID:  "sec1",
		Err: errors.New("secret remove failed"),
	}}

	_, stderr := captureStd(t, func() {
		if err := printPlayKubeDown(reports); err != nil {
			t.Fatalf("printPlayKubeDown: %v", err)
		}
	})
	if !strings.Contains(stderr, "secret remove failed") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestPrintPlayKubeDownPodErrorDoesNotPrintNilSecret(t *testing.T) {
	reports := &entities.PlayKubeReport{}
	reports.RmReport = []*entities.PodRmReport{{
		Id:  "pod1",
		Err: errors.New("pod remove failed"),
	}}
	reports.SecretRmReport = []*entities.SecretRmReport{{
		ID: "sec1",
	}}

	stdout, stderr := captureStd(t, func() {
		if err := printPlayKubeDown(reports); err != nil {
			t.Fatalf("printPlayKubeDown: %v", err)
		}
	})
	if !strings.Contains(stderr, "pod remove failed") {
		t.Fatalf("stderr = %q", stderr)
	}
	if strings.Contains(stderr, "%!s") || strings.Contains(stderr, "<nil>") {
		t.Fatalf("stderr = %q", stderr)
	}
	if !strings.Contains(stdout, "sec1") {
		t.Fatalf("stdout = %q", stdout)
	}
}
