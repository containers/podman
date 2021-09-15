package buildah

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// OCIv1ImageManifest is the MIME type of an OCIv1 image manifest,
	// suitable for specifying as a value of the PreferredManifestType
	// member of a CommitOptions structure.  It is also the default.
	OCIv1ImageManifest = define.OCIv1ImageManifest
	// Dockerv2ImageManifest is the MIME type of a Docker v2s2 image
	// manifest, suitable for specifying as a value of the
	// PreferredManifestType member of a CommitOptions structure.
	Dockerv2ImageManifest = define.Dockerv2ImageManifest
)

type containerImageRef struct {
	fromImageName         string
	fromImageID           string
	store                 storage.Store
	compression           archive.Compression
	name                  reference.Named
	names                 []string
	containerID           string
	mountLabel            string
	layerID               string
	oconfig               []byte
	dconfig               []byte
	created               *time.Time
	createdBy             string
	historyComment        string
	annotations           map[string]string
	preferredManifestType string
	squash                bool
	emptyLayer            bool
	idMappingOptions      *define.IDMappingOptions
	parent                string
	blobDirectory         string
	preEmptyLayers        []v1.History
	postEmptyLayers       []v1.History
}

type blobLayerInfo struct {
	ID   string
	Size int64
}

type containerImageSource struct {
	path          string
	ref           *containerImageRef
	store         storage.Store
	containerID   string
	mountLabel    string
	layerID       string
	names         []string
	compression   archive.Compression
	config        []byte
	configDigest  digest.Digest
	manifest      []byte
	manifestType  string
	blobDirectory string
	blobLayers    map[digest.Digest]blobLayerInfo
}

func (i *containerImageRef) NewImage(ctx context.Context, sc *types.SystemContext) (types.ImageCloser, error) {
	src, err := i.NewImageSource(ctx, sc)
	if err != nil {
		return nil, err
	}
	return image.FromSource(ctx, sc, src)
}

func expectedOCIDiffIDs(image v1.Image) int {
	expected := 0
	for _, history := range image.History {
		if !history.EmptyLayer {
			expected = expected + 1
		}
	}
	return expected
}

func expectedDockerDiffIDs(image docker.V2Image) int {
	expected := 0
	for _, history := range image.History {
		if !history.EmptyLayer {
			expected = expected + 1
		}
	}
	return expected
}

// Compute the media types which we need to attach to a layer, given the type of
// compression that we'll be applying.
func computeLayerMIMEType(what string, layerCompression archive.Compression) (omediaType, dmediaType string, err error) {
	omediaType = v1.MediaTypeImageLayer
	dmediaType = docker.V2S2MediaTypeUncompressedLayer
	if layerCompression != archive.Uncompressed {
		switch layerCompression {
		case archive.Gzip:
			omediaType = v1.MediaTypeImageLayerGzip
			dmediaType = manifest.DockerV2Schema2LayerMediaType
			logrus.Debugf("compressing %s with gzip", what)
		case archive.Bzip2:
			// Until the image specs define a media type for bzip2-compressed layers, even if we know
			// how to decompress them, we can't try to compress layers with bzip2.
			return "", "", errors.New("media type for bzip2-compressed layers is not defined")
		case archive.Xz:
			// Until the image specs define a media type for xz-compressed layers, even if we know
			// how to decompress them, we can't try to compress layers with xz.
			return "", "", errors.New("media type for xz-compressed layers is not defined")
		case archive.Zstd:
			// Until the image specs define a media type for zstd-compressed layers, even if we know
			// how to decompress them, we can't try to compress layers with zstd.
			return "", "", errors.New("media type for zstd-compressed layers is not defined")
		default:
			logrus.Debugf("compressing %s with unknown compressor(?)", what)
		}
	}
	return omediaType, dmediaType, nil
}

