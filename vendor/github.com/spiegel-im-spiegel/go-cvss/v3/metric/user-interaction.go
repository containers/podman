package metric

import "strings"

//UserInteraction is metric type for Base Metrics
type UserInteraction int

//Constant of UserInteraction result
const (
	UserInteractionUnknown UserInteraction = iota
	UserInteractionNotDefined
	UserInteractionRequired
	UserInteractionNone
)

var userInteractionMap = map[UserInteraction]string{
	UserInteractionNotDefined: "X",
	UserInteractionRequired:   "R",
	UserInteractionNone:       "N",
}

var userInteractionValueMap = map[UserInteraction]float64{
	UserInteractionNotDefined: 0,
	UserInteractionRequired:   0.62,
	UserInteractionNone:       0.85,
}

//GetUserInteraction returns result of UserInteraction metric
func GetUserInteraction(s string) UserInteraction {
	s = strings.ToUpper(s)
	for k, v := range userInteractionMap {
		if s == v {
			return k
		}
	}
	return UserInteractionUnknown
}

func (ui UserInteraction) String() string {
	if s, ok := userInteractionMap[ui]; ok {
		return s
	}
	return ""
}

//Value returns value of UserInteraction metric
func (ui UserInteraction) Value() float64 {
	if v, ok := userInteractionValueMap[ui]; ok {
		return v
	}
	return 0.0
}

//IsUnknown returns false if undefined result value of metric
func (ui UserInteraction) IsUnknown() bool {
	return ui == UserInteractionUnknown
}

//IsDefined returns false if undefined result value of metric
func (ui UserInteraction) IsDefined() bool {
	return !ui.IsUnknown() && ui != UserInteractionNotDefined
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
