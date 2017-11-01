package layout

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/containers/image/pkg/tlsclientconfig"
	"github.com/containers/image/types"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type ociImageSource struct {
	ref        ociReference
	descriptor imgspecv1.Descriptor
	client     *http.Client
}

// newImageSource returns an ImageSource for reading from an existing directory.
func newImageSource(ctx *types.SystemContext, ref ociReference) (types.ImageSource, error) {
	tr := tlsclientconfig.NewTransport()
	tr.TLSClientConfig = tlsconfig.ServerDefault()

	if ctx != nil && ctx.OCICertPath != "" {
		if err := tlsclientconfig.SetupCertificates(ctx.OCICertPath, tr.TLSClientConfig); err != nil {
			return nil, err
		}
		tr.TLSClientConfig.InsecureSkipVerify = ctx.OCIInsecureSkipTLSVerify
	}

	client := &http.Client{}
	client.Transport = tr
	descriptor, err := ref.getManifestDescriptor()
	if err != nil {
		return nil, err
	}
	return &ociImageSource{ref: ref, descriptor: descriptor, client: client}, nil
}

// Reference returns the reference used to set up this source.
func (s *ociImageSource) Reference() types.ImageReference {
	return s.ref
}

// Close removes resources associated with an initialized ImageSource, if any.
func (s *ociImageSource) Close() error {
	return nil
}

// GetManifest returns the image's manifest along with its MIME type (which may be empty when it can't be determined but the manifest is available).
// It may use a remote (= slow) service.
func (s *ociImageSource) GetManifest() ([]byte, string, error) {
	manifestPath, err := s.ref.blobPath(digest.Digest(s.descriptor.Digest))
	if err != nil {
		return nil, "", err
	}
	m, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, "", err
	}

	return m, s.descriptor.MediaType, nil
}

func (s *ociImageSource) GetTargetManifest(digest digest.Digest) ([]byte, string, error) {
	manifestPath, err := s.ref.blobPath(digest)
	if err != nil {
		return nil, "", err
	}

	m, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, "", err
	}

	// XXX: GetTargetManifest means that we don't have the context of what
	//      mediaType the manifest has. In OCI this means that we don't know
	//      what reference it came from, so we just *assume* that its
	//      MediaTypeImageManifest.
	return m, imgspecv1.MediaTypeImageManifest, nil
}

// GetBlob returns a stream for the specified blob, and the blob's size.
func (s *ociImageSource) GetBlob(info types.BlobInfo) (io.ReadCloser, int64, error) {
	if len(info.URLs) != 0 {
		return s.getExternalBlob(info.URLs)
	}

	path, err := s.ref.blobPath(info.Digest)
	if err != nil {
		return nil, 0, err
	}

	r, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	fi, err := r.Stat()
	if err != nil {
		return nil, 0, err
	}
	return r, fi.Size(), nil
}

func (s *ociImageSource) GetSignatures(context.Context) ([][]byte, error) {
	return [][]byte{}, nil
}

func (s *ociImageSource) getExternalBlob(urls []string) (io.ReadCloser, int64, error) {
	errWrap := errors.New("failed fetching external blob from all urls")
	for _, url := range urls {
		resp, err := s.client.Get(url)
		if err != nil {
			errWrap = errors.Wrapf(errWrap, "fetching %s failed %s", url, err.Error())
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			errWrap = errors.Wrapf(errWrap, "fetching %s failed, response code not 200", url)
			continue
		}

		return resp.Body, getBlobSize(resp), nil
	}

	return nil, 0, errWrap
}

// UpdatedLayerInfos() returns updated layer info that should be used when reading, in preference to values in the manifest, if specified.
func (s *ociImageSource) UpdatedLayerInfos() []types.BlobInfo {
	return nil
}

func getBlobSize(resp *http.Response) int64 {
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	return size
}
