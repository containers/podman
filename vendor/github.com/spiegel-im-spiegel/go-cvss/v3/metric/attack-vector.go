package metric

import "strings"

//AttackVector is metric type for Base Metrics
type AttackVector int

//Constant of AttackVector result
const (
	AttackVectorUnknown AttackVector = iota
	AttackVectorNotDefined
	AttackVectorPhysical
	AttackVectorLocal
	AttackVectorAdjacent
	AttackVectorNetwork
)

var attackVectorMap = map[AttackVector]string{
	AttackVectorNotDefined: "X",
	AttackVectorPhysical:   "P",
	AttackVectorLocal:      "L",
	AttackVectorAdjacent:   "A",
	AttackVectorNetwork:    "N",
}

var attackVectorValueMap = map[AttackVector]float64{
	AttackVectorNotDefined: 0,
	AttackVectorPhysical:   0.20,
	AttackVectorLocal:      0.55,
	AttackVectorAdjacent:   0.62,
	AttackVectorNetwork:    0.85,
}

//GetAttackVector returns result of AttackVector metric
func GetAttackVector(s string) AttackVector {
	s = strings.ToUpper(s)
	for k, v := range attackVectorMap {
		if s == v {
			return k
		}
	}
	return AttackVectorUnknown
}

func (av AttackVector) String() string {
	if s, ok := attackVectorMap[av]; ok {
		return s
	}
	return ""
}

//Value returns value of AttackVector metric
func (av AttackVector) Value() float64 {
	if v, ok := attackVectorValueMap[av]; ok {
		return v
	}
	return 0.0
}

//IsUnknown returns false if unknouwn result value of metric
func (av AttackVector) IsUnknown() bool {
	return av == AttackVectorUnknown
}

//IsDefined returns false if undefined result value of metric
func (av AttackVector) IsDefined() bool {
	return !av.IsUnknown() && av != AttackVectorNotDefined
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