// Extract the container's whole filesystem as if it were a single layer.
func (i *containerImageRef) extractRootfs() (io.ReadCloser, chan error, error) {
	var uidMap, gidMap []idtools.IDMap
	mountPoint, err := i.store.Mount(i.containerID, i.mountLabel)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error mounting container %q", i.containerID)
	}
	pipeReader, pipeWriter := io.Pipe()
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		if i.idMappingOptions != nil {
			uidMap, gidMap = convertRuntimeIDMaps(i.idMappingOptions.UIDMap, i.idMappingOptions.GIDMap)
		}
		copierOptions := copier.GetOptions{
			UIDMap: uidMap,
			GIDMap: gidMap,
		}
		err = copier.Get(mountPoint, mountPoint, copierOptions, []string{"."}, pipeWriter)
		errChan <- err
		pipeWriter.Close()

	}()
	return ioutils.NewReadCloserWrapper(pipeReader, func() error {
		if err = pipeReader.Close(); err != nil {
			err = errors.Wrapf(err, "error closing tar archive of container %q", i.containerID)
		}
		if _, err2 := i.store.Unmount(i.containerID, false); err == nil {
			if err2 != nil {
				err2 = errors.Wrapf(err2, "error unmounting container %q", i.containerID)
			}
			err = err2
		}
		return err
	}), errChan, nil
}

// Build fresh copies of the container configuration structures so that we can edit them
// without making unintended changes to the original Builder.
func (i *containerImageRef) createConfigsAndManifests() (v1.Image, v1.Manifest, docker.V2Image, docker.V2S2Manifest, error) {
	created := time.Now().UTC()
	if i.created != nil {
		created = *i.created
	}

	// Build an empty image, and then decode over it.
	oimage := v1.Image{}
	if err := json.Unmarshal(i.oconfig, &oimage); err != nil {
		return v1.Image{}, v1.Manifest{}, docker.V2Image{}, docker.V2S2Manifest{}, err
	}
	// Always replace this value, since we're newer than our base image.
	oimage.Created = &created
	// Clear the list of diffIDs, since we always repopulate it.
	oimage.RootFS.Type = docker.TypeLayers
	oimage.RootFS.DiffIDs = []digest.Digest{}
	// Only clear the history if we're squashing, otherwise leave it be so that we can append
	// entries to it.
	if i.squash {
		oimage.History = []v1.History{}
	}

	// Build an empty image, and then decode over it.
	dimage := docker.V2Image{}
	if err := json.Unmarshal(i.dconfig, &dimage); err != nil {
		return v1.Image{}, v1.Manifest{}, docker.V2Image{}, docker.V2S2Manifest{}, err
	}
	dimage.Parent = docker.ID(i.parent)
	dimage.Container = i.containerID
	if dimage.Config != nil {
		dimage.ContainerConfig = *dimage.Config
	}
	// Always replace this value, since we're newer than our base image.
	dimage.Created = created
	// Clear the list of diffIDs, since we always repopulate it.
	dimage.RootFS = &docker.V2S2RootFS{}
	dimage.RootFS.Type = docker.TypeLayers
	dimage.RootFS.DiffIDs = []digest.Digest{}
	// Only clear the history if we're squashing, otherwise leave it be so
	// that we can append entries to it.  Clear the parent, too, we no
	// longer include its layers and history.
	if i.squash {
		dimage.Parent = ""
		dimage.History = []docker.V2S2History{}
	}

	// Build empty manifests.  The Layers lists will be populated later.
	omanifest := v1.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Config: v1.Descriptor{
			MediaType: v1.MediaTypeImageConfig,
		},
		Layers:      []v1.Descriptor{},
		Annotations: i.annotations,
	}

	dmanifest := docker.V2S2Manifest{
		V2Versioned: docker.V2Versioned{
			SchemaVersion: 2,
			MediaType:     manifest.DockerV2Schema2MediaType,
		},
		Config: docker.V2S2Descriptor{
			MediaType: manifest.DockerV2Schema2ConfigMediaType,
		},
		Layers: []docker.V2S2Descriptor{},
	}

	return oimage, omanifest, dimage, dmanifest, nil
}

