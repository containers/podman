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

package provenance

import (
	"encoding/json"
	"os"

	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	"github.com/pkg/errors"
)

type Predicate struct {
	slsa.ProvenancePredicate
	impl PredicateImplementation
}

// setImplementation sets the predicate implementation
func (p *Predicate) SetImplementation(impl PredicateImplementation) {
	p.impl = impl
}

// AddMaterial adds an entry to the listo of materials
func (p *Predicate) AddMaterial(uri string, ds slsa.DigestSet) {
	p.impl.AddMaterial(p, uri, ds)
}

// Write outputs the predicate as JSON to a file
func (p *Predicate) Write(path string) error {
	return p.impl.Write(p, path)
}

//counterfeiter:generate . PredicateImplementation
type PredicateImplementation interface {
	AddMaterial(*Predicate, string, slsa.DigestSet)
	Write(*Predicate, string) error
}

type defaultPredicateImplementation struct{}

// Write dumps the predicate data into a JSON file
func (pi *defaultPredicateImplementation) Write(p *Predicate, path string) error {
	jsonData, err := json.Marshal(p)
	if err != nil {
		return errors.Wrap(err, "marshalling predicate to json")
	}
	return errors.Wrap(
		os.WriteFile(path, jsonData, os.FileMode(0o644)),
		"writing predicate file",
	)
}

// AddMaterial adds a material to the entry
func (pi *defaultPredicateImplementation) AddMaterial(p *Predicate, uri string, ds slsa.DigestSet) {
	if p.Materials == nil {
		p.Materials = []slsa.ProvenanceMaterial{}
	}
	p.Materials = append(p.Materials, slsa.ProvenanceMaterial{
		URI:    uri,
		Digest: ds,
	})
}
