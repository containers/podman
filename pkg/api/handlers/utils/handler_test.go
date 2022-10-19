package utils

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/containers/podman/v4/version"
	"github.com/gorilla/mux"
)

func TestSupportedVersion(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("/v%s/libpod/testing/versions", version.APIVersion[version.Libpod][version.CurrentAPI]),
		nil)
	if err != nil {
		t.Fatal(err)
	}
	req = mux.SetURLVars(req, map[string]string{"version": version.APIVersion[version.Libpod][version.CurrentAPI].String()})

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := SupportedVersionWithDefaults(r)
		switch {
		case errors.Is(err, ErrVersionNotGiven): // for compat endpoints version optional
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
		case errors.Is(err, ErrVersionNotSupported): // version given but not supported
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
		default: // all good
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		}
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := `OK`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %q want %q",
			rr.Body.String(), expected)
	}
}

func TestUnsupportedVersion(t *testing.T) {
	version := "999.999.999"
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("/v%s/libpod/testing/versions", version),
		nil)
	if err != nil {
		t.Fatal(err)
	}
	req = mux.SetURLVars(req, map[string]string{"version": version})

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := SupportedVersionWithDefaults(r)
		switch {
		case errors.Is(err, ErrVersionNotGiven): // for compat endpoints version optional
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
		case errors.Is(err, ErrVersionNotSupported): // version given but not supported
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
		default: // all good
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		}
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the response body is what we expect.
	expected := ErrVersionNotSupported.Error()
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %q want %q",
			rr.Body.String(), expected)
	}
}

func TestEqualVersion(t *testing.T) {
	version := "1.30.0"
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("/v%s/libpod/testing/versions", version),
		nil)
	if err != nil {
		t.Fatal(err)
	}
	req = mux.SetURLVars(req, map[string]string{"version": version})

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := SupportedVersion(r, "=="+version)
		switch {
		case errors.Is(err, ErrVersionNotGiven): // for compat endpoints version optional
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
		case errors.Is(err, ErrVersionNotSupported): // version given but not supported
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
		case err != nil:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
		default: // all good
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
		}
	})
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	expected := http.StatusText(http.StatusOK)
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %q want %q",
			rr.Body.String(), expected)
	}
}

func TestErrorEncoderFuncOmit(t *testing.T) {
	data, err := json.Marshal(struct {
		Err  error   `json:"err,omitempty"`
		Errs []error `json:"errs,omitempty"`
	}{})
	if err != nil {
		t.Fatal(err)
	}

	dataAsMap := make(map[string]interface{})
	err = json.Unmarshal(data, &dataAsMap)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := dataAsMap["err"]
	if ok {
		t.Errorf("the `err` field should have been omitted")
	}
	_, ok = dataAsMap["errs"]
	if ok {
		t.Errorf("the `errs` field should have been omitted")
	}

	dataAsMap = make(map[string]interface{})
	data, err = json.Marshal(struct {
		Err  error   `json:"err"`
		Errs []error `json:"errs"`
	}{})
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(data, &dataAsMap)
	if err != nil {
		t.Fatal(err)
	}

	_, ok = dataAsMap["err"]
	if !ok {
		t.Errorf("the `err` field shouldn't have been omitted")
	}
	_, ok = dataAsMap["errs"]
	if !ok {
		t.Errorf("the `errs` field shouldn't have been omitted")
	}
}
