package libimage

import (
	"context"
	"net/url"
	"os"

	storageTransport "github.com/containers/image/v5/storage"
	tarballTransport "github.com/containers/image/v5/tarball"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ImportOptions allow for customizing image imports.
type ImportOptions struct {
	CopyOptions

	// Apply the specified changes to the created image. Please refer to
	// `ImageConfigFromChanges` for supported change instructions.
	Changes []string
	// Set the commit message as a comment to created image's history.
	CommitMessage string
	// Tag the imported image with this value.
	Tag string
}

// Import imports a custom tarball at the specified path.  Returns the name of
// the imported image.
func (r *Runtime) Import(ctx context.Context, path string, options *ImportOptions) (string, error) {
	logrus.Debugf("Importing image from %q", path)

	if options == nil {
		options = &ImportOptions{}
	}

	ic := v1.ImageConfig{}
	if len(options.Changes) > 0 {
		config, err := ImageConfigFromChanges(options.Changes)
		if err != nil {
			return "", err
		}
		ic = config.ImageConfig
	}

	hist := []v1.History{
		{Comment: options.CommitMessage},
	}

	config := v1.Image{
		Config:  ic,
		History: hist,
	}

	u, err := url.ParseRequestURI(path)
	if err == nil && u.Scheme != "" {
		// If source is a URL, download the file.
		file, err := r.downloadFromURL(path)
		if err != nil {
			return "", err
		}
		defer os.Remove(file)
		path = file
	} else if path == "-" {
		// "-" special cases stdin
		path = os.Stdin.Name()
	}

	srcRef, err := tarballTransport.Transport.ParseReference(path)
	if err != nil {
		return "", err
	}

	updater, ok := srcRef.(tarballTransport.ConfigUpdater)
	if !ok {
		return "", errors.New("unexpected type, a tarball reference should implement tarball.ConfigUpdater")
	}
	annotations := make(map[string]string)
	if err := updater.ConfigUpdate(config, annotations); err != nil {
		return "", err
	}

	name := options.Tag
	if name == "" {
		name, err = getImageDigest(ctx, srcRef, r.systemContextCopy())
		if err != nil {
			return "", err
		}
		name = "sha256:" + name[1:] // strip leading "@"
	}

	destRef, err := storageTransport.Transport.ParseStoreReference(r.store, name)
	if err != nil {
		return "", err
	}

	c, err := r.newCopier(&options.CopyOptions)
	if err != nil {
		return "", err
	}
	defer c.close()

	if _, err := c.copy(ctx, srcRef, destRef); err != nil {
		return "", err
	}

	return name, nil
}
