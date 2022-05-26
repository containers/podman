// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crane

import (
	"fmt"
	"os"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Append reads a layer from path and appends it the the v1.Image base.
func Append(base v1.Image, paths ...string) (v1.Image, error) {
	layers := make([]v1.Layer, 0, len(paths))
	for _, path := range paths {
		layer, err := getLayer(path)
		if err != nil {
			return nil, fmt.Errorf("reading layer %q: %w", path, err)
		}

		layers = append(layers, layer)
	}

	return mutate.AppendLayers(base, layers...)
}

func getLayer(path string) (v1.Layer, error) {
	f, err := streamFile(path)
	if err != nil {
		return nil, err
	}
	if f != nil {
		return stream.NewLayer(f), nil
	}

	return tarball.LayerFromFile(path)
}

// If we're dealing with a named pipe, trying to open it multiple times will
// fail, so we need to do a streaming upload.
//
// returns nil, nil for non-streaming files
func streamFile(path string) (*os.File, error) {
	if path == "-" {
		return os.Stdin, nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !fi.Mode().IsRegular() {
		return os.Open(path)
	}

	return nil, nil
}
