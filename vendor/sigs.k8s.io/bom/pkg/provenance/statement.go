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

/*
	Useful links:
	SLSA Provenance predicate spec: https://slsa.dev/provenance/v0.1
	In-toto attestation spec: https://github.com/in-toto/attestation/blob/main/spec/README.md
*/

package provenance

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/hash"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
)

// Statement is the middle layer of the attestation, binding it to a particular subject and
// unambiguously identifying the types of the predicate.
// https://github.com/in-toto/attestation/blob/main/spec/README.md#statement
type Statement struct {
	intoto.StatementHeader
	Predicate Predicate `json:"predicate"`
	impl      StatementImplementation
}

func (s *Statement) SetImplementation(si StatementImplementation) {
	s.impl = si
}

// Write outputs the predicate as JSON to a file
func (s *Statement) Write(path string) error {
	return s.impl.Write(s, path)
}

func (s *Statement) ToJSON() ([]byte, error) {
	return s.impl.ToJSON(s)
}

// ReadSubjectsFromDir reads a directory and adds every file as a subject
// to the statement.
func (s *Statement) ReadSubjectsFromDir(path string) (err error) {
	return s.impl.ReadSubjectsFromDir(s, path)
}

// AddSubject adds an entry to the listo of materials
func (s *Statement) AddSubject(uri string, ds slsa.DigestSet) {
	s.impl.AddSubject(s, uri, ds)
}

// AddSubjectFromFile adds a subject to the list by checking a file in the filesystem
func (s *Statement) AddSubjectFromFile(filePath string) error {
	subject, err := s.impl.SubjectFromFile(filePath)
	if err != nil {
		return errors.Wrapf(err, "creating subject from file %s", filePath)
	}
	s.impl.AddSubject(s, subject.Name, subject.Digest)
	return nil
}

// LoadPredicate loads a predicate from a json file
func (s *Statement) LoadPredicate(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "opening predicate file")
	}
	p := Predicate{}
	if err := json.Unmarshal(data, &p); err != nil {
		return errors.Wrap(err, "unmarshalling predicate json")
	}
	s.Predicate = p
	return nil
}

// ClonePredicate reads a provenance metadata file from `manifestPath` and
// clones the predicate data to the current object.
func (s *Statement) ClonePredicate(manifestPath string) error {
	return s.impl.ClonePredicate(s, manifestPath)
}

// VerifySubjects checks the provenance metadata of the attestation
// subjects by reading them from `path`.
func (s *Statement) VerifySubjects(path string) (err error) {
	return s.impl.VerifySubjects(path, &s.Subject)
}

//counterfeiter:generate . StatementImplementation
type StatementImplementation interface {
	AddSubject(*Statement, string, slsa.DigestSet)
	ReadSubjectsFromDir(*Statement, string) error
	SubjectFromFile(string) (intoto.Subject, error)
	Write(*Statement, string) error
	ToJSON(s *Statement) ([]byte, error)
	ClonePredicate(*Statement, string) error
	VerifySubjects(path string, subjects *[]intoto.Subject) (err error)
}

type defaultStatementImplementation struct{}

// AddSubject adds a material to the entry
func (si *defaultStatementImplementation) AddSubject(
	s *Statement, name string, ds slsa.DigestSet,
) {
	if s.Subject == nil {
		s.Subject = []intoto.Subject{}
	}
	s.Subject = append(s.Subject, intoto.Subject{
		Name:   name,
		Digest: ds,
	})
}

// ReadSubjectsFromDir reads a directory and adds all files found as
// subjects of the statement.
func (si *defaultStatementImplementation) ReadSubjectsFromDir(
	s *Statement, dirPath string) (err error) {
	// Traverse the directory
	if err := fs.WalkDir(os.DirFS(dirPath), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if d.Type() == os.ModeSymlink {
			return nil
		}
		hashVal, err := hash.SHA256ForFile(filepath.Join(dirPath, path))
		if err != nil {
			return errors.Wrapf(err, "hashing file %s", path)
		}
		s.AddSubject(path, slsa.DigestSet{"sha256": hashVal})
		return nil
	}); err != nil {
		return errors.Wrap(err, "buiding directory tree")
	}
	return nil
}

// SubjectFromFile reads a file and return an in-toto subject describing it
func (si *defaultStatementImplementation) SubjectFromFile(filePath string) (subject intoto.Subject, err error) {
	subject = intoto.Subject{
		Name:   filePath,
		Digest: map[string]string{},
	}
	h256, err := hash.SHA256ForFile(filePath)
	if err != nil {
		return subject, errors.Wrapf(err, "getting sha256 for file %s", filePath)
	}
	h512, err := hash.SHA512ForFile(filePath)
	if err != nil {
		return subject, errors.Wrapf(err, "getting sha512 for %s", filePath)
	}
	subject.Digest = map[string]string{
		"sha256": h256,
		"sha512": h512,
	}
	return subject, nil
}

func (si *defaultStatementImplementation) ToJSON(s *Statement) ([]byte, error) {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling statement to json")
	}
	return jsonData, nil
}

// Write dumps the statement data to disk in json
func (si *defaultStatementImplementation) Write(s *Statement, path string) error {
	jsonData, err := si.ToJSON(s)
	if err != nil {
		return err
	}
	return errors.Wrap(
		os.WriteFile(path, jsonData, os.FileMode(0o644)),
		"writing predicate file",
	)
}

// ClonePredicate clones the predicate from the file in manifestPath
// to into the Statement
func (si *defaultStatementImplementation) ClonePredicate(s *Statement, manifestPath string) error {
	otherStatment, err := LoadStatement(manifestPath)
	if err != nil {
		return errors.Wrap(err, "loading other manifest to clone data")
	}

	s.Predicate = otherStatment.Predicate
	return nil
}

// VerifySubjects checks the subjects registered in the manifest to make
// sure the attesttion data matches the artifacts
func (si *defaultStatementImplementation) VerifySubjects(path string, subjects *[]intoto.Subject) (err error) {
	errs := 0
	for _, sub := range *subjects {
		computed := ""
		if sub.Name == "" {
			logrus.Error("found empty subject in provenance data")
			errs++
			continue
		}
		if len(sub.Digest) == 0 {
			logrus.Errorf("%s has no hash information", sub.Name)
			errs++
			continue
		}
		for algo, val := range sub.Digest {
			computed = ""

			switch algo {
			case "sha256":
				computed, err = hash.SHA256ForFile(filepath.Join(path, sub.Name))
			case "sha512":
				computed, err = hash.SHA512ForFile(filepath.Join(path, sub.Name))
			}

			if err != nil {
				logrus.Errorf("Error validating %s: %v", sub.Name, err)
				errs++
				continue
			}

			if val != computed {
				errs++
				logrus.Errorf("Invalid hash in %s", sub.Name)
			}
		}
	}
	if errs > 0 {
		return errors.Errorf("%d errors validating subjects in provenance metadata", errs)
	}
	return nil
}