func (i *containerImageRef) NewImageSource(ctx context.Context, sc *types.SystemContext) (src types.ImageSource, err error) {
	// Decide which type of manifest and configuration output we're going to provide.
	manifestType := i.preferredManifestType
	// If it's not a format we support, return an error.
	if manifestType != v1.MediaTypeImageManifest && manifestType != manifest.DockerV2Schema2MediaType {
		return nil, errors.Errorf("no supported manifest types (attempted to use %q, only know %q and %q)",
			manifestType, v1.MediaTypeImageManifest, manifest.DockerV2Schema2MediaType)
	}
	// Start building the list of layers using the read-write layer.
	layers := []string{}
	layerID := i.layerID
	layer, err := i.store.Layer(layerID)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read layer %q", layerID)
	}
	// Walk the list of parent layers, prepending each as we go.  If we're squashing,
	// stop at the layer ID of the top layer, which we won't really be using anyway.
	for layer != nil {
		layers = append(append([]string{}, layerID), layers...)
		layerID = layer.Parent
		if layerID == "" || i.squash {
			err = nil
			break
		}
		layer, err = i.store.Layer(layerID)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to read layer %q", layerID)
		}
	}
	logrus.Debugf("layer list: %q", layers)

	// Make a temporary directory to hold blobs.
	path, err := ioutil.TempDir(os.TempDir(), define.Package)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating temporary directory to hold layer blobs")
	}
	logrus.Debugf("using %q to hold temporary data", path)
	defer func() {
		if src == nil {
			err2 := os.RemoveAll(path)
			if err2 != nil {
				logrus.Errorf("error removing layer blob directory: %v", err)
			}
		}
	}()

	// Build fresh copies of the configurations and manifest so that we don't mess with any
	// values in the Builder object itself.
	oimage, omanifest, dimage, dmanifest, err := i.createConfigsAndManifests()
	if err != nil {
		return nil, err
	}

	// Extract each layer and compute its digests, both compressed (if requested) and uncompressed.
	blobLayers := make(map[digest.Digest]blobLayerInfo)
	for _, layerID := range layers {
		what := fmt.Sprintf("layer %q", layerID)
		if i.squash {
			what = fmt.Sprintf("container %q", i.containerID)
		}
		// The default layer media type assumes no compression.
		omediaType := v1.MediaTypeImageLayer
		dmediaType := docker.V2S2MediaTypeUncompressedLayer
		// Look up this layer.
		layer, err := i.store.Layer(layerID)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to locate layer %q", layerID)
		}
		// If we're up to the final layer, but we don't want to include
		// a diff for it, we're done.
		if i.emptyLayer && layerID == i.layerID {
			continue
		}
		// If we already know the digest of the contents of parent
		// layers, reuse their blobsums, diff IDs, and sizes.
		if !i.squash && layerID != i.layerID && layer.UncompressedDigest != "" {
			layerBlobSum := layer.UncompressedDigest
			layerBlobSize := layer.UncompressedSize
			diffID := layer.UncompressedDigest
			// Note this layer in the manifest, using the appropriate blobsum.
			olayerDescriptor := v1.Descriptor{
				MediaType: omediaType,
				Digest:    layerBlobSum,
				Size:      layerBlobSize,
			}
			omanifest.Layers = append(omanifest.Layers, olayerDescriptor)
			dlayerDescriptor := docker.V2S2Descriptor{
				MediaType: dmediaType,
				Digest:    layerBlobSum,
				Size:      layerBlobSize,
			}
			dmanifest.Layers = append(dmanifest.Layers, dlayerDescriptor)
			// Note this layer in the list of diffIDs, again using the uncompressed digest.
			oimage.RootFS.DiffIDs = append(oimage.RootFS.DiffIDs, diffID)
			dimage.RootFS.DiffIDs = append(dimage.RootFS.DiffIDs, diffID)
			blobLayers[diffID] = blobLayerInfo{
				ID:   layer.ID,
				Size: layerBlobSize,
			}
			continue
		}
		// Figure out if we need to change the media type, in case we've changed the compression.
		omediaType, dmediaType, err = computeLayerMIMEType(what, i.compression)
		if err != nil {
			return nil, err
		}
		// Start reading either the layer or the whole container rootfs.
		noCompression := archive.Uncompressed
		diffOptions := &storage.DiffOptions{
			Compression: &noCompression,
		}
		var rc io.ReadCloser
		var errChan chan error
		if i.squash {
			// Extract the root filesystem as a single layer.
			rc, errChan, err = i.extractRootfs()
			if err != nil {
				return nil, err
			}
		} else {
			// Extract this layer, one of possibly many.
			rc, err = i.store.Diff("", layerID, diffOptions)
			if err != nil {
				return nil, errors.Wrapf(err, "error extracting %s", what)
			}
		}
		srcHasher := digest.Canonical.Digester()
		// Set up to write the possibly-recompressed blob.
		layerFile, err := os.OpenFile(filepath.Join(path, "layer"), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			rc.Close()
			return nil, errors.Wrapf(err, "error opening file for %s", what)
		}

		counter := ioutils.NewWriteCounter(layerFile)
		var destHasher digest.Digester
		var multiWriter io.Writer
		// Avoid rehashing when we do not compress.
		if i.compression != archive.Uncompressed {
			destHasher = digest.Canonical.Digester()
			multiWriter = io.MultiWriter(counter, destHasher.Hash())
		} else {
			destHasher = srcHasher
			multiWriter = counter
		}
		// Compress the layer, if we're recompressing it.
		writeCloser, err := archive.CompressStream(multiWriter, i.compression)
		if err != nil {
			layerFile.Close()
			rc.Close()
			return nil, errors.Wrapf(err, "error compressing %s", what)
		}
		writer := io.MultiWriter(writeCloser, srcHasher.Hash())
		// Use specified timestamps in the layer, if we're doing that for
		// history entries.
		if i.created != nil {
			nestedWriteCloser := ioutils.NewWriteCloserWrapper(writer, writeCloser.Close)
			writeCloser = newTarFilterer(nestedWriteCloser, func(hdr *tar.Header) (bool, bool, io.Reader) {
				// Changing a zeroed field to a non-zero field
				// can affect the format that the library uses
				// for writing the header, so only change
				// fields that are already set to avoid
				// changing the format (and as a result,
				// changing the length) of the header that we
				// write.
				if !hdr.ModTime.IsZero() {
					hdr.ModTime = *i.created
				}
				if !hdr.AccessTime.IsZero() {
					hdr.AccessTime = *i.created
				}
				if !hdr.ChangeTime.IsZero() {
					hdr.ChangeTime = *i.created
				}
				return false, false, nil
			})
			writer = io.Writer(writeCloser)
		}
		size, err := io.Copy(writer, rc)
		writeCloser.Close()
		layerFile.Close()
		rc.Close()

		if errChan != nil {
			err = <-errChan
			if err != nil {
				return nil, err
			}
		}

		if err != nil {
			return nil, errors.Wrapf(err, "error storing %s to file", what)
		}
		if i.compression == archive.Uncompressed {
			if size != counter.Count {
				return nil, errors.Errorf("error storing %s to file: inconsistent layer size (copied %d, wrote %d)", what, size, counter.Count)
			}
		} else {
			size = counter.Count
		}
		logrus.Debugf("%s size is %d bytes, uncompressed digest %s, possibly-compressed digest %s", what, size, srcHasher.Digest().String(), destHasher.Digest().String())
		// Rename the layer so that we can more easily find it by digest later.
		finalBlobName := filepath.Join(path, destHasher.Digest().String())
		if err = os.Rename(filepath.Join(path, "layer"), finalBlobName); err != nil {
			return nil, errors.Wrapf(err, "error storing %s to file while renaming %q to %q", what, filepath.Join(path, "layer"), finalBlobName)
		}
		// Add a note in the manifest about the layer.  The blobs are identified by their possibly-
		// compressed blob digests.
		olayerDescriptor := v1.Descriptor{
			MediaType: omediaType,
			Digest:    destHasher.Digest(),
			Size:      size,
		}
		omanifest.Layers = append(omanifest.Layers, olayerDescriptor)
		dlayerDescriptor := docker.V2S2Descriptor{
			MediaType: dmediaType,
			Digest:    destHasher.Digest(),
			Size:      size,
		}
		dmanifest.Layers = append(dmanifest.Layers, dlayerDescriptor)
		// Add a note about the diffID, which is always the layer's uncompressed digest.
		oimage.RootFS.DiffIDs = append(oimage.RootFS.DiffIDs, srcHasher.Digest())
		dimage.RootFS.DiffIDs = append(dimage.RootFS.DiffIDs, srcHasher.Digest())
	}

	// Build history notes in the image configurations.
	appendHistory := func(history []v1.History) {
		for i := range history {
			var created *time.Time
			if history[i].Created != nil {
				copiedTimestamp := *history[i].Created
				created = &copiedTimestamp
			}
			onews := v1.History{
				Created:    created,
				CreatedBy:  history[i].CreatedBy,
				Author:     history[i].Author,
				Comment:    history[i].Comment,
				EmptyLayer: true,
			}
			oimage.History = append(oimage.History, onews)
			if created == nil {
				created = &time.Time{}
			}
			dnews := docker.V2S2History{
				Created:    *created,
				CreatedBy:  history[i].CreatedBy,
				Author:     history[i].Author,
				Comment:    history[i].Comment,
				EmptyLayer: true,
			}
			dimage.History = append(dimage.History, dnews)
		}
	}
	appendHistory(i.preEmptyLayers)
	created := time.Now().UTC()
	if i.created != nil {
		created = (*i.created).UTC()
	}
	comment := i.historyComment
	// Add a comment for which base image is being used
	if strings.Contains(i.parent, i.fromImageID) && i.fromImageName != i.fromImageID {
		comment += "FROM " + i.fromImageName
	}
	onews := v1.History{
		Created:    &created,
		CreatedBy:  i.createdBy,
		Author:     oimage.Author,
		Comment:    comment,
		EmptyLayer: i.emptyLayer,
	}
	oimage.History = append(oimage.History, onews)
	dnews := docker.V2S2History{
		Created:    created,
		CreatedBy:  i.createdBy,
		Author:     dimage.Author,
		Comment:    comment,
		EmptyLayer: i.emptyLayer,
	}
	dimage.History = append(dimage.History, dnews)
	appendHistory(i.postEmptyLayers)

	// Sanity check that we didn't just create a mismatch between non-empty layers in the
	// history and the number of diffIDs.
	expectedDiffIDs := expectedOCIDiffIDs(oimage)
	if len(oimage.RootFS.DiffIDs) != expectedDiffIDs {
		return nil, errors.Errorf("internal error: history lists %d non-empty layers, but we have %d layers on disk", expectedDiffIDs, len(oimage.RootFS.DiffIDs))
	}
	expectedDiffIDs = expectedDockerDiffIDs(dimage)
	if len(dimage.RootFS.DiffIDs) != expectedDiffIDs {
		return nil, errors.Errorf("internal error: history lists %d non-empty layers, but we have %d layers on disk", expectedDiffIDs, len(dimage.RootFS.DiffIDs))
	}

	// Encode the image configuration blob.
	oconfig, err := json.Marshal(&oimage)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding %#v as json", oimage)
	}
	logrus.Debugf("OCIv1 config = %s", oconfig)

	// Add the configuration blob to the manifest.
	omanifest.Config.Digest = digest.Canonical.FromBytes(oconfig)
	omanifest.Config.Size = int64(len(oconfig))
	omanifest.Config.MediaType = v1.MediaTypeImageConfig

	// Encode the manifest.
	omanifestbytes, err := json.Marshal(&omanifest)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding %#v as json", omanifest)
	}
	logrus.Debugf("OCIv1 manifest = %s", omanifestbytes)

	// Encode the image configuration blob.
	dconfig, err := json.Marshal(&dimage)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding %#v as json", dimage)
	}
	logrus.Debugf("Docker v2s2 config = %s", dconfig)

	// Add the configuration blob to the manifest.
	dmanifest.Config.Digest = digest.Canonical.FromBytes(dconfig)
	dmanifest.Config.Size = int64(len(dconfig))
	dmanifest.Config.MediaType = manifest.DockerV2Schema2ConfigMediaType

	// Encode the manifest.
	dmanifestbytes, err := json.Marshal(&dmanifest)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding %#v as json", dmanifest)
	}
	logrus.Debugf("Docker v2s2 manifest = %s", dmanifestbytes)

	// Decide which manifest and configuration blobs we'll actually output.
	var config []byte
	var imageManifest []byte
	switch manifestType {
	case v1.MediaTypeImageManifest:
		imageManifest = omanifestbytes
		config = oconfig
	case manifest.DockerV2Schema2MediaType:
		imageManifest = dmanifestbytes
		config = dconfig
	default:
		panic("unreachable code: unsupported manifest type")
	}
	src = &containerImageSource{
		path:          path,
		ref:           i,
		store:         i.store,
		containerID:   i.containerID,
		mountLabel:    i.mountLabel,
		layerID:       i.layerID,
		names:         i.names,
		compression:   i.compression,
		config:        config,
		configDigest:  digest.Canonical.FromBytes(config),
		manifest:      imageManifest,
		manifestType:  manifestType,
		blobDirectory: i.blobDirectory,
		blobLayers:    blobLayers,
	}
	return src, nil
}

