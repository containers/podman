package pods

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/containers/podman/v4/pkg/bindings"
)

func TestRemove(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v4.1.0/libpod/_ping":
			// NewConnection() always pings service
		case "/v4.1.0/libpod/pods/pod001":
			if r.Method != http.MethodDelete {
				t.Errorf("Expected to DELETE request, got %q", r.Method)
			}
		default:
			t.Errorf("Unexpected request %q", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"Id":"pod001","Err":""}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	conn, err := bindings.NewConnection(context.Background(), server.URL)
	if err != nil {
		t.Error(err)
	}

	report, err := Remove(conn, "pod001", nil)
	if err != nil {
		t.Error(err)
	}

	if report.Err != nil {
		t.Errorf(`Expected Err field to be nil, got "%v"`, report.Err)
	}
}

func TestRemoveWithError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v4.1.0/libpod/_ping":
			// NewConnection() always pings service
		case "/v4.1.0/libpod/pods/pod001":
			if r.Method != http.MethodDelete {
				t.Errorf("Expected to DELETE request, got %q", r.Method)
			}
		default:
			t.Errorf("Unexpected request %q", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"Id":"pod001","Err":"forced error message"}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()

	conn, err := bindings.NewConnection(context.Background(), server.URL)
	if err != nil {
		t.Error(err)
	}

	report, err := Remove(conn, "pod001", nil)
	if err != nil {
		t.Error(err)
	}

	if report.Err == nil {
		t.Error("Expected Err field to be populated, got nil")
	}

	if report.Err.Error() != "forced error message" {
		t.Errorf("Expected error 'forced error message', got %q", report.Err.Error())
	}
}
