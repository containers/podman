package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type (
	// VersionTree determines which API endpoint tree for version
	VersionTree int
	// VersionLevel determines which API level, current or something from the past
	VersionLevel int
)

const (
	// LibpodTree supports Libpod endpoints
	LibpodTree = VersionTree(iota)
	// CompatTree supports Libpod endpoints
	CompatTree

	// CurrentApiVersion announces what is the current API level
	CurrentApiVersion = VersionLevel(iota)
	// MinimalApiVersion announces what is the oldest API level supported
	MinimalApiVersion
)

var (
	// See https://docs.docker.com/engine/api/v1.40/
	// libpod compat handlers are expected to honor docker API versions

	// ApiVersion provides the current and minimal API versions for compat and libpod endpoint trees
	// Note: GET|HEAD /_ping is never versioned and provides the API-Version and Libpod-API-Version headers to allow
	//       clients to shop for the Version they wish to support
	ApiVersion = map[VersionTree]map[VersionLevel]semver.Version{
		LibpodTree: {
			CurrentApiVersion: semver.MustParse("1.0.0"),
			MinimalApiVersion: semver.MustParse("1.0.0"),
		},
		CompatTree: {
			CurrentApiVersion: semver.MustParse("1.40.0"),
			MinimalApiVersion: semver.MustParse("1.24.0"),
		},
	}

	// ErrVersionNotGiven returned when version not given by client
	ErrVersionNotGiven = errors.New("version not given in URL path")
	// ErrVersionNotSupported returned when given version is too old
	ErrVersionNotSupported = errors.New("given version is not supported")
)

// IsLibpodRequest returns true if the request related to a libpod endpoint
// (e.g., /v2/libpod/...).
func IsLibpodRequest(r *http.Request) bool {
	split := strings.Split(r.URL.String(), "/")
	return len(split) >= 3 && split[2] == "libpod"
}

// SupportedVersion validates that the version provided by client is included in the given condition
// https://github.com/blang/semver#ranges provides the details for writing conditions
// If a version is not given in URL path, ErrVersionNotGiven is returned
func SupportedVersion(r *http.Request, condition string) (semver.Version, error) {
	version := semver.Version{}
	val, ok := mux.Vars(r)["version"]
	if !ok {
		return version, ErrVersionNotGiven
	}
	safeVal, err := url.PathUnescape(val)
	if err != nil {
		return version, errors.Wrapf(err, "unable to unescape given API version: %q", val)
	}
	version, err = semver.ParseTolerant(safeVal)
	if err != nil {
		return version, errors.Wrapf(err, "unable to parse given API version: %q from %q", safeVal, val)
	}

	inRange, err := semver.ParseRange(condition)
	if err != nil {
		return version, err
	}

	if inRange(version) {
		return version, nil
	}
	return version, ErrVersionNotSupported
}

// SupportedVersionWithDefaults validates that the version provided by client valid is supported by server
// minimal API version <= client path version <= maximum API version focused on the endpoint tree from URL
func SupportedVersionWithDefaults(r *http.Request) (semver.Version, error) {
	tree := CompatTree
	if IsLibpodRequest(r) {
		tree = LibpodTree
	}

	return SupportedVersion(r,
		fmt.Sprintf(">=%s <=%s", ApiVersion[tree][MinimalApiVersion].String(),
			ApiVersion[tree][CurrentApiVersion].String()))
}

// WriteResponse encodes the given value as JSON or string and renders it for http client
func WriteResponse(w http.ResponseWriter, code int, value interface{}) {
	// RFC2616 explicitly states that the following status codes "MUST NOT
	// include a message-body":
	switch code {
	case http.StatusNoContent, http.StatusNotModified: // 204, 304
		w.WriteHeader(code)
		return
	}

	switch v := value.(type) {
	case string:
		w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := fmt.Fprintln(w, v); err != nil {
			logrus.Errorf("unable to send string response: %q", err)
		}
	case *os.File:
		w.Header().Set("Content-Type", "application/octet; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := io.Copy(w, v); err != nil {
			logrus.Errorf("unable to copy to response: %q", err)
		}
	case io.Reader:
		w.Header().Set("Content-Type", "application/x-tar")
		w.WriteHeader(code)

		if _, err := io.Copy(w, v); err != nil {
			logrus.Errorf("unable to copy to response: %q", err)
		}
	default:
		WriteJSON(w, code, value)
	}
}

func WriteJSON(w http.ResponseWriter, code int, value interface{}) {
	// FIXME: we don't need to write the header in all/some circumstances.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)
	if err := coder.Encode(value); err != nil {
		logrus.Errorf("unable to write json: %q", err)
	}
}

func FilterMapToString(filters map[string][]string) (string, error) {
	f, err := json.Marshal(filters)
	if err != nil {
		return "", err
	}
	return string(f), nil
}

func getVar(r *http.Request, k string) string {
	val := mux.Vars(r)[k]
	safeVal, err := url.PathUnescape(val)
	if err != nil {
		logrus.Error(errors.Wrapf(err, "failed to unescape mux key %s, value %s", k, val))
		return val
	}
	return safeVal
}

// GetName extracts the name from the mux
func GetName(r *http.Request) string {
	return getVar(r, "name")
}
