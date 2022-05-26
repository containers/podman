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

package release

import (
	"crypto/sha1" //nolint:gosec // used for file integrity checks, NOT security
	"fmt"
	"os"
	"path/filepath"
	"strings"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"sigs.k8s.io/bom/pkg/provenance"
	"sigs.k8s.io/bom/pkg/spdx"
	"sigs.k8s.io/release-sdk/object"
	"sigs.k8s.io/release-utils/util"
)

func NewProvenanceChecker(opts *ProvenanceCheckerOptions) *ProvenanceChecker {
	p := &ProvenanceChecker{
		objStore: object.NewGCS(),
		options:  opts,
	}
	p.objStore.WithConcurrent(true)
	p.objStore.WithRecursive(true)
	p.impl = &defaultProvenanceCheckerImpl{}
	return p
}

// ProvenanceChecker
type ProvenanceChecker struct {
	objStore *object.GCS
	options  *ProvenanceCheckerOptions
	impl     provenanceCheckerImplementation
}

// CheckStageProvenance
func (pc *ProvenanceChecker) CheckStageProvenance(buildVersion string) error {
	//nolint:gosec // used for file integrity checks, NOT security
	// Init the local dir
	h := sha1.New()
	if _, err := h.Write([]byte(buildVersion)); err != nil {
		return errors.Wrap(err, "creating dir")
	}
	pc.options.StageDirectory = filepath.Join(pc.options.ScratchDirectory, fmt.Sprintf("%x", h.Sum(nil)))

	gcsPath, err := pc.objStore.NormalizePath(
		object.GcsPrefix + filepath.Join(
			pc.options.StageBucket, StagePath, buildVersion,
		) + string(filepath.Separator),
	)
	if err != nil {
		return errors.Wrap(err, "normalizing GCS stage path")
	}
	// Download all the artifacts from the bucket
	if err := pc.impl.downloadStagedArtifacts(pc.options, pc.objStore, gcsPath); err != nil {
		return errors.Wrap(err, "downloading staged artifacts")
	}

	// Preprocess the attestation file. We have to rewrite the paths
	// to strip the GCS prefix
	statement, err := pc.impl.processAttestation(pc.options, buildVersion)
	if err != nil {
		return errors.Wrap(err, "processing provenance attestation")
	}

	// Run the check of the artifacts
	if err := pc.impl.checkProvenance(pc.options, statement); err != nil {
		return errors.Wrap(err, "verifying provenance of staged artifacts")
	}
	logrus.Infof(
		"Successfully verified provenance information of %d staged artifacts",
		len(statement.Subject),
	)
	return nil
}

// GenerateFinalAttestation combines the stage provenance attestation
// with a release sbom to create the end-user provenance atteatation
func (pc *ProvenanceChecker) GenerateFinalAttestation(buildVersion string, versions *Versions) error {
	statementPath := filepath.Join(pc.options.StageDirectory, buildVersion, ProvenanceFilename)
	for _, version := range versions.Ordered() {
		if err := pc.impl.generateFinalAttestation(
			pc.options,
			filepath.Join(
				pc.options.StageDirectory, buildVersion, version, GCSStagePath, version, "kubernetes-release.spdx",
			),
			statementPath, version,
		); err != nil {
			return errors.Wrapf(err, "generating provenance data for %s", version)
		}
	}
	return nil
}

type ProvenanceCheckerOptions struct {
	StageBucket      string // Bucket where the artifacts are stored
	StageDirectory   string // Directory where artifacts will be downloaded
	ScratchDirectory string // Directory where StageDirectory will be created
}

type provenanceCheckerImplementation interface {
	downloadStagedArtifacts(*ProvenanceCheckerOptions, *object.GCS, string) error
	processAttestation(*ProvenanceCheckerOptions, string) (*provenance.Statement, error)
	checkProvenance(*ProvenanceCheckerOptions, *provenance.Statement) error
	generateFinalAttestation(opts *ProvenanceCheckerOptions, sbom, stageProvenance, version string) error
}

type defaultProvenanceCheckerImpl struct{}

// downloadReleaseArtifacts sybc
func (di *defaultProvenanceCheckerImpl) downloadStagedArtifacts(
	opts *ProvenanceCheckerOptions, objStore *object.GCS, path string,
) error {
	logrus.Infof("Synching stage from %s to %s", path, opts.StageDirectory)
	if !util.Exists(opts.StageDirectory) {
		if err := os.MkdirAll(opts.StageDirectory, os.FileMode(0o755)); err != nil {
			return errors.Wrap(err, "creating local working directory")
		}
	}
	return errors.Wrap(
		objStore.CopyToLocal(path, opts.StageDirectory),
		"synching staged sources",
	)
}

// processAttestation
func (di *defaultProvenanceCheckerImpl) processAttestation(
	opts *ProvenanceCheckerOptions, buildVersion string) (s *provenance.Statement, err error) {
	// Load the downloaded statement
	s, err = provenance.LoadStatement(filepath.Join(opts.StageDirectory, buildVersion, ProvenanceFilename))
	if err != nil {
		return nil, errors.Wrap(err, "loading staging provenance file")
	}

	// We've downloaded all artifacts, so to check we need to strip
	// the gcs bucket prefix from the subjects to read from the local copy
	gcsPath := object.GcsPrefix + filepath.Join(opts.StageBucket, StagePath)

	newSubjects := []intoto.Subject{}

	for i, sub := range s.Subject {
		newSubjects = append(newSubjects, intoto.Subject{
			Name:   strings.TrimPrefix(sub.Name, gcsPath),
			Digest: sub.Digest,
		})
		s.Subject[i].Name = strings.TrimPrefix(sub.Name, gcsPath)
	}
	s.Subject = newSubjects
	return s, nil
}

