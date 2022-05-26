/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

package provenance

import (
	"encoding/json"
	"os"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	"github.com/pkg/errors"
)

// LoadStatement loads a statement from a json file
func LoadStatement(path string) (s *Statement, err error) {
	statement := NewSLSAStatement()

	jsonData, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "opening stament JSON file")
	}

	if err := json.Unmarshal(jsonData, &statement); err != nil {
		return nil, errors.Wrap(err, "decoding attestation JSON data")
	}

	return statement, nil
}

// NewSLSAStatement creates a new attestation
func NewSLSAStatement() *Statement {
	return &Statement{

		StatementHeader: intoto.StatementHeader{
			Type:          intoto.StatementInTotoV01,
			PredicateType: slsa.PredicateSLSAProvenance,
			Subject:       []intoto.Subject{},
		},
		Predicate: NewSLSAPredicate(),

		impl: &defaultStatementImplementation{},
	}
}

// NewSLSAPredicate returns a new SLSA provenance predicate
func NewSLSAPredicate() Predicate {
	return Predicate{
		ProvenancePredicate: slsa.ProvenancePredicate{
			Builder: slsa.ProvenanceBuilder{
				ID: "",
			},
			Invocation: slsa.ProvenanceInvocation{
				ConfigSource: slsa.ConfigSource{
					Digest: map[string]string{},
				},
				Parameters:  nil,
				Environment: nil,
			},
			Metadata: &slsa.ProvenanceMetadata{
				Completeness: slsa.ProvenanceComplete{},
			},
			Materials: []slsa.ProvenanceMaterial{},
		},
		impl: &defaultPredicateImplementation{},
	}
}

// Envelope is the outermost layer of the attestation, handling authentication and
// serialization. The format and protocol are defined in DSSE and adopted by in-toto in ITE-5.
// https://github.com/in-toto/attestation/blob/main/spec/README.md#envelope
type Envelope struct {
	PayloadType string        `json:"payloadType"`
	Payload     string        `json:"payload"`
	Signatures  []interface{} `json:"signatures"`
}
