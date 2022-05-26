package metric

import "strings"

//IntegrityImpact is metric type for Base Metrics
type IntegrityImpact int

//Constant of IntegrityImpact result
const (
	IntegrityImpactUnknown IntegrityImpact = iota
	IntegrityImpactNotDefined
	IntegrityImpactNone
	IntegrityImpactLow
	IntegrityImpactHigh
)

var integrityImpactMap = map[IntegrityImpact]string{
	IntegrityImpactNotDefined: "X",
	IntegrityImpactNone:       "N",
	IntegrityImpactLow:        "L",
	IntegrityImpactHigh:       "H",
}

var integrityImpactValueMap = map[IntegrityImpact]float64{
	IntegrityImpactNotDefined: 0.00,
	IntegrityImpactNone:       0.00,
	IntegrityImpactLow:        0.22,
	IntegrityImpactHigh:       0.56,
}

//GetIntegrityImpact returns result of IntegrityImpact metric
func GetIntegrityImpact(s string) IntegrityImpact {
	s = strings.ToUpper(s)
	for k, v := range integrityImpactMap {
		if s == v {
			return k
		}
	}
	return IntegrityImpactUnknown
}

func (ii IntegrityImpact) String() string {
	if s, ok := integrityImpactMap[ii]; ok {
		return s
	}
	return ""
}

//Value returns value of IntegrityImpact metric
func (ii IntegrityImpact) Value() float64 {
	if v, ok := integrityImpactValueMap[ii]; ok {
		return v
	}
	return 0.0
}

//IsUnKnown returns false if undefined result value of metric
func (ii IntegrityImpact) IsUnKnown() bool {
	return ii == IntegrityImpactUnknown
}

//IsDefined returns false if undefined result value of metric
func (ii IntegrityImpact) IsDefined() bool {
	return !ii.IsUnKnown() && ii != IntegrityImpactNotDefined
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
