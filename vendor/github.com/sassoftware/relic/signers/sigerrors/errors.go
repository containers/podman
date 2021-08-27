//
// Copyright (c) SAS Institute Inc.
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
//

package sigerrors

import (
	"errors"
)

var (
	ErrExist = errors.New("object already exists in token")
)

type KeyNotFoundError struct{}

func (KeyNotFoundError) Error() string {
	return "No object found in token with the specified label"
}

type PinIncorrectError struct{}

func (PinIncorrectError) Error() string {
	return "The entered PIN was incorrect"
}

type ErrNoCertificate struct {
	Type string
}

func (e ErrNoCertificate) Error() string {
	return "no certificate of type \"" + e.Type + "\" defined for this key"
}

type NotSignedError struct {
	Type string
}

func (e NotSignedError) Error() string {
	return e.Type + " contains no signatures"
}
