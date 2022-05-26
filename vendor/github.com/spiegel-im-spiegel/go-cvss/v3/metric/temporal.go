package metric

import (
	"strings"

	"github.com/spiegel-im-spiegel/errs"
	"github.com/spiegel-im-spiegel/go-cvss/cvsserr"
)

//Base is Temporal Metrics for CVSSv3
type Temporal struct {
	*Base
	E  Exploitability
	RL RemediationLevel
	RC ReportConfidence
}

//NewBase returns Base Metrics instance
func NewTemporal() *Temporal {
	return &Temporal{
		Base: NewBase(),
		E:    ExploitabilityNotDefined,
		RL:   RemediationLevelNotDefined,
		RC:   ReportConfidenceNotDefined,
	}
}

func (tm *Temporal) Decode(vector string) (*Temporal, error) {
	if tm == nil {
		tm = NewTemporal()
	}
	values := strings.Split(vector, "/")
	if len(values) < 9 { // E, RL, RC metrics are optional.
		return tm, errs.Wrap(cvsserr.ErrInvalidVector, errs.WithContext("vector", vector))
	}
	//CVSS version
	ver, err := GetVersion(values[0])
	if err != nil {
		return tm, errs.Wrap(err, errs.WithContext("vector", vector))
	}
	if ver == VUnknown {
		return tm, errs.Wrap(cvsserr.ErrNotSupportVer, errs.WithContext("vector", vector))
	}
	tm.Ver = ver
	//parse vector
	var lastErr error
	for _, value := range values[1:] {
		if err := tm.decodeOne(value); err != nil {
			if !errs.Is(err, cvsserr.ErrNotSupportMetric) {
				return tm, errs.Wrap(err, errs.WithContext("vector", vector))
			}
			lastErr = err
		}
	}
	if lastErr != nil {
		return tm, lastErr
	}
	return tm, tm.GetError()
}
func (tm *Temporal) decodeOne(str string) error {
	if err := tm.Base.decodeOne(str); err != nil {
		if !errs.Is(err, cvsserr.ErrNotSupportMetric) {
			return errs.Wrap(err, errs.WithContext("metric", str))
		}
	} else {
		return nil
	}
	m := strings.Split(str, ":")
	if len(m) != 2 {
		return errs.Wrap(cvsserr.ErrInvalidVector, errs.WithContext("metric", str))
	}
	switch strings.ToUpper(m[0]) {
	case "E": //Exploitability
		tm.E = GetExploitability(m[1])
	case "RL": //RemediationLevel
		tm.RL = GetRemediationLevel(m[1])
	case "RC": //RemediationLevel
		tm.RC = GetReportConfidence(m[1])
	default:
		return errs.Wrap(cvsserr.ErrNotSupportMetric, errs.WithContext("metric", str))
	}
	return nil
}

//GetError returns error instance if undefined metric
func (tm *Temporal) GetError() error {
	if tm == nil {
		return errs.Wrap(cvsserr.ErrUndefinedMetric)
	}
	if err := tm.Base.GetError(); err != nil {
		return errs.Wrap(err)
	}
	switch true {
	case !tm.E.IsDefined(), !tm.RL.IsDefined(), !tm.RC.IsDefined():
		return errs.Wrap(cvsserr.ErrUndefinedMetric)
	default:
		return nil
	}
}

//Encode returns CVSSv3 vector string
func (tm *Temporal) Encode() (string, error) {
	if err := tm.GetError(); err != nil {
		return "", errs.Wrap(err)
	}
	bs, err := tm.Base.Encode()
	if err != nil {
		return "", errs.Wrap(err)
	}
	r := &strings.Builder{}
	r.WriteString(bs)                      //Vector of Base metrics
	r.WriteString("/E:" + tm.E.String())   //Exploitability
	r.WriteString("/RL:" + tm.RL.String()) //Remediation Level
	r.WriteString("/RC:" + tm.RC.String()) //Report Confidence
	return r.String(), nil
}

//Score returns score of Temporal metrics
func (tm *Temporal) Score() float64 {
	if err := tm.GetError(); err != nil {
		return 0.0
	}
	return roundUp(tm.Base.Score() * tm.E.Value() * tm.RL.Value() * tm.RC.Value())
}

//Severity returns severity by score of Temporal metrics
func (tm *Temporal) Severity() Severity {
	return severity(tm.Score())
}

//BaseMetrics returns Base metrics in Temporal metrics instance
func (tm *Temporal) BaseMetrics() *Base {
	if tm == nil {
		return nil
	}
	return tm.Base
}

/* Copyright by Florent Viel, 2020 */
/* Contributed by Spiegel, 2020 */
