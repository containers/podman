package buildah

import (
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

	"github.com/containers/buildah/docker"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/image"
	"github.com/containers/image/manifest"
	is "github.com/containers/image/storage"
	"github.com/containers/image/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/ioutils"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// OCIv1ImageManifest is the MIME type of an OCIv1 image manifest,
	// suitable for specifying as a value of the PreferredManifestType
	// member of a CommitOptions structure.  It is also the default.
	OCIv1ImageManifest = v1.MediaTypeImageManifest
	// Dockerv2ImageManifest is the MIME type of a Docker v2s2 image
	// manifest, suitable for specifying as a value of the
	// PreferredManifestType member of a CommitOptions structure.
	Dockerv2ImageManifest = manifest.DockerV2Schema2MediaType
)

type containerImageRef struct {
	store                 storage.Store
	compression           archive.Compression
	name                  reference.Named
	names                 []string
	containerID           string
	mountLabel            string
	layerID               string
	oconfig               []byte
	dconfig               []byte
	created               time.Time
	createdBy             string
	historyComment        string
	annotations           map[string]string
	preferredManifestType string
	exporting             bool
	squash                bool
	emptyLayer            bool
	tarPath               func(path string) (io.ReadCloser, error)
	parent                string
	blobDirectory         string
	preEmptyLayers        []v1.History
	postEmptyLayers       []v1.History
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
	exporting     bool
	blobDirectory string
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
		default:
			logrus.Debugf("compressing %s with unknown compressor(?)", what)
		}
	}
	return omediaType, dmediaType, nil
}

// Extract the container's whole filesystem as if it were a single layer.
func (i *containerImageRef) extractRootfs() (io.ReadCloser, error) {
	mountPoint, err := i.store.Mount(i.containerID, i.mountLabel)
	if err != nil {
		return nil, errors.Wrapf(err, "error mounting container %q", i.containerID)
	}
	rc, err := i.tarPath(mountPoint)
	if err != nil {
		return nil, errors.Wrapf(err, "error extracting rootfs from container %q", i.containerID)
	}
	return ioutils.NewReadCloserWrapper(rc, func() error {
		if err = rc.Close(); err != nil {
			err = errors.Wrapf(err, "error closing tar archive of container %q", i.containerID)
		}
		if _, err2 := i.store.Unmount(i.containerID, false); err == nil {
			if err2 != nil {
				err2 = errors.Wrapf(err2, "error unmounting container %q", i.containerID)
			}
			err = err2
		}
		return err
	}), nil
}

