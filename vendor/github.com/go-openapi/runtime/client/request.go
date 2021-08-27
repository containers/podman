// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"

	"github.com/go-openapi/runtime"
)

// NewRequest creates a new swagger http client request
func newRequest(method, pathPattern string, writer runtime.ClientRequestWriter) (*request, error) {
	return &request{
		pathPattern: pathPattern,
		method:      method,
		writer:      writer,
		header:      make(http.Header),
		query:       make(url.Values),
		timeout:     DefaultTimeout,
		getBody:     getRequestBuffer,
	}, nil
}

// Request represents a swagger client request.
//
// This Request struct converts to a HTTP request.
// There might be others that convert to other transports.
// There is no error checking here, it is assumed to be used after a spec has been validated.
// so impossible combinations should not arise (hopefully).
//
// The main purpose of this struct is to hide the machinery of adding params to a transport request.
// The generated code only implements what is necessary to turn a param into a valid value for these methods.
type request struct {
	pathPattern string
	method      string
	writer      runtime.ClientRequestWriter

	pathParams map[string]string
	header     http.Header
	query      url.Values
	formFields url.Values
	fileFields map[string][]runtime.NamedReadCloser
	payload    interface{}
	timeout    time.Duration
	buf        *bytes.Buffer

	getBody func(r *request) []byte
}

var (
	// ensure interface compliance
	_ runtime.ClientRequest = new(request)
)

func (r *request) isMultipart(mediaType string) bool {
	if len(r.fileFields) > 0 {
		return true
	}

	return runtime.MultipartFormMime == mediaType
}

// BuildHTTP creates a new http request based on the data from the params
func (r *request) BuildHTTP(mediaType, basePath string, producers map[string]runtime.Producer, registry strfmt.Registry) (*http.Request, error) {
	return r.buildHTTP(mediaType, basePath, producers, registry, nil)
}
func escapeQuotes(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(s)
}
func (r *request) buildHTTP(mediaType, basePath string, producers map[string]runtime.Producer, registry strfmt.Registry, auth runtime.ClientAuthInfoWriter) (*http.Request, error) {
	// build the data
	if err := r.writer.WriteToRequest(r, registry); err != nil {
		return nil, err
	}

	// Our body must be an io.Reader.
	// When we create the http.Request, if we pass it a
	// bytes.Buffer then it will wrap it in an io.ReadCloser
	// and set the content length automatically.
	var body io.Reader
	var pr *io.PipeReader
	var pw *io.PipeWriter

	r.buf = bytes.NewBuffer(nil)
	if r.payload != nil || len(r.formFields) > 0 || len(r.fileFields) > 0 {
		body = r.buf
		if r.isMultipart(mediaType) {
			pr, pw = io.Pipe()
			body = pr
		}
	}

	// check if this is a form type request
	if len(r.formFields) > 0 || len(r.fileFields) > 0 {
		if !r.isMultipart(mediaType) {
			r.header.Set(runtime.HeaderContentType, mediaType)
			formString := r.formFields.Encode()
			r.buf.WriteString(formString)
			goto DoneChoosingBodySource
		}

		mp := multipart.NewWriter(pw)
		r.header.Set(runtime.HeaderContentType, mangleContentType(mediaType, mp.Boundary()))

		go func() {
			defer func() {
				mp.Close()
				pw.Close()
			}()

			for fn, v := range r.formFields {
				for _, vi := range v {
					if err := mp.WriteField(fn, vi); err != nil {
						pw.CloseWithError(err)
						log.Println(err)
					}
				}
			}

			defer func() {
				for _, ff := range r.fileFields {
					for _, ffi := range ff {
						ffi.Close()
					}
				}
			}()
			for fn, f := range r.fileFields {
				for _, fi := range f {
					buf := bytes.NewBuffer([]byte{})

					// Need to read the data so that we can detect the content type
					_, err := io.Copy(buf, fi)
					if err != nil {
						_ = pw.CloseWithError(err)
						log.Println(err)
					}
					fileBytes := buf.Bytes()
					fileContentType := http.DetectContentType(fileBytes)

					newFi := runtime.NamedReader(fi.Name(), buf)

					// Create the MIME headers for the new part
					h := make(textproto.MIMEHeader)
					h.Set("Content-Disposition",
						fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
							escapeQuotes(fn), escapeQuotes(filepath.Base(fi.Name()))))
					h.Set("Content-Type", fileContentType)

					wrtr, err := mp.CreatePart(h)
					if err != nil {
						pw.CloseWithError(err)
						log.Println(err)
					} else if _, err := io.Copy(wrtr, newFi); err != nil {
						pw.CloseWithError(err)
						log.Println(err)
					}
				}
			}
		}()

		goto DoneChoosingBodySource
	}

	// if there is payload, use the producer to write the payload, and then
	// set the header to the content-type appropriate for the payload produced
	if r.payload != nil {
		// TODO: infer most appropriate content type based on the producer used,
		// and the `consumers` section of the spec/operation
		r.header.Set(runtime.HeaderContentType, mediaType)
		if rdr, ok := r.payload.(io.ReadCloser); ok {
			body = rdr
			goto DoneChoosingBodySource
		}

		if rdr, ok := r.payload.(io.Reader); ok {
			body = rdr
			goto DoneChoosingBodySource
		}

		producer := producers[mediaType]
		if err := producer.Produce(r.buf, r.payload); err != nil {
			return nil, err
		}
	}