func (i *containerImageRef) NewImageDestination(ctx context.Context, sc *types.SystemContext) (types.ImageDestination, error) {
	return nil, errors.Errorf("can't write to a container")
}

func (i *containerImageRef) DockerReference() reference.Named {
	return i.name
}

func (i *containerImageRef) StringWithinTransport() string {
	if len(i.names) > 0 {
		return i.names[0]
	}
	return ""
}

func (i *containerImageRef) DeleteImage(context.Context, *types.SystemContext) error {
	// we were never here
	return nil
}

func (i *containerImageRef) PolicyConfigurationIdentity() string {
	return ""
}

func (i *containerImageRef) PolicyConfigurationNamespaces() []string {
	return nil
}

func (i *containerImageRef) Transport() types.ImageTransport {
	return is.Transport
}

func (i *containerImageSource) Close() error {
	err := os.RemoveAll(i.path)
	if err != nil {
		return errors.Wrapf(err, "error removing layer blob directory")
	}
	return nil
}

func (i *containerImageSource) Reference() types.ImageReference {
	return i.ref
}

func (i *containerImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	return nil, nil
}

func (i *containerImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	return i.manifest, i.manifestType, nil
}

func (i *containerImageSource) LayerInfosForCopy(ctx context.Context, instanceDigest *digest.Digest) ([]types.BlobInfo, error) {
	return nil, nil
}

