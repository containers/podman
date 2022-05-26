package metric

import (
	"math"
	"strings"

	"github.com/spiegel-im-spiegel/errs"
	"github.com/spiegel-im-spiegel/go-cvss/cvsserr"
)

//Base is Base Metrics for CVSSv3
type Base struct {
	Ver Version
	AV  AttackVector
	AC  AttackComplexity
	PR  PrivilegesRequired
	UI  UserInteraction
	S   Scope
	C   ConfidentialityImpact
	I   IntegrityImpact
	A   AvailabilityImpact
}

//NewBase returns Base Metrics instance
func NewBase() *Base {
	return &Base{
		Ver: VUnknown,
		AV:  AttackVectorUnknown,
		AC:  AttackComplexityUnknown,
		PR:  PrivilegesRequiredUnknown,
		UI:  UserInteractionUnknown,
		S:   ScopeUnknown,
		C:   ConfidentialityImpactUnknown,
		I:   IntegrityImpactUnknown,
		A:   AvailabilityImpactUnknown,
	}
}

func (bm *Base) Decode(vector string) (*Base, error) {
	if bm == nil {
		bm = NewBase()
	}
	values := strings.Split(vector, "/")
	if len(values) < 9 {
		return bm, errs.Wrap(cvsserr.ErrInvalidVector, errs.WithContext("vector", vector))
	}
	//CVSS version
	ver, err := GetVersion(values[0])
	if err != nil {
		return bm, errs.Wrap(err, errs.WithContext("vector", vector))
	}
	if ver == VUnknown {
		return bm, errs.Wrap(cvsserr.ErrNotSupportVer, errs.WithContext("vector", vector))
	}
	bm.Ver = ver
	//parse vector
	var lastErr error
	for _, value := range values[1:] {
		if err := bm.decodeOne(value); err != nil {
			if !errs.Is(err, cvsserr.ErrNotSupportMetric) {
				return bm, errs.Wrap(err, errs.WithContext("vector", vector))
			}
			lastErr = err
		}
	}
	if lastErr != nil {
		return bm, lastErr
	}
	return bm, bm.GetError()
}
func (bm *Base) decodeOne(str string) error {
	m := strings.Split(str, ":")
	if len(m) != 2 {
		return errs.Wrap(cvsserr.ErrInvalidVector, errs.WithContext("metric", str))
	}
	switch strings.ToUpper(m[0]) {
	case "AV": //Attack Vector
		bm.AV = GetAttackVector(m[1])
	case "AC": //Attack Complexity
		bm.AC = GetAttackComplexity(m[1])
	case "PR": //Privileges Required
		bm.PR = GetPrivilegesRequired(m[1])
	case "UI": //User Interaction
		bm.UI = GetUserInteraction(m[1])
	case "S": //Scope
		bm.S = GetScope(m[1])
	case "C": //Confidentiality Impact
		bm.C = GetConfidentialityImpact(m[1])
	case "I": //Integrity Impact
		bm.I = GetIntegrityImpact(m[1])
	case "A": //Availability Impact
		bm.A = GetAvailabilityImpact(m[1])
	default:
		return errs.Wrap(cvsserr.ErrNotSupportMetric, errs.WithContext("metric", str))
	}
	return nil
}

//GetError returns error instance if undefined metric
func (bm *Base) GetError() error {
	if bm == nil {
		return errs.Wrap(cvsserr.ErrUndefinedMetric)
	}
	switch true {
	case bm.Ver == VUnknown, !bm.AV.IsDefined(), !bm.AC.IsDefined(), !bm.PR.IsDefined(), !bm.UI.IsDefined(), !bm.S.IsDefined(), !bm.C.IsDefined(), !bm.I.IsDefined(), !bm.A.IsDefined():
		return errs.Wrap(cvsserr.ErrUndefinedMetric)
	default:
		return nil
	}
}

//Encode returns CVSSv3 vector string
func (bm *Base) Encode() (string, error) {
	if err := bm.GetError(); err != nil {
		return "", err
	}
	r := &strings.Builder{}
	r.WriteString("CVSS:" + bm.Ver.String()) //CVSS Version
	r.WriteString("/AV:" + bm.AV.String())   //Attack Vector
	r.WriteString("/AC:" + bm.AC.String())   //Attack Complexity
	r.WriteString("/PR:" + bm.PR.String())   //Privileges Required
	r.WriteString("/UI:" + bm.UI.String())   //User Interaction
	r.WriteString("/S:" + bm.S.String())     //Scope
	r.WriteString("/C:" + bm.C.String())     //Confidentiality Impact
	r.WriteString("/I:" + bm.I.String())     //Integrity Impact
	r.WriteString("/A:" + bm.A.String())     //Availability Impact
	return r.String(), nil
}

//Score returns score of Base metrics
func (bm *Base) Score() float64 {
	if err := bm.GetError(); err != nil {
		return 0.0
	}

	impact := 1.0 - (1-bm.C.Value())*(1-bm.I.Value())*(1-bm.A.Value())
	if bm.S == ScopeUnchanged {
		impact *= 6.42
	} else {
		impact = 7.52*(impact-0.029) - 3.25*math.Pow(impact-0.02, 15.0)
	}
	ease := 8.22 * bm.AV.Value() * bm.AC.Value() * bm.PR.Value(bm.S) * bm.UI.Value()

	var score float64
	if impact <= 0 {
		score = 0.0
	} else if bm.S == ScopeUnchanged {
		score = roundUp(math.Min(impact+ease, 10))
	} else {
		score = roundUp(math.Min(1.08*(impact+ease), 10))
	}
	return score
}

//Severity returns severity by score of Base metrics
func (bm *Base) Severity() Severity {
	return severity(bm.Score())
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