DoneChoosingBodySource:

	if runtime.CanHaveBody(r.method) && body == nil && r.header.Get(runtime.HeaderContentType) == "" {
		r.header.Set(runtime.HeaderContentType, mediaType)
	}

	if auth != nil {
		// If we're not using r.buf as our http.Request's body,
		// either the payload is an io.Reader or io.ReadCloser,
		// or we're doing a multipart form/file.
		//
		// In those cases, if the AuthenticateRequest call asks for the body,
		// we must read it into a buffer and provide that, then use that buffer
		// as the body of our http.Request.
		//
		// This is done in-line with the GetBody() request rather than ahead
		// of time, because there's no way to know if the AuthenticateRequest
		// will even ask for the body of the request.
		//
		// If for some reason the copy fails, there's no way to return that
		// error to the GetBody() call, so return it afterwards.
		//
		// An error from the copy action is prioritized over any error
		// from the AuthenticateRequest call, because the mis-read
		// body may have interfered with the auth.
		//
		var copyErr error
		if buf, ok := body.(*bytes.Buffer); body != nil && (!ok || buf != r.buf) {
			var copied bool
			r.getBody = func(r *request) []byte {
				if copied {
					return getRequestBuffer(r)
				}

				defer func() {
					copied = true
				}()

				if _, copyErr = io.Copy(r.buf, body); copyErr != nil {
					return nil
				}

				if closer, ok := body.(io.ReadCloser); ok {
					if copyErr = closer.Close(); copyErr != nil {
						return nil
					}
				}

				body = r.buf
				return getRequestBuffer(r)
			}
		}

		authErr := auth.AuthenticateRequest(r, registry)

		if copyErr != nil {
			return nil, fmt.Errorf("error retrieving the response body: %v", copyErr)
		}

		if authErr != nil {
			return nil, authErr
		}
	}

	// create http request
	var reinstateSlash bool
	if r.pathPattern != "" && r.pathPattern != "/" && r.pathPattern[len(r.pathPattern)-1] == '/' {
		reinstateSlash = true
	}

	// In case the basePath includes hardcoded query parameters, parse those out before
	// constructing the final path. The parameters themselves will be merged with the
	// ones set by the client, with the priority given to the latter.
	basePathURL, err := url.Parse(basePath)
	if err != nil {
		return nil, err
	}
	basePathQueryParams := basePathURL.Query()

	urlPath := path.Join(basePathURL.Path, r.pathPattern)
	for k, v := range r.pathParams {
		urlPath = strings.Replace(urlPath, "{"+k+"}", url.PathEscape(v), -1)
	}
	if reinstateSlash {
		urlPath = urlPath + "/"
	}

	req, err := http.NewRequest(r.method, urlPath, body)
	if err != nil {
		return nil, err
	}

	originalParams := r.GetQueryParams()

	// Merge the query parameters extracted from the basePath with the ones set by
	// the client in this struct. In case of conflict, the client wins.
	for k, v := range basePathQueryParams {
		_, present := originalParams[k]
		if !present {
			if err = r.SetQueryParam(k, v...); err != nil {
				return nil, err
			}
		}
	}

	req.URL.RawQuery = r.query.Encode()
	req.Header = r.header

	return req, nil
}

