package containers

import (
	"bufio"
	"io"
	"net/url"
	"reflect"
	"strings"

	"github.com/containers/podman/v2/pkg/bindings/util"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ExecStartAndAttachOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *ExecStartAndAttachOptions) ToParams() (url.Values, error) {
	params := url.Values{}
	if o == nil {
		return params, nil
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	s := reflect.ValueOf(o)
	if reflect.Ptr == s.Kind() {
		s = s.Elem()
	}
	sType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		fieldName := sType.Field(i).Name
		if !o.Changed(fieldName) {
			continue
		}
		fieldName = strings.ToLower(fieldName)
		f := s.Field(i)
		if reflect.Ptr == f.Kind() {
			f = f.Elem()
		}
		switch {
		case util.IsSimpleType(f):
			params.Set(fieldName, util.SimpleTypeToParam(f))
		case f.Kind() == reflect.Slice:
			for i := 0; i < f.Len(); i++ {
				elem := f.Index(i)
				if util.IsSimpleType(elem) {
					params.Add(fieldName, util.SimpleTypeToParam(elem))
				} else {
					return nil, errors.New("slices must contain only simple types")
				}
			}
		case f.Kind() == reflect.Map:
			lowerCaseKeys := make(map[string][]string)
			iter := f.MapRange()
			for iter.Next() {
				lowerCaseKeys[iter.Key().Interface().(string)] = iter.Value().Interface().([]string)

			}
			s, err := json.MarshalToString(lowerCaseKeys)
			if err != nil {
				return nil, err
			}

			params.Set(fieldName, s)
		}

	}
	return params, nil
}

// WithOutputStream
func (o *ExecStartAndAttachOptions) WithOutputStream(value io.WriteCloser) *ExecStartAndAttachOptions {
	v := &value
	o.OutputStream = v
	return o
}

// GetOutputStream
func (o *ExecStartAndAttachOptions) GetOutputStream() io.WriteCloser {
	var outputStream io.WriteCloser
	if o.OutputStream == nil {
		return outputStream
	}
	return *o.OutputStream
}

// WithErrorStream
func (o *ExecStartAndAttachOptions) WithErrorStream(value io.WriteCloser) *ExecStartAndAttachOptions {
	v := &value
	o.ErrorStream = v
	return o
}

// GetErrorStream
func (o *ExecStartAndAttachOptions) GetErrorStream() io.WriteCloser {
	var errorStream io.WriteCloser
	if o.ErrorStream == nil {
		return errorStream
	}
	return *o.ErrorStream
}

// WithInputStream
func (o *ExecStartAndAttachOptions) WithInputStream(value bufio.Reader) *ExecStartAndAttachOptions {
	v := &value
	o.InputStream = v
	return o
}

// GetInputStream
func (o *ExecStartAndAttachOptions) GetInputStream() bufio.Reader {
	var inputStream bufio.Reader
	if o.InputStream == nil {
		return inputStream
	}
	return *o.InputStream
}

// WithAttachOutput
func (o *ExecStartAndAttachOptions) WithAttachOutput(value bool) *ExecStartAndAttachOptions {
	v := &value
	o.AttachOutput = v
	return o
}

// GetAttachOutput
func (o *ExecStartAndAttachOptions) GetAttachOutput() bool {
	var attachOutput bool
	if o.AttachOutput == nil {
		return attachOutput
	}
	return *o.AttachOutput
}

// WithAttachError
func (o *ExecStartAndAttachOptions) WithAttachError(value bool) *ExecStartAndAttachOptions {
	v := &value
	o.AttachError = v
	return o
}

// GetAttachError
func (o *ExecStartAndAttachOptions) GetAttachError() bool {
	var attachError bool
	if o.AttachError == nil {
		return attachError
	}
	return *o.AttachError
}

// WithAttachInput
func (o *ExecStartAndAttachOptions) WithAttachInput(value bool) *ExecStartAndAttachOptions {
	v := &value
	o.AttachInput = v
	return o
}

// GetAttachInput
func (o *ExecStartAndAttachOptions) GetAttachInput() bool {
	var attachInput bool
	if o.AttachInput == nil {
		return attachInput
	}
	return *o.AttachInput
}
