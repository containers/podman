/*
 *
 * Copyright 2021 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package xdsresource contains functions to proto xds updates (unmarshal from
// proto), and types for the resource updates.
package xdsresource

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/protobuf/types/known/anypb"
)

// UnmarshalOptions wraps the input parameters for `UnmarshalXxx` functions.
type UnmarshalOptions struct {
	// Version is the version of the received response.
	Version string
	// Resources are the xDS resources resources in the received response.
	Resources []*anypb.Any
	// Logger is the prefix logger to be used during unmarshaling.
	Logger *grpclog.PrefixLogger
	// UpdateValidator is a post unmarshal validation check provided by the
	// upper layer.
	UpdateValidator UpdateValidatorFunc
}

// processAllResources unmarshals and validates the resources, populates the
// provided ret (a map), and returns metadata and error.
//
// After this function, the ret map will be populated with both valid and
// invalid updates. Invalid resources will have an entry with the key as the
// resource name, value as an empty update.
//
// The type of the resource is determined by the type of ret. E.g.
// map[string]ListenerUpdate means this is for LDS.
func processAllResources(opts *UnmarshalOptions, ret interface{}) (UpdateMetadata, error) {
	timestamp := time.Now()
	md := UpdateMetadata{
		Version:   opts.Version,
		Timestamp: timestamp,
	}
	var topLevelErrors []error
	perResourceErrors := make(map[string]error)

	for _, r := range opts.Resources {
		switch ret2 := ret.(type) {
		case map[string]ListenerUpdateErrTuple:
			name, update, err := unmarshalListenerResource(r, opts.UpdateValidator, opts.Logger)
			name = ParseName(name).String()
			if err == nil {
				ret2[name] = ListenerUpdateErrTuple{Update: update}
				continue
			}
			if name == "" {
				topLevelErrors = append(topLevelErrors, err)
				continue
			}
			perResourceErrors[name] = err
			// Add place holder in the map so we know this resource name was in
			// the response.
			ret2[name] = ListenerUpdateErrTuple{Err: err}
		case map[string]RouteConfigUpdateErrTuple:
			name, update, err := unmarshalRouteConfigResource(r, opts.Logger)
			name = ParseName(name).String()
			if err == nil {
				ret2[name] = RouteConfigUpdateErrTuple{Update: update}
				continue
			}
			if name == "" {
				topLevelErrors = append(topLevelErrors, err)
				continue
			}
			perResourceErrors[name] = err
			// Add place holder in the map so we know this resource name was in
			// the response.
			ret2[name] = RouteConfigUpdateErrTuple{Err: err}
		case map[string]ClusterUpdateErrTuple:
			name, update, err := unmarshalClusterResource(r, opts.UpdateValidator, opts.Logger)
			name = ParseName(name).String()
			if err == nil {
				ret2[name] = ClusterUpdateErrTuple{Update: update}
				continue
			}
			if name == "" {
				topLevelErrors = append(topLevelErrors, err)
				continue
			}
			perResourceErrors[name] = err
			// Add place holder in the map so we know this resource name was in
			// the response.
			ret2[name] = ClusterUpdateErrTuple{Err: err}
		case map[string]EndpointsUpdateErrTuple:
			name, update, err := unmarshalEndpointsResource(r, opts.Logger)
			name = ParseName(name).String()
			if err == nil {
				ret2[name] = EndpointsUpdateErrTuple{Update: update}
				continue
			}
			if name == "" {
				topLevelErrors = append(topLevelErrors, err)
				continue
			}
			perResourceErrors[name] = err
			// Add place holder in the map so we know this resource name was in
			// the response.
			ret2[name] = EndpointsUpdateErrTuple{Err: err}
		}
	}

	if len(topLevelErrors) == 0 && len(perResourceErrors) == 0 {
		md.Status = ServiceStatusACKed
		return md, nil
	}

	var typeStr string
	switch ret.(type) {
	case map[string]ListenerUpdate:
		typeStr = "LDS"
	case map[string]RouteConfigUpdate:
		typeStr = "RDS"
	case map[string]ClusterUpdate:
		typeStr = "CDS"
	case map[string]EndpointsUpdate:
		typeStr = "EDS"
	}

	md.Status = ServiceStatusNACKed
	errRet := combineErrors(typeStr, topLevelErrors, perResourceErrors)
	md.ErrState = &UpdateErrorMetadata{
		Version:   opts.Version,
		Err:       errRet,
		Timestamp: timestamp,
	}
	return md, errRet
}

func combineErrors(rType string, topLevelErrors []error, perResourceErrors map[string]error) error {
	var errStrB strings.Builder
	errStrB.WriteString(fmt.Sprintf("error parsing %q response: ", rType))
	if len(topLevelErrors) > 0 {
		errStrB.WriteString("top level errors: ")
		for i, err := range topLevelErrors {
			if i != 0 {
				errStrB.WriteString(";\n")
			}
			errStrB.WriteString(err.Error())
		}
	}
	if len(perResourceErrors) > 0 {
		var i int
		for name, err := range perResourceErrors {
			if i != 0 {
				errStrB.WriteString(";\n")
			}
			i++
			errStrB.WriteString(fmt.Sprintf("resource %q: %v", name, err.Error()))
		}
	}
	return errors.New(errStrB.String())
}