func mangleContentType(mediaType, boundary string) string {
	if strings.ToLower(mediaType) == runtime.URLencodedFormMime {
		return fmt.Sprintf("%s; boundary=%s", mediaType, boundary)
	}
	return "multipart/form-data; boundary=" + boundary
}

func (r *request) GetMethod() string {
	return r.method
}

func (r *request) GetPath() string {
	path := r.pathPattern
	for k, v := range r.pathParams {
		path = strings.Replace(path, "{"+k+"}", v, -1)
	}
	return path
}

func (r *request) GetBody() []byte {
	return r.getBody(r)
}

func getRequestBuffer(r *request) []byte {
	if r.buf == nil {
		return nil
	}
	return r.buf.Bytes()
}

// SetHeaderParam adds a header param to the request
// when there is only 1 value provided for the varargs, it will set it.
// when there are several values provided for the varargs it will add it (no overriding)
func (r *request) SetHeaderParam(name string, values ...string) error {
	if r.header == nil {
		r.header = make(http.Header)
	}
	r.header[http.CanonicalHeaderKey(name)] = values
	return nil
}

// GetHeaderParams returns the all headers currently set for the request
func (r *request) GetHeaderParams() http.Header {
	return r.header
}

// SetQueryParam adds a query param to the request
// when there is only 1 value provided for the varargs, it will set it.
// when there are several values provided for the varargs it will add it (no overriding)
func (r *request) SetQueryParam(name string, values ...string) error {
	if r.query == nil {
		r.query = make(url.Values)
	}
	r.query[name] = values
	return nil
}

// GetQueryParams returns a copy of all query params currently set for the request
func (r *request) GetQueryParams() url.Values {
	var result = make(url.Values)
	for key, value := range r.query {
		result[key] = append([]string{}, value...)
	}
	return result
}

// SetFormParam adds a forn param to the request
// when there is only 1 value provided for the varargs, it will set it.
// when there are several values provided for the varargs it will add it (no overriding)
func (r *request) SetFormParam(name string, values ...string) error {
	if r.formFields == nil {
		r.formFields = make(url.Values)
	}
	r.formFields[name] = values
	return nil
}

// SetPathParam adds a path param to the request
func (r *request) SetPathParam(name string, value string) error {
	if r.pathParams == nil {
		r.pathParams = make(map[string]string)
	}

	r.pathParams[name] = value
	return nil
}

// SetFileParam adds a file param to the request
func (r *request) SetFileParam(name string, files ...runtime.NamedReadCloser) error {
	for _, file := range files {
		if actualFile, ok := file.(*os.File); ok {
			fi, err := os.Stat(actualFile.Name())
			if err != nil {
				return err
			}
			if fi.IsDir() {
				return fmt.Errorf("%q is a directory, only files are supported", file.Name())
			}
		}
	}

	if r.fileFields == nil {
		r.fileFields = make(map[string][]runtime.NamedReadCloser)
	}
	if r.formFields == nil {
		r.formFields = make(url.Values)
	}

	r.fileFields[name] = files
	return nil
}

func (r *request) GetFileParam() map[string][]runtime.NamedReadCloser {
	return r.fileFields
}

// SetBodyParam sets a body parameter on the request.
// This does not yet serialze the object, this happens as late as possible.
func (r *request) SetBodyParam(payload interface{}) error {
	r.payload = payload
	return nil
}

func (r *request) GetBodyParam() interface{} {
	return r.payload
}

// SetTimeout sets the timeout for a request
func (r *request) SetTimeout(timeout time.Duration) error {
	r.timeout = timeout
	return nil
}
