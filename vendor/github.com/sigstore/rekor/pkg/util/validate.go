//
// Copyright 2021 The Sigstore Authors.
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

package util

import (
	"strings"

	validator "github.com/go-playground/validator/v10"
)

// validateSHA512Value ensures that the supplied string matches the
// following format: [sha512:]<128 hexadecimal characters>
// where [sha512:] is optional
func ValidateSHA512Value(v string) error {
	var prefix, hash string

	split := strings.SplitN(v, ":", 2)
	switch len(split) {
	case 1:
		hash = split[0]
	case 2:
		prefix = split[0]
		hash = split[1]
	}

	s := struct {
		Prefix string `validate:"omitempty,oneof=sha512"`
		Hash   string `validate:"required,len=128,hexadecimal"`
	}{prefix, hash}

	validate := validator.New()
	return validate.Struct(s)
}

// validateSHA256Value ensures that the supplied string matches the following format:
// [sha256:]<64 hexadecimal characters>
// where [sha256:] is optional
func ValidateSHA256Value(v string) error {
	var prefix, hash string

	split := strings.SplitN(v, ":", 2)
	switch len(split) {
	case 1:
		hash = split[0]
	case 2:
		prefix = split[0]
		hash = split[1]
	}

	s := struct {
		Prefix string `validate:"omitempty,oneof=sha256"`
		Hash   string `validate:"required,len=64,hexadecimal"`
	}{prefix, hash}

	validate := validator.New()
	return validate.Struct(s)
}

func ValidateSHA1Value(v string) error {
	var prefix, hash string

	split := strings.SplitN(v, ":", 2)
	switch len(split) {
	case 1:
		hash = split[0]
	case 2:
		prefix = split[0]
		hash = split[1]
	}

	s := struct {
		Prefix string `validate:"omitempty,oneof=sha1"`
		Hash   string `validate:"required,len=40,hexadecimal"`
	}{prefix, hash}

	validate := validator.New()
	return validate.Struct(s)

}
