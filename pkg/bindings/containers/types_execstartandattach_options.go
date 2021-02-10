package containers

import (
	"bufio"
	"io"
	"net/url"
	"reflect"
	"strconv"
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

	if o.OutputStream != nil {
		panic("*** GENERATOR DOESN'T IMPLEMENT THIS YET ***")
	}

	if o.ErrorStream != nil {
		panic("*** GENERATOR DOESN'T IMPLEMENT THIS YET ***")
	}

	if o.InputStream != nil {
		panic("*** GENERATOR DOESN'T IMPLEMENT THIS YET ***")
	}

	if o.AttachOutput != nil {
		params.Set("attachoutput", strconv.FormatBool(*o.AttachOutput))
	}

	if o.AttachError != nil {
		params.Set("attacherror", strconv.FormatBool(*o.AttachError))
	}

	if o.AttachInput != nil {
		params.Set("attachinput", strconv.FormatBool(*o.AttachInput))
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
