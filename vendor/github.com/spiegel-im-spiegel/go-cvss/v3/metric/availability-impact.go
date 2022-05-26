package metric

import "strings"

//AvailabilityImpact is metric type for Base Metrics
type AvailabilityImpact int

//Constant of AvailabilityImpact result
const (
	AvailabilityImpactUnknown AvailabilityImpact = iota
	AvailabilityImpactNotDefined
	AvailabilityImpactNone
	AvailabilityImpactLow
	AvailabilityImpactHigh
)

var availabilityImpactMap = map[AvailabilityImpact]string{
	AvailabilityImpactNotDefined: "X",
	AvailabilityImpactNone:       "N",
	AvailabilityImpactLow:        "L",
	AvailabilityImpactHigh:       "H",
}

var availabilityImpactValueMap = map[AvailabilityImpact]float64{
	AvailabilityImpactNotDefined: 0.00,
	AvailabilityImpactNone:       0.00,
	AvailabilityImpactLow:        0.22,
	AvailabilityImpactHigh:       0.56,
}

//GetAvailabilityImpact returns result of AvailabilityImpact metric
func GetAvailabilityImpact(s string) AvailabilityImpact {
	s = strings.ToUpper(s)
	for k, v := range availabilityImpactMap {
		if s == v {
			return k
		}
	}
	return AvailabilityImpactUnknown
}

func (ai AvailabilityImpact) String() string {
	if s, ok := availabilityImpactMap[ai]; ok {
		return s
	}
	return ""
}

//Value returns value of AvailabilityImpact metric
func (ai AvailabilityImpact) Value() float64 {
	if v, ok := availabilityImpactValueMap[ai]; ok {
		return v
	}
	return 0.0
}

//IsUnknown returns false if undefined result value of metric
func (ai AvailabilityImpact) IsUnknown() bool {
	return ai == AvailabilityImpactUnknown
}

//IsDefined returns false if undefined result value of metric
func (ai AvailabilityImpact) IsDefined() bool {
	return !ai.IsUnknown() && ai != AvailabilityImpactNotDefined
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