func (i *containerImageSource) HasThreadSafeGetBlob() bool {
	return false
}

func (i *containerImageSource) GetBlob(ctx context.Context, blob types.BlobInfo, cache types.BlobInfoCache) (reader io.ReadCloser, size int64, err error) {
	if blob.Digest == i.configDigest {
		logrus.Debugf("start reading config")
		reader := bytes.NewReader(i.config)
		closer := func() error {
			logrus.Debugf("finished reading config")
			return nil
		}
		return ioutils.NewReadCloserWrapper(reader, closer), reader.Size(), nil
	}
	var layerReadCloser io.ReadCloser
	size = -1
	if blobLayerInfo, ok := i.blobLayers[blob.Digest]; ok {
		noCompression := archive.Uncompressed
		diffOptions := &storage.DiffOptions{
			Compression: &noCompression,
		}
		layerReadCloser, err = i.store.Diff("", blobLayerInfo.ID, diffOptions)
		size = blobLayerInfo.Size
	} else {
		for _, blobDir := range []string{i.blobDirectory, i.path} {
			var layerFile *os.File
			layerFile, err = os.OpenFile(filepath.Join(blobDir, blob.Digest.String()), os.O_RDONLY, 0600)
			if err == nil {
				st, err := layerFile.Stat()
				if err != nil {
					logrus.Warnf("error reading size of layer file %q: %v", blob.Digest.String(), err)
				} else {
					size = st.Size()
					layerReadCloser = layerFile
					break
				}
				layerFile.Close()
			}
			if !os.IsNotExist(err) {
				logrus.Debugf("error checking for layer %q in %q: %v", blob.Digest.String(), blobDir, err)
			}
		}
	}
	if err != nil || layerReadCloser == nil || size == -1 {
		logrus.Debugf("error reading layer %q: %v", blob.Digest.String(), err)
		return nil, -1, errors.Wrap(err, "error opening layer blob")
	}
	logrus.Debugf("reading layer %q", blob.Digest.String())
	closer := func() error {
		logrus.Debugf("finished reading layer %q", blob.Digest.String())
		if err := layerReadCloser.Close(); err != nil {
			return errors.Wrapf(err, "error closing layer %q after reading", blob.Digest.String())
		}
		return nil
	}
	return ioutils.NewReadCloserWrapper(layerReadCloser, closer), size, nil
}

