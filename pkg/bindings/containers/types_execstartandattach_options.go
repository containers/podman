package containers

import (
	"bufio"
	"io"
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *ExecStartAndAttachOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *ExecStartAndAttachOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
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
