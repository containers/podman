//go:build !remote

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/common/libimage"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v5/pkg/libartifact"
	libartTypes "github.com/containers/podman/v5/pkg/libartifact/types"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	specV1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

var (
	ErrEmptyArtifactName = errors.New("artifact name cannot be empty")
)

const ManifestSchemaVersion = 2

type ArtifactStore struct {
	SystemContext *types.SystemContext
	storePath     string
}

// NewArtifactStore is a constructor for artifact stores.  Most artifact dealings depend on this. Store path is
// the filesystem location.
func NewArtifactStore(storePath string, sc *types.SystemContext) (*ArtifactStore, error) {
	if storePath == "" {
		return nil, errors.New("store path cannot be empty")
	}
	logrus.Debugf("Using artifact store path: %s", storePath)

	artifactStore := &ArtifactStore{
		storePath:     storePath,
		SystemContext: sc,
	}

	// if the storage dir does not exist, we need to create it.
	baseDir := filepath.Dir(artifactStore.indexPath())
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, err
	}
	// if the index file is not present we need to create an empty one
	if err := fileutils.Exists(artifactStore.indexPath()); err != nil && errors.Is(err, os.ErrNotExist) {
		if createErr := artifactStore.createEmptyManifest(); createErr != nil {
			return nil, createErr
		}
	}
	return artifactStore, nil
}

// Remove an artifact from the local artifact store
func (as ArtifactStore) Remove(ctx context.Context, name string) (*digest.Digest, error) {
	if len(name) == 0 {
		return nil, ErrEmptyArtifactName
	}

	// validate and see if the input is a digest
	artifacts, err := as.getArtifacts(ctx, nil)
	if err != nil {
		return nil, err
	}

	arty, nameIsDigest, err := artifacts.GetByNameOrDigest(name)
	if err != nil {
		return nil, err
	}
	if nameIsDigest {
		name = arty.Name
	}
	ir, err := layout.NewReference(as.storePath, name)
	if err != nil {
		return nil, err
	}
	artifactDigest, err := arty.GetDigest()
	if err != nil {
		return nil, err
	}
	return artifactDigest, ir.DeleteImage(ctx, as.SystemContext)
}

// Inspect an artifact in a local store
func (as ArtifactStore) Inspect(ctx context.Context, nameOrDigest string) (*libartifact.Artifact, error) {
	if len(nameOrDigest) == 0 {
		return nil, ErrEmptyArtifactName
	}
	artifacts, err := as.getArtifacts(ctx, nil)
	if err != nil {
		return nil, err
	}
	inspectData, _, err := artifacts.GetByNameOrDigest(nameOrDigest)
	return inspectData, err
}

// List artifacts in the local store
func (as ArtifactStore) List(ctx context.Context) (libartifact.ArtifactList, error) {
	return as.getArtifacts(ctx, nil)
}

// Pull an artifact from an image registry to a local store
func (as ArtifactStore) Pull(ctx context.Context, name string, opts libimage.CopyOptions) error {
	if len(name) == 0 {
		return ErrEmptyArtifactName
	}
	srcRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", name))
	if err != nil {
		return err
	}
	destRef, err := layout.NewReference(as.storePath, name)
	if err != nil {
		return err
	}
	copyer, err := libimage.NewCopier(&opts, as.SystemContext, nil)
	if err != nil {
		return err
	}
	_, err = copyer.Copy(ctx, srcRef, destRef)
	if err != nil {
		return err
	}
	return copyer.Close()
}

// Push an artifact to an image registry
func (as ArtifactStore) Push(ctx context.Context, src, dest string, opts libimage.CopyOptions) error {
	if len(dest) == 0 {
		return ErrEmptyArtifactName
	}
	destRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", dest))
	if err != nil {
		return err
	}
	srcRef, err := layout.NewReference(as.storePath, src)
	if err != nil {
		return err
	}
	copyer, err := libimage.NewCopier(&opts, as.SystemContext, nil)
	if err != nil {
		return err
	}
	_, err = copyer.Copy(ctx, srcRef, destRef)
	if err != nil {
		return err
	}
	return copyer.Close()
}

