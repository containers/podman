package metric

import "math"

func roundUp(input float64) float64 {
	intInput := math.Round(input * 100000)

	if int(intInput)%10000 == 0 {
		return intInput / 100000
	}

	return (math.Floor(intInput/10000) + 1) / 10.0
}

//GetSeverity returns severity by score of Base metrics
func severity(score float64) Severity {
	switch true {
	case score <= 0:
		return SeverityNone
	case score > 0 && score < 4.0:
		return SeverityLow
	case score >= 4.0 && score < 7.0:
		return SeverityMedium
	case score >= 7.0 && score < 9.0:
		return SeverityHigh
	case score >= 9.0:
		return SeverityCritical
	default:
		return SeverityUnknown
	}
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
