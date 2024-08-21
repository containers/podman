//go:build !remote

package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"unsafe"

	"github.com/blang/semver/v4"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"

	"github.com/containers/podman/v5/pkg/api/handlers/utils/apiutil"
	api "github.com/containers/podman/v5/pkg/api/types"
)

// IsLibpodRequest returns true if the request related to a libpod endpoint
// (e.g., /v2/libpod/...).
func IsLibpodRequest(r *http.Request) bool {
	return apiutil.IsLibpodRequest(r)
}

// SupportedVersion validates that the version provided by client is included in the given condition
// https://github.com/blang/semver#ranges provides the details for writing conditions
// If a version is not given in URL path, ErrVersionNotGiven is returned
func SupportedVersion(r *http.Request, condition string) (semver.Version, error) {
	return apiutil.SupportedVersion(r, condition)
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
			logrus.Errorf("Unable to send string response: %q", err)
		}
	case *os.File:
		w.Header().Set("Content-Type", "application/octet; charset=us-ascii")
		w.WriteHeader(code)

		if _, err := io.Copy(w, v); err != nil {
			logrus.Errorf("Unable to copy to response: %q", err)
		}
	case io.Reader:
		w.Header().Set("Content-Type", "application/x-tar")
		w.WriteHeader(code)

		if _, err := io.Copy(w, v); err != nil {
			logrus.Errorf("Unable to copy to response: %q", err)
		}
	default:
		WriteJSON(w, code, value)
	}
}

func init() {
	jsoniter.RegisterTypeEncoderFunc("error", MarshalErrorJSON, MarshalErrorJSONIsEmpty)
	jsoniter.RegisterTypeEncoderFunc("[]error", MarshalErrorSliceJSON, MarshalErrorSliceJSONIsEmpty)
}

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// MarshalErrorJSON writes error to stream as string
func MarshalErrorJSON(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	p := *((*error)(ptr))
	if p == nil {
		stream.WriteNil()
	} else {
		stream.WriteString(p.Error())
	}
}

// MarshalErrorSliceJSON writes []error to stream as []string JSON blob
func MarshalErrorSliceJSON(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	a := *((*[]error)(ptr))
	switch {
	case len(a) == 0:
		stream.WriteNil()
	default:
		stream.WriteArrayStart()
		for i, e := range a {
			if i > 0 {
				stream.WriteMore()
			}
			stream.WriteString(e.Error())
		}
		stream.WriteArrayEnd()
	}
}

func MarshalErrorJSONIsEmpty(ptr unsafe.Pointer) bool {
	return *((*error)(ptr)) == nil
}

func MarshalErrorSliceJSONIsEmpty(ptr unsafe.Pointer) bool {
	return len(*((*[]error)(ptr))) == 0
}

// WriteJSON writes an interface value encoded as JSON to w
func WriteJSON(w http.ResponseWriter, code int, value interface{}) {
	// FIXME: we don't need to write the header in all/some circumstances.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)
	if err := coder.Encode(value); err != nil {
		logrus.Errorf("Unable to write json: %q", err)
	}
}

func FilterMapToString(filters map[string][]string) (string, error) {
	f, err := json.Marshal(filters)
	if err != nil {
		return "", err
	}
	return string(f), nil
}

func GetVar(r *http.Request, k string) string {
	val := mux.Vars(r)[k]
	safeVal, err := url.PathUnescape(val)
	if err != nil {
		logrus.Error(fmt.Errorf("failed to unescape mux key %s, value %s: %w", k, val, err))
		return val
	}
	return safeVal
}

// GetName extracts the name from the mux
func GetName(r *http.Request) string {
	return GetVar(r, "name")
}

func GetDecoder(r *http.Request) *schema.Decoder {
	if IsLibpodRequest(r) {
		return r.Context().Value(api.DecoderKey).(*schema.Decoder)
	}
	return r.Context().Value(api.CompatDecoderKey).(*schema.Decoder)
}