// Add takes one or more local files and adds them to the local artifact store.  The empty
// string input is for possible custom artifact types.
func (as ArtifactStore) Add(ctx context.Context, dest string, paths []string, options *libartTypes.AddOptions) (*digest.Digest, error) {
	if len(dest) == 0 {
		return nil, ErrEmptyArtifactName
	}

	if options.Append && len(options.ArtifactType) > 0 {
		return nil, errors.New("append option is not compatible with ArtifactType option")
	}

	// currently we don't allow override of the filename ; if a user requirement emerges,
	// we could seemingly accommodate but broadens possibilities of something bad happening
	// for things like `artifact extract`
	if _, hasTitle := options.Annotations[specV1.AnnotationTitle]; hasTitle {
		return nil, fmt.Errorf("cannot override filename with %s annotation", specV1.AnnotationTitle)
	}

	// Check if artifact already exists
	artifacts, err := as.getArtifacts(ctx, nil)
	if err != nil {
		return nil, err
	}

	var artifactManifest specV1.Manifest
	var oldDigest *digest.Digest
	fileNames := map[string]struct{}{}

	if !options.Append {
		// Check if artifact exists; in GetByName not getting an
		// error means it exists
		_, _, err := artifacts.GetByNameOrDigest(dest)
		if err == nil {
			return nil, fmt.Errorf("%s: %w", dest, libartTypes.ErrArtifactAlreadyExists)
		}
		artifactManifest = specV1.Manifest{
			Versioned:    specs.Versioned{SchemaVersion: ManifestSchemaVersion},
			MediaType:    specV1.MediaTypeImageManifest,
			ArtifactType: options.ArtifactType,
			// TODO This should probably be configurable once the CLI is capable
			Config: specV1.DescriptorEmptyJSON,
			Layers: make([]specV1.Descriptor, 0),
		}
	} else {
		artifact, _, err := artifacts.GetByNameOrDigest(dest)
		if err != nil {
			return nil, err
		}
		artifactManifest = artifact.Manifest.Manifest
		oldDigest, err = artifact.GetDigest()
		if err != nil {
			return nil, err
		}

		for _, layer := range artifactManifest.Layers {
			if value, ok := layer.Annotations[specV1.AnnotationTitle]; ok && value != "" {
				fileNames[value] = struct{}{}
			}
		}
	}

	for _, path := range paths {
		fileName := filepath.Base(path)
		if _, ok := fileNames[fileName]; ok {
			return nil, fmt.Errorf("%s: %w", fileName, libartTypes.ErrArtifactFileExists)
		}
		fileNames[fileName] = struct{}{}
	}

	ir, err := layout.NewReference(as.storePath, dest)
	if err != nil {
		return nil, err
	}

	imageDest, err := ir.NewImageDestination(ctx, as.SystemContext)
	if err != nil {
		return nil, err
	}
	defer imageDest.Close()

	// ImageDestination, in general, requires the caller to write a full image; here we may write only the added layers.
	// This works for the oci/layout transport we hard-code.
	for _, path := range paths {
		// get the new artifact into the local store
		newBlobDigest, newBlobSize, err := layout.PutBlobFromLocalFile(ctx, imageDest, path)
		if err != nil {
			return nil, err
		}
		detectedType, err := determineManifestType(path)
		if err != nil {
			return nil, err
		}

		annotations := maps.Clone(options.Annotations)
		annotations[specV1.AnnotationTitle] = filepath.Base(path)
		newLayer := specV1.Descriptor{
			MediaType:   detectedType,
			Digest:      newBlobDigest,
			Size:        newBlobSize,
			Annotations: annotations,
		}
		artifactManifest.Layers = append(artifactManifest.Layers, newLayer)
	}

	rawData, err := json.Marshal(artifactManifest)
	if err != nil {
		return nil, err
	}
	if err := imageDest.PutManifest(ctx, rawData, nil); err != nil {
		return nil, err
	}

	unparsed := newUnparsedArtifactImage(ir, artifactManifest)
	if err := imageDest.Commit(ctx, unparsed); err != nil {
		return nil, err
	}

	artifactManifestDigest := digest.FromBytes(rawData)

	// the config is an empty JSON stanza i.e. '{}'; if it does not yet exist, it needs
	// to be created
	if err := createEmptyStanza(filepath.Join(as.storePath, specV1.ImageBlobsDir, artifactManifestDigest.Algorithm().String(), artifactManifest.Config.Digest.Encoded())); err != nil {
		logrus.Errorf("failed to check or write empty stanza file: %v", err)
	}

	// Clean up after append. Remove previous artifact from store.
	if oldDigest != nil {
		lrs, err := layout.List(as.storePath)
		if err != nil {
			return nil, err
		}

		for _, l := range lrs {
			if oldDigest.String() == l.ManifestDescriptor.Digest.String() {
				if _, ok := l.ManifestDescriptor.Annotations[specV1.AnnotationRefName]; ok {
					continue
				}

				if err := l.Reference.DeleteImage(ctx, as.SystemContext); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	return &artifactManifestDigest, nil
}

// Inspect an artifact in a local store
func (as ArtifactStore) Extract(ctx context.Context, nameOrDigest string, target string, options *libartTypes.ExtractOptions) error {
	if len(options.Digest) > 0 && len(options.Title) > 0 {
		return errors.New("cannot specify both digest and title")
	}
	if len(nameOrDigest) == 0 {
		return ErrEmptyArtifactName
	}

	artifacts, err := as.getArtifacts(ctx, nil)
	if err != nil {
		return err
	}

	arty, nameIsDigest, err := artifacts.GetByNameOrDigest(nameOrDigest)
	if err != nil {
		return err
	}
	name := nameOrDigest
	if nameIsDigest {
		name = arty.Name
	}

	if len(arty.Manifest.Layers) == 0 {
		return fmt.Errorf("the artifact has no blobs, nothing to extract")
	}

	ir, err := layout.NewReference(as.storePath, name)
	if err != nil {
		return err
	}
	imgSrc, err := ir.NewImageSource(ctx, as.SystemContext)
	if err != nil {
		return err
	}
	defer imgSrc.Close()

	// check if dest is a dir to know if we can copy more than one blob
	destIsFile := true
	stat, err := os.Stat(target)
	if err == nil {
		destIsFile = !stat.IsDir()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	if destIsFile {
		var digest digest.Digest
		if len(arty.Manifest.Layers) > 1 {
			if len(options.Digest) == 0 && len(options.Title) == 0 {
				return fmt.Errorf("the artifact consists of several blobs and the target %q is not a directory and neither digest or title was specified to only copy a single blob", target)
			}
			digest, err = findDigest(arty, options)
			if err != nil {
				return err
			}
		} else {
			digest = arty.Manifest.Layers[0].Digest
		}

		return copyImageBlobToFile(ctx, imgSrc, digest, target)
	}

	if len(options.Digest) > 0 || len(options.Title) > 0 {
		digest, err := findDigest(arty, options)
		if err != nil {
			return err
		}
		// In case the digest is set we always use it as target name
		// so we do not have to get the actual title annotation form the blob.
		// Passing options.Title is enough because we know it is empty when digest
		// is set as we only allow either one.
		filename, err := generateArtifactBlobName(options.Title, digest)
		if err != nil {
			return err
		}
		return copyImageBlobToFile(ctx, imgSrc, digest, filepath.Join(target, filename))
	}

	for _, l := range arty.Manifest.Layers {
		title := l.Annotations[specV1.AnnotationTitle]
		filename, err := generateArtifactBlobName(title, l.Digest)
		if err != nil {
			return err
		}
		err = copyImageBlobToFile(ctx, imgSrc, l.Digest, filepath.Join(target, filename))
		if err != nil {
			return err
		}
	}

	return nil
}

func generateArtifactBlobName(title string, digest digest.Digest) (string, error) {
	filename := title
	if len(filename) == 0 {
		// No filename given, use the digest. But because ":" is not a valid path char
		// on all platforms replace it with "-".
		filename = strings.ReplaceAll(digest.String(), ":", "-")
	}

	// Important: A potentially malicious artifact could contain a title name with "/"
	// and could try via relative paths such as "../" try to overwrite files on the host
	// the user did not intend. As there is no use for directories in this path we
	// disallow all of them and not try to "make it safe" via securejoin or others.
	// We must use os.IsPathSeparator() as on Windows it checks both "\\" and "/".
	for i := 0; i < len(filename); i++ {
		if os.IsPathSeparator(filename[i]) {
			return "", fmt.Errorf("invalid name: %q cannot contain %c", filename, filename[i])
		}
	}
	return filename, nil
}

func findDigest(arty *libartifact.Artifact, options *libartTypes.ExtractOptions) (digest.Digest, error) {
	var digest digest.Digest
	for _, l := range arty.Manifest.Layers {
		if options.Digest == l.Digest.String() {
			if len(digest.String()) > 0 {
				return digest, fmt.Errorf("more than one match for the digest %q", options.Digest)
			}
			digest = l.Digest
		}
		if len(options.Title) > 0 {
			if val, ok := l.Annotations[specV1.AnnotationTitle]; ok &&
				val == options.Title {
				if len(digest.String()) > 0 {
					return digest, fmt.Errorf("more than one match for the title %q", options.Title)
				}
				digest = l.Digest
			}
		}
	}
	if len(digest.String()) == 0 {
		if len(options.Title) > 0 {
			return digest, fmt.Errorf("no blob with the title %q", options.Title)
		}
		return digest, fmt.Errorf("no blob with the digest %q", options.Digest)
	}
	return digest, nil
}

func copyImageBlobToFile(ctx context.Context, imgSrc types.ImageSource, digest digest.Digest, target string) error {
	src, _, err := imgSrc.GetBlob(ctx, types.BlobInfo{Digest: digest}, nil)
	if err != nil {
		return fmt.Errorf("failed to get artifact file: %w", err)
	}
	defer src.Close()
	dest, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer dest.Close()

	// By default the c/image oci layout API for GetBlob() should always return a os.File in our usage here.
	// And since it is a file we can try to reflink it. In case it is not we should default to the normal copy.
	if file, ok := src.(*os.File); ok {
		return fileutils.ReflinkOrCopy(file, dest)
	}

	_, err = io.Copy(dest, src)
	return err
}

// readIndex is currently unused but I want to keep this around until
// the artifact code is more mature.
func (as ArtifactStore) readIndex() (*specV1.Index, error) { //nolint:unused
	index := specV1.Index{}
	rawData, err := os.ReadFile(as.indexPath())
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(rawData, &index)
	return &index, err
}

func (as ArtifactStore) createEmptyManifest() error {
	index := specV1.Index{
		MediaType: specV1.MediaTypeImageIndex,
		Versioned: specs.Versioned{SchemaVersion: ManifestSchemaVersion},
	}
	rawData, err := json.Marshal(&index)
	if err != nil {
		return err
	}

	return os.WriteFile(as.indexPath(), rawData, 0o644)
}

func (as ArtifactStore) indexPath() string {
	return filepath.Join(as.storePath, specV1.ImageIndexFile)
}

// getArtifacts returns an ArtifactList based on the artifact's store.  The return error and
// unused opts is meant for future growth like filters, etc so the API does not change.
func (as ArtifactStore) getArtifacts(ctx context.Context, _ *libartTypes.GetArtifactOptions) (libartifact.ArtifactList, error) {
	var (
		al libartifact.ArtifactList
	)

	lrs, err := layout.List(as.storePath)
	if err != nil {
		return nil, err
	}
	for _, l := range lrs {
		imgSrc, err := l.Reference.NewImageSource(ctx, as.SystemContext)
		if err != nil {
			return nil, err
		}
		manifest, err := getManifest(ctx, imgSrc)
		imgSrc.Close()
		if err != nil {
			return nil, err
		}
		artifact := libartifact.Artifact{
			Manifest: manifest,
		}
		if val, ok := l.ManifestDescriptor.Annotations[specV1.AnnotationRefName]; ok {
			artifact.SetName(val)
		}

		al = append(al, &artifact)
	}
	return al, nil
}

// getManifest takes an imgSrc and returns the manifest for the imgSrc.
// A OCI index list is not supported and will return an error.
func getManifest(ctx context.Context, imgSrc types.ImageSource) (*manifest.OCI1, error) {
	b, manifestType, err := imgSrc.GetManifest(ctx, nil)
	if err != nil {
		return nil, err
	}

	// We only support a single flat manifest and not an oci index list
	if manifest.MIMETypeIsMultiImage(manifestType) {
		return nil, fmt.Errorf("manifest %q is index list", imgSrc.Reference().StringWithinTransport())
	}

	// parse the single manifest
	mani, err := manifest.OCI1FromManifest(b)
	if err != nil {
		return nil, err
	}
	return mani, nil
}

func createEmptyStanza(path string) error {
	if err := fileutils.Exists(path); err == nil {
		return nil
	}
	return os.WriteFile(path, specV1.DescriptorEmptyJSON.Data, 0644)
}

func determineManifestType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	// DetectContentType looks at the first 512 bytes
	b := make([]byte, 512)
	// Because DetectContentType will return a default value
	// we don't sweat the error
	n, err := f.Read(b)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return http.DetectContentType(b[:n]), nil
}
