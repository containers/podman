/*
Copyright 2017 The Kubernetes Authors.

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

package notes

import (
	"io"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"sigs.k8s.io/release-sdk/object"
)

// MapProvider interface that obtains release notes maps from a source
type MapProvider interface {
	GetMapsForPR(int) ([]*ReleaseNotesMap, error)
}

// NewProviderFromInitString creates a new map provider from an initialization string
func NewProviderFromInitString(initString string) (MapProvider, error) {
	// If init string starts with gs:// return a CloudStorageProvider
	if initString[0:5] == object.GcsPrefix {
		// Currently for illustration purposes
		return nil, errors.New("CloudStorageProvider is not yet implemented")
	}

	// Otherwise, build a DirectoryMapProvider using the
	// whole init string as the path
	fileStat, err := os.Stat(initString)
	if os.IsNotExist(err) {
		return nil, errors.New("release notes map path does not exist")
	}
	if !fileStat.IsDir() {
		return nil, errors.New("release notes map path is not a directory")
	}

	return &DirectoryMapProvider{Path: initString}, nil
}

// ParseReleaseNotesMap Parses a Release Notes Map
func ParseReleaseNotesMap(mapPath string) (*[]ReleaseNotesMap, error) {
	notemaps := []ReleaseNotesMap{}
	yamlReader, err := os.Open(mapPath)
	if err != nil {
		return nil, errors.Wrap(err, "opening maps")
	}
	defer yamlReader.Close()

	decoder := yaml.NewDecoder(yamlReader)

	for {
		noteMap := ReleaseNotesMap{}
		if err := decoder.Decode(&noteMap); err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.Wrap(err, "decoding note map")
		}
		notemaps = append(notemaps, noteMap)
	}

	return &notemaps, nil
}

// ReleaseNotesMap holds the changes that will be applied to the notes
// the struct has double annotation because of this: https://play.golang.org/p/C7QM1varozy
type ReleaseNotesMap struct {
	// Pull request where the note was published
	PR int `json:"pr"`
	// SHA of the notes commit
	Commit      string `json:"commit,omitempty" yaml:"commit,omitempty"`
	ReleaseNote struct {
		// Text is the actual content of the release note
		Text *string `json:"text,omitempty" yaml:"text,omitempty"`

		// Docs is additional documentation for the release note
		Documentation *[]*Documentation `json:"documentation,omitempty" yaml:"documentation,omitempty"`

		// Author is the GitHub username of the commit author
		Author *string `json:"author,omitempty" yaml:"author,omitempty"`

		// Areas is a list of the labels beginning with area/
		Areas *[]string `json:"areas,omitempty" yaml:"areas,omitempty"`

		// Kinds is a list of the labels beginning with kind/
		Kinds *[]string `json:"kinds,omitempty" yaml:"kinds,omitempty"`

		// SIGs is a list of the labels beginning with sig/
		SIGs *[]string `json:"sigs,omitempty" yaml:"sigs,omitempty"`

		// Indicates whether or not a note will appear as a new feature
		Feature *bool `json:"feature,omitempty" yaml:"feature,omitempty"`

		// ActionRequired indicates whether or not the release-note-action-required
		// label was set on the PR
		ActionRequired *bool `json:"action_required,omitempty" yaml:"action_required,omitempty"`

		// DoNotPublish by default represents release-note-none label on GitHub
		DoNotPublish *bool `json:"do_not_publish,omitempty" yaml:"do_not_publish,omitempty"`
	} `json:"releasenote"`

	DataFields map[string]ReleaseNotesDataField `json:"datafields,omitempty" yaml:"datafields,omitempty"`
}

// ReleaseNotesDataField extra data added to a release note
type ReleaseNotesDataField interface{}

// DirectoryMapProvider is a provider that gets maps from a directory
type DirectoryMapProvider struct {
	Path string
	Maps map[int][]*ReleaseNotesMap
}

// readMaps Open the dir and read dir notes
func (mp *DirectoryMapProvider) readMaps() error {
	var fileList []string
	mp.Maps = map[int][]*ReleaseNotesMap{}

	err := filepath.Walk(mp.Path, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
			fileList = append(fileList, path)
		}
		return nil
	})

	for _, fileName := range fileList {
		notemaps, err := ParseReleaseNotesMap(fileName)
		if err != nil {
			return errors.Wrapf(err, "while parsing note map in %s", fileName)
		}
		for i, notemap := range *notemaps {
			if _, ok := mp.Maps[notemap.PR]; !ok {
				mp.Maps[notemap.PR] = make([]*ReleaseNotesMap, 0)
			}
			mp.Maps[notemap.PR] = append(mp.Maps[notemap.PR], &(*notemaps)[i])
		}
	}
	logrus.Infof("Successfully parsed release notes maps for %d PRs from %s", len(mp.Maps), mp.Path)
	return err
}

// GetMapsForPR get the release notes maps for a specific PR number
func (mp *DirectoryMapProvider) GetMapsForPR(pr int) (notesMap []*ReleaseNotesMap, err error) {
	if mp.Maps == nil {
		err := mp.readMaps()
		if err != nil {
			return nil, errors.Wrap(err, "while reading release notes maps")
		}
	}
	if notesMap, ok := mp.Maps[pr]; ok {
		return notesMap, nil
	}
	return nil, nil
}
