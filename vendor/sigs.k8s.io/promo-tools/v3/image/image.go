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

package image

import (
	"fmt"
	"os"
	"sort"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"sigs.k8s.io/release-utils/util"
)

const (
	// Production registry root URL
	ProdRegistry = "k8s.gcr.io"

	// Staging repository root URL prefix
	StagingRepoPrefix = "gcr.io/k8s-staging-"

	// The suffix of the default image repository to promote images from
	// i.e., gcr.io/<staging-prefix>-<staging-suffix>
	// e.g., gcr.io/k8s-staging-foo
	StagingRepoSuffix = "kubernetes"
)

// ManifestList abstracts the manifest used by the image promoter
type ManifestList []struct {
	Name string `json:"name"`

	// A digest to tag(s) map used in the promoter manifest e.g.,
	// "sha256:ef9493aff21f7e368fb3968b46ff2542b0f6863a5de2b9bc58d8d151d8b0232c": ["v1.17.12-rc.0", "foo", "bar"]
	DMap map[string][]string `json:"dmap"`
}

// NewManifestListFromFile parses an image promoter manifest file
func NewManifestListFromFile(manifestPath string) (imagesList *ManifestList, err error) {
	if !util.Exists(manifestPath) {
		return nil, errors.New("could not find image promoter manifest")
	}
	yamlCode, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading yaml code from file")
	}

	imagesList = &ManifestList{}
	if err := imagesList.Parse(yamlCode); err != nil {
		return nil, errors.Wrap(err, "parsing manifest yaml")
	}

	return imagesList, nil
}

// Parse reads yaml code into an ImagePromoterManifest object
func (imagesList *ManifestList) Parse(yamlCode []byte) error {
	if err := yaml.Unmarshal(yamlCode, imagesList); err != nil {
		return err
	}
	return nil
}

// Write writes the promoter image list into an YAML file.
func (imagesList *ManifestList) Write(filePath string) error {
	yamlCode, err := imagesList.ToYAML()
	if err != nil {
		return errors.Wrap(err, "while marshalling image list")
	}
	// Write the yaml into the specified file
	if err := os.WriteFile(filePath, yamlCode, os.FileMode(0o644)); err != nil {
		return errors.Wrap(err, "writing yaml code into file")
	}

	return nil
}

// ToYAML serializes an image list into an YAML file.
// We serialize the data by hand to emulate the way it's done by the image promoter
func (imagesList *ManifestList) ToYAML() ([]byte, error) {
	// The image promoter code sorts images by:
	//	  1. Name 2. Digest SHA (asc)  3. Tag

	// First, sort by name (sort #1)
	sort.Slice(*imagesList, func(i, j int) bool {
		return (*imagesList)[i].Name < (*imagesList)[j].Name
	})

	// Let's build the YAML code
	yamlCode := ""
	for _, imgData := range *imagesList {
		// Add the new name key (it is not sorted in the promoter code)
		yamlCode += fmt.Sprintf("- name: %s\n", imgData.Name)
		yamlCode += "  dmap:\n"

		// Now, lets sort by the digest sha (sort #2)
		keys := make([]string, 0, len(imgData.DMap))
		for k := range imgData.DMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, digestSHA := range keys {
			// Finally, sort bt tag (sort #3)
			tags := imgData.DMap[digestSHA]
			sort.Strings(tags)
			yamlCode += fmt.Sprintf("    %q: [", digestSHA)
			for i, tag := range tags {
				if i > 0 {
					yamlCode += ","
				}
				yamlCode += fmt.Sprintf("%q", tag)
			}
			yamlCode += "]\n"
		}
	}

	return []byte(yamlCode), nil
}
