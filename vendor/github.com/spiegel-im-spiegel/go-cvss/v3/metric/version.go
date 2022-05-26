package metric

import (
	"strings"

	"github.com/spiegel-im-spiegel/errs"
	"github.com/spiegel-im-spiegel/go-cvss/cvsserr"
)

//Version is error number for CVSS
type Version int

const (
	VUnknown Version = iota //unknown version
	V3_0                    //v3.0
	V3_1                    //v3.1
)

var verStrings = map[Version]string{
	V3_0: "3.0",
	V3_1: "3.1",
}

//String is Stringer method
func (n Version) String() string {
	if s, ok := verStrings[n]; ok {
		return s
	}
	return "unknown"
}

//GetVersion returns Version number from string
func GetVersion(vec string) (Version, error) {
	v := strings.Split(vec, ":")
	if len(v) != 2 {
		return VUnknown, errs.Wrap(cvsserr.ErrInvalidVector, errs.WithContext("vector", vec))
	}
	if strings.ToUpper(v[0]) != "CVSS" {
		return VUnknown, errs.Wrap(cvsserr.ErrInvalidVector, errs.WithContext("vector", vec))
	}
	return get(v[1]), nil
}

func get(s string) Version {
	for k, v := range verStrings {
		if s == v {
			return k
		}
	}
	return VUnknown
}

/* Copyright 2018-2020 Spiegel
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
