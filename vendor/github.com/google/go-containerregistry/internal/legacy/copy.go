// Copyright 2019 Google LLC All Rights Reserved.
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

// Package legacy provides methods for interacting with legacy image formats.
package legacy

import (
	"bytes"
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// CopySchema1 allows `[g]crane cp` to work with old images without adding
// full support for schema 1 images to this package.
func CopySchema1(desc *remote.Descriptor, srcRef, dstRef name.Reference, opts ...remote.Option) error {
	m := schema1{}
	if err := json.NewDecoder(bytes.NewReader(desc.Manifest)).Decode(&m); err != nil {
		return err
	}

	for _, layer := range m.FSLayers {
		src := srcRef.Context().Digest(layer.BlobSum)
		dst := dstRef.Context().Digest(layer.BlobSum)

		blob, err := remote.Layer(src, opts...)
		if err != nil {
			return err
		}

		if err := remote.WriteLayer(dst.Context(), blob, opts...); err != nil {
			return err
		}
	}

	return remote.Put(dstRef, desc, opts...)
}

type fslayer struct {
	BlobSum string `json:"blobSum"`
}

type schema1 struct {
	FSLayers []fslayer `json:"fsLayers"`
}
