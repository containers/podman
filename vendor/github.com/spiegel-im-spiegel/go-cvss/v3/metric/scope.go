package metric

import "strings"

//Scope is metric type for Base Metrics
type Scope int

//Constant of Scope result
const (
	ScopeUnknown Scope = iota
	ScopeNotDefined
	ScopeUnchanged
	ScopeChanged
)

var scopeMap = map[Scope]string{
	ScopeNotDefined: "X",
	ScopeUnchanged:  "U",
	ScopeChanged:    "C",
}

//GetScope returns result of Scope metric
func GetScope(s string) Scope {
	s = strings.ToUpper(s)
	for k, v := range scopeMap {
		if s == v {
			return k
		}
	}
	return ScopeUnknown
}

func (sc Scope) String() string {
	if s, ok := scopeMap[sc]; ok {
		return s
	}
	return ""
}

//IsUnknown returns false if undefined result value of metric
func (sc Scope) IsUnknown() bool {
	return sc == ScopeUnknown
}

//IsDefined returns false if undefined result value of metric
func (sc Scope) IsDefined() bool {
	return !sc.IsUnknown() && sc != ScopeNotDefined
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
