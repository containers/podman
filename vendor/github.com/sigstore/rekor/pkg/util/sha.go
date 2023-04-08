// Copyright 2022 The Sigstore Authors.
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
	"fmt"
	"strings"
)

// PrefixSHA sets the prefix of a sha hash to match how it is stored based on the length.
func PrefixSHA(sha string) string {
	var prefix string
	var components = strings.Split(sha, ":")

	if len(components) == 2 {
		return sha
	}

	switch len(sha) {
	case 40:
		prefix = "sha1:"
	case 64:
		prefix = "sha256:"
	case 128:
		prefix = "sha512:"
	}

	return fmt.Sprintf("%v%v", prefix, sha)
}