func (b *Builder) makeImageRef(options CommitOptions) (types.ImageReference, error) {
	var name reference.Named
	container, err := b.store.Container(b.ContainerID)
	if err != nil {
		return nil, errors.Wrapf(err, "error locating container %q", b.ContainerID)
	}
	if len(container.Names) > 0 {
		if parsed, err2 := reference.ParseNamed(container.Names[0]); err2 == nil {
			name = parsed
		}
	}
	manifestType := options.PreferredManifestType
	if manifestType == "" {
		manifestType = define.OCIv1ImageManifest
	}
	oconfig, err := json.Marshal(&b.OCIv1)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding OCI-format image configuration %#v", b.OCIv1)
	}
	dconfig, err := json.Marshal(&b.Docker)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding docker-format image configuration %#v", b.Docker)
	}
	var created *time.Time
	if options.HistoryTimestamp != nil {
		historyTimestampUTC := options.HistoryTimestamp.UTC()
		created = &historyTimestampUTC
	}
	createdBy := b.CreatedBy()
	if createdBy == "" {
		createdBy = strings.Join(b.Shell(), " ")
		if createdBy == "" {
			createdBy = "/bin/sh"
		}
	}

	parent := ""
	if b.FromImageID != "" {
		parentDigest := digest.NewDigestFromEncoded(digest.Canonical, b.FromImageID)
		if parentDigest.Validate() == nil {
			parent = parentDigest.String()
		}
	}

	ref := &containerImageRef{
		fromImageName:         b.FromImage,
		fromImageID:           b.FromImageID,
		store:                 b.store,
		compression:           options.Compression,
		name:                  name,
		names:                 container.Names,
		containerID:           container.ID,
		mountLabel:            b.MountLabel,
		layerID:               container.LayerID,
		oconfig:               oconfig,
		dconfig:               dconfig,
		created:               created,
		createdBy:             createdBy,
		historyComment:        b.HistoryComment(),
		annotations:           b.Annotations(),
		preferredManifestType: manifestType,
		squash:                options.Squash,
		emptyLayer:            options.EmptyLayer && !options.Squash,
		idMappingOptions:      &b.IDMappingOptions,
		parent:                parent,
		blobDirectory:         options.BlobDirectory,
		preEmptyLayers:        b.PrependedEmptyLayers,
		postEmptyLayers:       b.AppendedEmptyLayers,
	}
	return ref, nil
}
