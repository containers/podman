package metric

import "strings"

//AttackComplexity is metric type for Base Metrics
type AttackComplexity int

//Constant of AttackComplexity result
const (
	AttackComplexityUnknown AttackComplexity = iota
	AttackComplexityNotDefined
	AttackComplexityHigh
	AttackComplexityLow
)

var attackComplexityMap = map[AttackComplexity]string{
	AttackComplexityNotDefined: "X",
	AttackComplexityHigh:       "H",
	AttackComplexityLow:        "L",
}

var attackComplexityValueMap = map[AttackComplexity]float64{
	AttackComplexityNotDefined: 0,
	AttackComplexityHigh:       0.44,
	AttackComplexityLow:        0.77,
}

//GetAttackComplexity returns result of AttackComplexity metric
func GetAttackComplexity(s string) AttackComplexity {
	s = strings.ToUpper(s)
	for k, v := range attackComplexityMap {
		if s == v {
			return k
		}
	}
	return AttackComplexityUnknown
}

func (ac AttackComplexity) String() string {
	if s, ok := attackComplexityMap[ac]; ok {
		return s
	}
	return ""
}

//Value returns value of AttackComplexity metric
func (ac AttackComplexity) Value() float64 {
	if v, ok := attackComplexityValueMap[ac]; ok {
		return v
	}
	return 0.0
}

//IsUnknown returns false if unknown result value of metric
func (ac AttackComplexity) IsUnknown() bool {
	return ac == AttackComplexityUnknown
}

//IsDefined returns false if undefined result value of metric
func (ac AttackComplexity) IsDefined() bool {
	return !ac.IsUnknown() && ac != AttackComplexityNotDefined
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