// Build fresh copies of the container configuration structures so that we can edit them
// without making unintended changes to the original Builder.
func (i *containerImageRef) createConfigsAndManifests() (v1.Image, v1.Manifest, docker.V2Image, docker.V2S2Manifest, error) {
	created := i.created

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
	// Always replace this value, since we're newer than our base image.
	dimage.Created = created
	// Clear the list of diffIDs, since we always repopulate it.
	dimage.RootFS = &docker.V2S2RootFS{}
	dimage.RootFS.Type = docker.TypeLayers
	dimage.RootFS.DiffIDs = []digest.Digest{}
	// Only clear the history if we're squashing, otherwise leave it be so that we can append
	// entries to it.
	if i.squash {
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
	path, err := ioutil.TempDir(os.TempDir(), Package)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating temporary directory to hold layer blobs")
	}
	logrus.Debugf("using %q to hold temporary data", path)
	defer func() {
		if src == nil {
			err2 := os.RemoveAll(path)
			if err2 != nil {
				logrus.Errorf("error removing layer blob directory %q: %v", path, err)
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
		// If we're not re-exporting the data, and we're reusing layers individually, reuse
		// the blobsum and diff IDs.
		if !i.exporting && !i.squash && layerID != i.layerID {
			if layer.UncompressedDigest == "" {
				return nil, errors.Errorf("unable to look up size of layer %q", layerID)
			}
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
		if i.squash {
			// Extract the root filesystem as a single layer.
			rc, err = i.extractRootfs()
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
		reader := io.TeeReader(rc, srcHasher.Hash())
		// Set up to write the possibly-recompressed blob.
		layerFile, err := os.OpenFile(filepath.Join(path, "layer"), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			rc.Close()
			return nil, errors.Wrapf(err, "error opening file for %s", what)
		}
		destHasher := digest.Canonical.Digester()
		counter := ioutils.NewWriteCounter(layerFile)
		multiWriter := io.MultiWriter(counter, destHasher.Hash())
		// Compress the layer, if we're recompressing it.
		writer, err := archive.CompressStream(multiWriter, i.compression)
		if err != nil {
			layerFile.Close()
			rc.Close()
			return nil, errors.Wrapf(err, "error compressing %s", what)
		}
		size, err := io.Copy(writer, reader)
		writer.Close()
		layerFile.Close()
		rc.Close()
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
		logrus.Debugf("%s size is %d bytes", what, size)
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
	onews := v1.History{
		Created:    &i.created,
		CreatedBy:  i.createdBy,
		Author:     oimage.Author,
		Comment:    i.historyComment,
		EmptyLayer: i.emptyLayer,
	}
	oimage.History = append(oimage.History, onews)
	dnews := docker.V2S2History{
		Created:    i.created,
		CreatedBy:  i.createdBy,
		Author:     dimage.Author,
		Comment:    i.historyComment,
		EmptyLayer: i.emptyLayer,
	}
	dimage.History = append(dimage.History, dnews)
	appendHistory(i.postEmptyLayers)
	dimage.Parent = docker.ID(i.parent)

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
		exporting:     i.exporting,
		blobDirectory: i.blobDirectory,
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
		return errors.Wrapf(err, "error removing layer blob directory %q", i.path)
	}
	return nil
}

func (i *containerImageSource) Reference() types.ImageReference {
	return i.ref
}

func (i *containerImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	if instanceDigest != nil {
		return nil, errors.Errorf("containerImageSource does not support manifest lists")
	}
	return nil, nil
}

func (i *containerImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	if instanceDigest != nil {
		return nil, "", errors.Errorf("containerImageSource does not support manifest lists")
	}
	return i.manifest, i.manifestType, nil
}

func (i *containerImageSource) LayerInfosForCopy(ctx context.Context) ([]types.BlobInfo, error) {
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
	var layerFile *os.File
	for _, path := range []string{i.blobDirectory, i.path} {
		layerFile, err = os.OpenFile(filepath.Join(path, blob.Digest.String()), os.O_RDONLY, 0600)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			logrus.Debugf("error checking for layer %q in %q: %v", blob.Digest.String(), path, err)
		}
	}
	if err != nil {
		logrus.Debugf("error reading layer %q: %v", blob.Digest.String(), err)
		return nil, -1, errors.Wrapf(err, "error opening file %q to buffer layer blob", filepath.Join(i.path, blob.Digest.String()))
	}
	size = -1
	st, err := layerFile.Stat()
	if err != nil {
		logrus.Warnf("error reading size of layer %q: %v", blob.Digest.String(), err)
	} else {
		size = st.Size()
	}
	logrus.Debugf("reading layer %q", blob.Digest.String())
	closer := func() error {
		logrus.Debugf("finished reading layer %q", blob.Digest.String())
		if err := layerFile.Close(); err != nil {
			return errors.Wrapf(err, "error closing layer %q after reading", blob.Digest.String())
		}
		return nil
	}
	return ioutils.NewReadCloserWrapper(layerFile, closer), size, nil
}

func (b *Builder) makeImageRef(options CommitOptions, exporting bool) (types.ImageReference, error) {
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
		manifestType = OCIv1ImageManifest
	}
	oconfig, err := json.Marshal(&b.OCIv1)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding OCI-format image configuration %#v", b.OCIv1)
	}
	dconfig, err := json.Marshal(&b.Docker)
	if err != nil {
		return nil, errors.Wrapf(err, "error encoding docker-format image configuration %#v", b.Docker)
	}
	created := time.Now().UTC()
	if options.HistoryTimestamp != nil {
		created = options.HistoryTimestamp.UTC()
	}
	createdBy := b.CreatedBy()
	if createdBy == "" {
		createdBy = strings.Join(b.Shell(), " ")
		if createdBy == "" {
			createdBy = "/bin/sh"
		}
	}

	if options.OmitTimestamp {
		created = time.Unix(0, 0)
	}

	parent := ""
	if b.FromImageID != "" {
		parentDigest := digest.NewDigestFromEncoded(digest.Canonical, b.FromImageID)
		if parentDigest.Validate() == nil {
			parent = parentDigest.String()
		}
	}

	ref := &containerImageRef{
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
		exporting:             exporting,
		squash:                options.Squash,
		emptyLayer:            options.EmptyLayer,
		tarPath:               b.tarPath(),
		parent:                parent,
		blobDirectory:         options.BlobDirectory,
		preEmptyLayers:        b.PrependedEmptyLayers,
		postEmptyLayers:       b.AppendedEmptyLayers,
	}
	return ref, nil
}
