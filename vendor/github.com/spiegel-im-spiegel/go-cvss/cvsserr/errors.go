package cvsserr

import "fmt"

//Num is error number for CVSS
type Num int

const (
	ErrNullPointer Num = iota + 1
	ErrUndefinedMetric
	ErrInvalidVector
	ErrNotSupportVer
	ErrNotSupportMetric
	ErrInvalidTemplate
)

var errMessage = map[Num]string{
	ErrNullPointer:      "Null reference instance",
	ErrUndefinedMetric:  "undefined metric",
	ErrInvalidVector:    "invalid vector",
	ErrNotSupportVer:    "not support version",
	ErrNotSupportMetric: "not support metric",
	ErrInvalidTemplate:  "invalid templete string",
}

func (n Num) Error() string {
	if s, ok := errMessage[n]; ok {
		return s
	}
	return fmt.Sprintf("unknown error (%d)", int(n))
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
