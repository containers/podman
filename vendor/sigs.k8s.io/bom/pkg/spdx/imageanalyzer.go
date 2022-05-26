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

package spdx

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
)

// ImageAnalyzer is an object that checks images to see if we can add more
//  information to a spdx package based on its content. Each analyzer is
//  written specifically for a layer type. The idea is to be able to enrich
//  common base images with more data to have the most common images covered.
type ImageAnalyzer struct {
	Analyzers map[string]ContainerLayerAnalyzer
}

func NewImageAnalyzer() *ImageAnalyzer {
	// Default options for all analyzers
	opts := &ContainerLayerAnalyzerOptions{
		LicenseCacheDir: filepath.Join(os.TempDir(), spdxLicenseData),
	}

	// Create the instance with all the drivers we have so far
	return &ImageAnalyzer{
		Analyzers: map[string]ContainerLayerAnalyzer{
			"distroless": &distrolessHandler{
				Options: opts,
			},
			"go-runner": &goRunnerHandler{
				Options: opts,
			},
		},
	}
}

// AnalyzeLayer is the main method of the analyzer
//  it will query each of the analyzers to see if we can
//  extract more image from the layer and enrich the
//  spdx package referenced by pkg
func (ia *ImageAnalyzer) AnalyzeLayer(layerPath string, pkg *Package) error {
	if pkg == nil {
		return errors.New("Unable to analyze layer, package is null")
	}
	for label, handler := range ia.Analyzers {
		logrus.Infof("Scanning layer with %s", label)
		can, err := handler.CanHandle(layerPath)
		if err != nil {
			return errors.Wrapf(err, "checking if layer can be handled with %s", label)
		}

		if can {
			return handler.ReadPackageData(layerPath, pkg)
		}
	}
	return nil
}

// ContainerLayerAnalyzer is an interface that knows how to read a
// known container layer and populate a SPDX package
type ContainerLayerAnalyzer interface {
	ReadPackageData(layerPath string, pkg *Package) error
	CanHandle(layerPath string) (bool, error)
}

type ContainerLayerAnalyzerOptions struct {
	LicenseCacheDir string
}
