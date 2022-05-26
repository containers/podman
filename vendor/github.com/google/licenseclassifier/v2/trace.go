// Copyright 2020 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package classifier

import (
	"fmt"
	"strings"
)

// This file contains routines for a simple trace execution mechanism.

// TraceConfiguration specifies the configuration for tracing execution of the
// license classifier.
type TraceConfiguration struct {
	// Comma-separated list of phases to be traced. Can use * for all phases.
	TracePhases string
	// Comma-separated list of licenses to be traced. Can use * as a suffix to
	// match prefixes, or by itself to match all licenses.
	TraceLicenses string

	// Tracer specifies a TraceFunc used to capture tracing information.
	// If not supplied, emits using fmt.Printf
	Tracer        TraceFunc
	tracePhases   map[string]bool
	traceLicenses map[string]bool
}

func (t *TraceConfiguration) init() {
	if t == nil {
		return
	}
	// Sample the config values to create the lookup maps
	t.traceLicenses = make(map[string]bool)
	t.tracePhases = make(map[string]bool)

	if len(t.TraceLicenses) > 0 {
		for _, lic := range strings.Split(t.TraceLicenses, ",") {
			t.traceLicenses[lic] = true
		}
	}

	if len(t.TracePhases) > 0 {
		for _, phase := range strings.Split(t.TracePhases, ",") {
			t.tracePhases[phase] = true
		}
	}
}

var traceLicenses map[string]bool
var tracePhases map[string]bool

func (t *TraceConfiguration) shouldTrace(phase string) bool {
	if t == nil {
		return false
	}
	if t.tracePhases["*"] {
		return true
	}
	return t.tracePhases[phase]
}

func (t *TraceConfiguration) isTraceLicense(lic string) bool {
	if t == nil {
		return false
	}
	if t.traceLicenses[lic] {
		return true
	}

	for e := range t.traceLicenses {
		if idx := strings.Index(e, "*"); idx != -1 {
			if strings.HasPrefix(lic, e[0:idx]) {
				return true
			}
		}
	}

	return false
}

func (t *TraceConfiguration) trace(f string, args ...interface{}) {
	if t == nil || t.Tracer == nil {
		fmt.Printf(f, args...)
		fmt.Println()
		return
	}

	t.Tracer(f, args...)
}

func (t *TraceConfiguration) traceSearchset(lic string) bool {
	return t.isTraceLicense(lic) && t.shouldTrace("searchset")
}

func (t *TraceConfiguration) traceTokenize(lic string) bool {
	return t.isTraceLicense(lic) && t.shouldTrace("tokenize")
}

func (t *TraceConfiguration) traceScoring(lic string) bool {
	return t.isTraceLicense(lic) && t.shouldTrace("score")
}

func (t *TraceConfiguration) traceFrequency(lic string) bool {
	return t.isTraceLicense(lic) && t.shouldTrace("frequency")
}

// TraceFunc works like fmt.Printf to emit tracing data for the
// classifier.
type TraceFunc func(string, ...interface{})
