// Package specerror implements runtime-spec-specific tooling for
// tracking RFC 2119 violations.
package specerror

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	rfc2119 "github.com/opencontainers/runtime-tools/error"
)

const referenceTemplate = "https://github.com/opencontainers/runtime-spec/blob/v%s/%s"

// Code represents the spec violation, enumerating both
// configuration violations and runtime violations.
type Code int

const (
	// NonError represents that an input is not an error
	NonError Code = iota
	// NonRFCError represents that an error is not a rfc2119 error
	NonRFCError

	// ConfigFileExistence represents the error code of 'config.json' existence test
	ConfigFileExistence
	// ArtifactsInSingleDir represents the error code of artifacts place test
	ArtifactsInSingleDir

	// SpecVersion represents the error code of specfication version test
	SpecVersion

	// RootOnNonHyperV represents the error code of root setting test on non hyper-v containers
	RootOnNonHyperV
	// RootOnHyperV represents the error code of root setting test on hyper-v containers
	RootOnHyperV
	// PathFormatOnWindows represents the error code of the path format test on Window
	PathFormatOnWindows
	// PathName represents the error code of the path name test
	PathName
	// PathExistence represents the error code of the path existence test
	PathExistence
	// ReadonlyFilesystem represents the error code of readonly test
	ReadonlyFilesystem
	// ReadonlyOnWindows represents the error code of readonly setting test on Windows
	ReadonlyOnWindows

	// DefaultFilesystems represents the error code of default filesystems test
	DefaultFilesystems

	// CreateWithID represents the error code of 'create' lifecyle test with 'id' provided
	CreateWithID
	// CreateWithUniqueID represents the error code of 'create' lifecyle test with unique 'id' provided
	CreateWithUniqueID
	// CreateNewContainer represents the error code 'create' lifecyle test that creates new container
	CreateNewContainer
)

type errorTemplate struct {
	Level     rfc2119.Level
	Reference func(version string) (reference string, err error)
}

// Error represents a runtime-spec violation.
type Error struct {
	// Err holds the RFC 2119 violation.
	Err rfc2119.Error

	// Code is a matchable holds a Code
	Code Code
}

var (
	containerFormatRef = func(version string) (reference string, err error) {
		return fmt.Sprintf(referenceTemplate, version, "bundle.md#container-format"), nil
	}
	specVersionRef = func(version string) (reference string, err error) {
		return fmt.Sprintf(referenceTemplate, version, "config.md#specification-version"), nil
	}
	rootRef = func(version string) (reference string, err error) {
		return fmt.Sprintf(referenceTemplate, version, "config.md#root"), nil
	}
	defaultFSRef = func(version string) (reference string, err error) {
		return fmt.Sprintf(referenceTemplate, version, "config-linux.md#default-filesystems"), nil
	}
	runtimeCreateRef = func(version string) (reference string, err error) {
		return fmt.Sprintf(referenceTemplate, version, "runtime.md#create"), nil
	}
)

var ociErrors = map[Code]errorTemplate{
	// Bundle.md
	// Container Format
	ConfigFileExistence:  {Level: rfc2119.Must, Reference: containerFormatRef},
	ArtifactsInSingleDir: {Level: rfc2119.Must, Reference: containerFormatRef},

	// Config.md
	// Specification Version
	SpecVersion: {Level: rfc2119.Must, Reference: specVersionRef},
	// Root
	RootOnNonHyperV: {Level: rfc2119.Required, Reference: rootRef},
	RootOnHyperV:    {Level: rfc2119.Must, Reference: rootRef},
	// TODO: add tests for 'PathFormatOnWindows'
	PathFormatOnWindows: {Level: rfc2119.Must, Reference: rootRef},
	PathName:            {Level: rfc2119.Should, Reference: rootRef},
	PathExistence:       {Level: rfc2119.Must, Reference: rootRef},
	ReadonlyFilesystem:  {Level: rfc2119.Must, Reference: rootRef},
	ReadonlyOnWindows:   {Level: rfc2119.Must, Reference: rootRef},

	// Config-Linux.md
	// Default Filesystems
	DefaultFilesystems: {Level: rfc2119.Should, Reference: defaultFSRef},

	// Runtime.md
	// Create
	CreateWithID:       {Level: rfc2119.Must, Reference: runtimeCreateRef},
	CreateWithUniqueID: {Level: rfc2119.Must, Reference: runtimeCreateRef},
	CreateNewContainer: {Level: rfc2119.Must, Reference: runtimeCreateRef},
}

// Error returns the error message with specification reference.
func (err *Error) Error() string {
	return err.Err.Error()
}

// NewError creates an Error referencing a spec violation.  The error
// can be cast to an *Error for extracting structured information
// about the level of the violation and a reference to the violated
// spec condition.
//
// A version string (for the version of the spec that was violated)
// must be set to get a working URL.
func NewError(code Code, err error, version string) error {
	template := ociErrors[code]
	reference, err2 := template.Reference(version)
	if err2 != nil {
		return err2
	}
	return &Error{
		Err: rfc2119.Error{
			Level:     template.Level,
			Reference: reference,
			Err:       err,
		},
		Code: code,
	}
}

// FindError finds an error from a source error (multiple error) and
// returns the error code if found.
// If the source error is nil or empty, return NonError.
// If the source error is not a multiple error, return NonRFCError.
func FindError(err error, code Code) Code {
	if err == nil {
		return NonError
	}

	if merr, ok := err.(*multierror.Error); ok {
		if merr.ErrorOrNil() == nil {
			return NonError
		}
		for _, e := range merr.Errors {
			if rfcErr, ok := e.(*Error); ok {
				if rfcErr.Code == code {
					return code
				}
			}
		}
	}
	return NonRFCError
}
