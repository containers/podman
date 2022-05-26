// Copyright 2020 Google LLC All Rights Reserved.
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
	"errors"
	"fmt"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// Optimize optimizes a remote image or index from src to dst.
// THIS API IS EXPERIMENTAL AND SUBJECT TO CHANGE WITHOUT WARNING.
func Optimize(src, dst string, prioritize []string, opt ...Option) error {
	pset := newStringSet(prioritize)
	o := makeOptions(opt...)
	srcRef, err := name.ParseReference(src, o.Name...)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %w", src, err)
	}

	dstRef, err := name.ParseReference(dst, o.Name...)
	if err != nil {
		return fmt.Errorf("parsing reference for %q: %w", dst, err)
	}

	logs.Progress.Printf("Optimizing from %v to %v", srcRef, dstRef)
	desc, err := remote.Get(srcRef, o.Remote...)
	if err != nil {
		return fmt.Errorf("fetching %q: %w", src, err)
	}

	switch desc.MediaType {
	case types.OCIImageIndex, types.DockerManifestList:
		// Handle indexes separately.
		if o.Platform != nil {
			// If platform is explicitly set, don't optimize the whole index, just the appropriate image.
			if err := optimizeAndPushImage(desc, dstRef, pset, o); err != nil {
				return fmt.Errorf("failed to optimize image: %w", err)
			}
		} else {
			if err := optimizeAndPushIndex(desc, dstRef, pset, o); err != nil {
				return fmt.Errorf("failed to optimize index: %w", err)
			}
		}

	case types.DockerManifestSchema1, types.DockerManifestSchema1Signed:
		return errors.New("docker schema 1 images are not supported")

	default:
		// Assume anything else is an image, since some registries don't set mediaTypes properly.
		if err := optimizeAndPushImage(desc, dstRef, pset, o); err != nil {
			return fmt.Errorf("failed to optimize image: %w", err)
		}
	}

	return nil
}

func optimizeAndPushImage(desc *remote.Descriptor, dstRef name.Reference, prioritize stringSet, o Options) error {
	img, err := desc.Image()
	if err != nil {
		return err
	}

	missing, oimg, err := optimizeImage(img, prioritize)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		return fmt.Errorf("the following prioritized files were missing from image: %v", missing.List())
	}

	return remote.Write(dstRef, oimg, o.Remote...)
}

func optimizeImage(img v1.Image, prioritize stringSet) (stringSet, v1.Image, error) {
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, nil, err
	}
	ocfg := cfg.DeepCopy()
	ocfg.History = nil
	ocfg.RootFS.DiffIDs = nil

	oimg, err := mutate.ConfigFile(empty.Image, ocfg)
	if err != nil {
		return nil, nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, nil, err
	}

	missingFromImage := newStringSet(prioritize.List())
	olayers := make([]mutate.Addendum, 0, len(layers))
	for _, layer := range layers {
		missingFromLayer := []string{}
		olayer, err := tarball.LayerFromOpener(layer.Uncompressed,
			tarball.WithEstargz,
			tarball.WithEstargzOptions(
				estargz.WithPrioritizedFiles(prioritize.List()),
				estargz.WithAllowPrioritizeNotFound(&missingFromLayer),
			))
		if err != nil {
			return nil, nil, err
		}
		missingFromImage = missingFromImage.Intersection(newStringSet(missingFromLayer))

		olayers = append(olayers, mutate.Addendum{
			Layer:     olayer,
			MediaType: types.DockerLayer,
		})
	}

	oimg, err = mutate.Append(oimg, olayers...)
	if err != nil {
		return nil, nil, err
	}
	return missingFromImage, oimg, nil
}

func optimizeAndPushIndex(desc *remote.Descriptor, dstRef name.Reference, prioritize stringSet, o Options) error {
	idx, err := desc.ImageIndex()
	if err != nil {
		return err
	}

	missing, oidx, err := optimizeIndex(idx, prioritize)
	if err != nil {
		return err
	}

	if len(missing) > 0 {
		return fmt.Errorf("the following prioritized files were missing from all images: %v", missing.List())
	}

	return remote.WriteIndex(dstRef, oidx, o.Remote...)
}

func optimizeIndex(idx v1.ImageIndex, prioritize stringSet) (stringSet, v1.ImageIndex, error) {
	im, err := idx.IndexManifest()
	if err != nil {
		return nil, nil, err
	}

	missingFromIndex := newStringSet(prioritize.List())

	// Build an image for each child from the base and append it to a new index to produce the result.
	adds := make([]mutate.IndexAddendum, 0, len(im.Manifests))
	for _, desc := range im.Manifests {
		img, err := idx.Image(desc.Digest)
		if err != nil {
			return nil, nil, err
		}

		missingFromImage, oimg, err := optimizeImage(img, prioritize)
		if err != nil {
			return nil, nil, err
		}
		missingFromIndex = missingFromIndex.Intersection(missingFromImage)
		adds = append(adds, mutate.IndexAddendum{
			Add: oimg,
			Descriptor: v1.Descriptor{
				URLs:        desc.URLs,
				MediaType:   desc.MediaType,
				Annotations: desc.Annotations,
				Platform:    desc.Platform,
			},
		})
	}

	idxType, err := idx.MediaType()
	if err != nil {
		return nil, nil, err
	}

	return missingFromIndex, mutate.IndexMediaType(mutate.AppendManifests(empty.Index, adds...), idxType), nil
}

type stringSet map[string]struct{}

func newStringSet(in []string) stringSet {
	ss := stringSet{}
	for _, s := range in {
		ss[s] = struct{}{}
	}
	return ss
}

func (s stringSet) List() []string {
	result := make([]string, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	return result
}

func (s stringSet) Intersection(rhs stringSet) stringSet {
	// To appease ST1016
	lhs := s

	// Make sure len(lhs) >= len(rhs)
	if len(lhs) < len(rhs) {
		return rhs.Intersection(lhs)
	}

	result := stringSet{}
	for k := range lhs {
		if _, ok := rhs[k]; ok {
			result[k] = struct{}{}
		}
	}
	return result
}