func (di *defaultProvenanceCheckerImpl) checkProvenance(
	opts *ProvenanceCheckerOptions, s *provenance.Statement) error {
	return errors.Wrap(s.VerifySubjects(opts.StageDirectory), "checking subjects in attestation")
}

func (di *defaultProvenanceCheckerImpl) generateFinalAttestation(
	opts *ProvenanceCheckerOptions, sbom, stageProvenance, version string) error {
	doc, err := spdx.OpenDoc(sbom)
	if err != nil {
		return errors.Wrapf(err, "parsing sbom for version %s from %s", version, sbom)
	}

	slsaStatement := doc.ToProvenanceStatement(spdx.DefaultProvenanceOptions)

	// Rewrite the provenance sublects to list their full paths in the bucket
	for i, sub := range slsaStatement.Subject {
		slsaStatement.Subject[i].Name = object.GcsPrefix + filepath.Join(
			opts.StageBucket, "release", version, sub.Name,
		)
	}
	if err := slsaStatement.ClonePredicate(stageProvenance); err != nil {
		return errors.Wrapf(
			err, "cloning SLSA predicate from staging provenance: %s", stageProvenance,
		)
	}
	if err := slsaStatement.Write(
		filepath.Join(os.TempDir(), fmt.Sprintf("provenance-%s.json", version)),
	); err != nil {
		return errors.Wrapf(err, "writing final provenance attestation for %s", version)
	}

	return nil
}

func NewProvenanceReader(opts *ProvenanceReaderOptions) *ProvenanceReader {
	return &ProvenanceReader{
		options: opts,
		impl:    &defaultProvenanceReaderImpl{},
	}
}

type ProvenanceReader struct {
	options *ProvenanceReaderOptions
	impl    provenanceReaderImplementation
}

type provenanceReaderImplementation interface {
	GetStagingSubjects(*ProvenanceReaderOptions, string) ([]intoto.Subject, error)
	GetBuildSubjects(*ProvenanceReaderOptions, string, string) ([]intoto.Subject, error)
}

type ProvenanceReaderOptions struct {
	Bucket       string
	BuildVersion string
	WorkspaceDir string
}

// GetBuildSubjects returns all artifacts in the output directory
// as intoto subjects, ready to add to the attestation
func (pr *ProvenanceReader) GetBuildSubjects(path, version string) ([]intoto.Subject, error) {
	return pr.impl.GetBuildSubjects(pr.options, path, version)
}

// GetStagingSubjects reads artifacts from the GCB workspace and returns them
// as in-toto subjects, with their paths normalized to their final locations
// in the staging bucket.
func (pr *ProvenanceReader) GetStagingSubjects(path string) ([]intoto.Subject, error) {
	return pr.impl.GetStagingSubjects(pr.options, path)
}

type defaultProvenanceReaderImpl struct{}

func (di *defaultProvenanceReaderImpl) GetStagingSubjects(
	opts *ProvenanceReaderOptions, path string) ([]intoto.Subject, error) {
	// Create the dummy statement to read artifacts
	dummy := provenance.NewSLSAStatement()

	// The path in the bucket were built artifacts will be staged
	gcsPath := filepath.Join(opts.Bucket, StagePath, opts.BuildVersion)

	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.Wrap(err, "checking artifact path to generate provenance subjects")
	}

	if info.IsDir() {
		if err := dummy.ReadSubjectsFromDir(path); err != nil {
			return nil, errors.Wrapf(err, "generating provenance subject from file %s", path)
		}
	} else {
		if err := dummy.AddSubjectFromFile(path); err != nil {
			return nil, errors.Wrapf(err, "generating provenance subject from file %s", path)
		}
	}

	// Check if we are dealing with the sources tar and translate to the top
	if dummy.Subject[0].Name == filepath.Join(opts.WorkspaceDir, SourcesTar) {
		dummy.Subject[0].Name = SourcesTar
	}

	for i, s := range dummy.Subject {
		dummy.Subject[i].Name = object.GcsPrefix + filepath.Join(gcsPath, s.Name)
	}

	return dummy.Subject, nil
}

func (di *defaultProvenanceReaderImpl) GetBuildSubjects(
	opts *ProvenanceReaderOptions, path, version string) ([]intoto.Subject, error) {
	// The path in the bucket were built artifacts will be staged
	gcsPath := filepath.Join(opts.Bucket, StagePath, opts.BuildVersion)

	// When adding the output directory for a specific version, we need
	// to modiy the paths in the attestation to match the bucket names.
	// In order to do that, we create a dummy statement. Use that to read
	// the files and translate those to the final attestation with the paths
	// translated.
	dummy := provenance.NewSLSAStatement()
	if err := dummy.ReadSubjectsFromDir(path); err != nil {
		return nil, errors.Wrap(err, "reading output directory provenance subjects")
	}

	// Cycle the subjects, translate the paths and copy them to the
	// real attestation:
	newSubjects := []intoto.Subject{}
	for _, subject := range dummy.Subject {
		// If the artifact is not in the images or gcs-stage dir, skip
		if !strings.HasPrefix(subject.Name, ImagesPath) &&
			!strings.HasPrefix(subject.Name, GCSStagePath) {
			continue
		}

		// Now the tricky part. We need to re-append the version tag. Eg
		// gcs-stage/v1.23.0-alpha.4/file.txt shoud be
		// v1.23.0-alpha.4/gcs-stage/v1.23.0-alpha.4/file.txt shoud be
		subject.Name = object.GcsPrefix + filepath.Join(gcsPath, version, subject.Name)

		newSubjects = append(newSubjects, subject)
	}
	return newSubjects, nil
}
