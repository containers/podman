//go:build !remote && linux

package compat

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
)

func TestProcessBuildContext_DockerfileConfinement(t *testing.T) {
	t.Parallel()

	contextDir := t.TempDir()
	anchorDir := t.TempDir()
	req := httptest.NewRequest(http.MethodPost, "/v1.41/build", nil)

	t.Run("relative dockerfile stays in context", func(t *testing.T) {
		t.Parallel()

		query := url.Values{}
		query.Set("dockerfile", "Dockerfile")

		buildContext, err := processBuildContext(query, req, &BuildContext{ContextDirectory: contextDir}, anchorDir)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(buildContext.ContainerFiles) != 1 {
			t.Fatalf("expected 1 dockerfile, got %d", len(buildContext.ContainerFiles))
		}
		want := filepath.Join(contextDir, "Dockerfile")
		if buildContext.ContainerFiles[0] != want {
			t.Fatalf("expected %q, got %q", want, buildContext.ContainerFiles[0])
		}
	})

	t.Run("absolute dockerfile within context is allowed", func(t *testing.T) {
		t.Parallel()

		query := url.Values{}
		abs := filepath.Join(contextDir, "Dockerfile")
		query.Set("dockerfile", abs)

		buildContext, err := processBuildContext(query, req, &BuildContext{ContextDirectory: contextDir}, anchorDir)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(buildContext.ContainerFiles) != 1 {
			t.Fatalf("expected 1 dockerfile, got %d", len(buildContext.ContainerFiles))
		}
		if buildContext.ContainerFiles[0] != abs {
			t.Fatalf("expected %q, got %q", abs, buildContext.ContainerFiles[0])
		}
	})

	t.Run("absolute host path outside context is rejected", func(t *testing.T) {
		t.Parallel()

		query := url.Values{}
		query.Set("dockerfile", "/etc/passwd")

		_, err := processBuildContext(query, req, &BuildContext{ContextDirectory: contextDir}, anchorDir)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("traversal escapes are rejected", func(t *testing.T) {
		t.Parallel()

		query := url.Values{}
		query.Set("dockerfile", "../outside")

		_, err := processBuildContext(query, req, &BuildContext{ContextDirectory: contextDir}, anchorDir)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("libpod local build allows absolute dockerfile outside context", func(t *testing.T) {
		t.Parallel()

		query := url.Values{}
		query.Set("dockerfile", "/etc/passwd")

		libpodReq := httptest.NewRequest(http.MethodPost, "/v1.41/libpod/local/build", nil)
		buildContext, err := processBuildContext(query, libpodReq, &BuildContext{ContextDirectory: contextDir}, anchorDir)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(buildContext.ContainerFiles) != 1 {
			t.Fatalf("expected 1 dockerfile, got %d", len(buildContext.ContainerFiles))
		}
		if buildContext.ContainerFiles[0] != "/etc/passwd" {
			t.Fatalf("expected %q, got %q", "/etc/passwd", buildContext.ContainerFiles[0])
		}
	})

	t.Run("libpod build dockerfile is confined to context", func(t *testing.T) {
		t.Parallel()

		query := url.Values{}
		query.Set("dockerfile", "/etc/passwd")

		libpodReq := httptest.NewRequest(http.MethodPost, "/v1.41/libpod/build", nil)
		_, err := processBuildContext(query, libpodReq, &BuildContext{ContextDirectory: contextDir}, anchorDir)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
