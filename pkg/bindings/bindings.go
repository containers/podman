// Package bindings provides golang-based access
// to the Podman REST API.  Users can then interact with API endpoints
// to manage containers, images, pods, etc.
//
// This package exposes a series of methods that allow users to firstly
// create their connection with the API endpoints.  Once the connection
// is established, users can then manage the Podman container runtime.

package bindings

import (
	"github.com/blang/semver"
)

var (
	// PTrue is a convenience variable that can be used in bindings where
	// a pointer to a bool (optional parameter) is required.
	pTrue = true
	PTrue = &pTrue
	// PFalse is a convenience variable that can be used in bindings where
	// a pointer to a bool (optional parameter) is required.
	pFalse = false
	PFalse = &pFalse

	// _*YES*- podman will fail to run if this value is wrong
	APIVersion = semver.MustParse("1.0.0")
)
