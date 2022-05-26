package metric

import "strings"

//RemediationLevel is metric type for Temporal Metrics
type RemediationLevel int

//Constant of RemediationLevel result
const (
	RemediationLevelNotDefined RemediationLevel = iota
	RemediationLevelOfficialFix
	RemediationLevelTemporaryFix
	RemediationLevelWorkaround
	RemediationLevelUnavailable
)

var remediationLevelMap = map[RemediationLevel]string{
	RemediationLevelNotDefined:   "X",
	RemediationLevelOfficialFix:  "O",
	RemediationLevelTemporaryFix: "T",
	RemediationLevelWorkaround:   "W",
	RemediationLevelUnavailable:  "U",
}

var remediationLevelValueMap = map[RemediationLevel]float64{
	RemediationLevelNotDefined:   1,
	RemediationLevelOfficialFix:  0.95,
	RemediationLevelTemporaryFix: 0.96,
	RemediationLevelWorkaround:   0.97,
	RemediationLevelUnavailable:  1,
}

//GetRemediationLevel returns result of RemediationLevel metric
func GetRemediationLevel(s string) RemediationLevel {
	s = strings.ToUpper(s)
	for k, v := range remediationLevelMap {
		if s == v {
			return k
		}
	}
	return RemediationLevelNotDefined
}

func (rl RemediationLevel) String() string {
	if s, ok := remediationLevelMap[rl]; ok {
		return s
	}
	return ""
}

//Value returns value of RemediationLevel metric
func (rl RemediationLevel) Value() float64 {
	if v, ok := remediationLevelValueMap[rl]; ok {
		return v
	}
	return 1
}

//IsDefined returns false if undefined result value of metric
func (rl RemediationLevel) IsDefined() bool {
	_, ok := remediationLevelValueMap[rl]
	return ok
}

/* Copyright by Florent Viel, 2020 */
/* Contributed by Spiegel, 2020 */
