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
	ref           ociReference
	descriptor    imgspecv1.Descriptor
	client        *http.Client
	sharedBlobDir string
}

// newImageSource returns an ImageSource for reading from an existing directory.
func newImageSource(sys *types.SystemContext, ref ociReference) (types.ImageSource, error) {
	tr := tlsclientconfig.NewTransport()
	tr.TLSClientConfig = tlsconfig.ServerDefault()

	if sys != nil && sys.OCICertPath != "" {
		if err := tlsclientconfig.SetupCertificates(sys.OCICertPath, tr.TLSClientConfig); err != nil {
			return nil, err
		}
		tr.TLSClientConfig.InsecureSkipVerify = sys.OCIInsecureSkipTLSVerify
	}

	client := &http.Client{}
	client.Transport = tr
	descriptor, err := ref.getManifestDescriptor()
	if err != nil {
		return nil, err
	}
	d := &ociImageSource{ref: ref, descriptor: descriptor, client: client}
	if sys != nil {
		// TODO(jonboulle): check dir existence?
		d.sharedBlobDir = sys.OCISharedBlobDirPath
	}
	return d, nil
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
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve (when the primary manifest is a manifest list);
// this never happens if the primary manifest is not a manifest list (e.g. if the source never returns manifest lists).
func (s *ociImageSource) GetManifest(ctx context.Context, instanceDigest *digest.Digest) ([]byte, string, error) {
	var dig digest.Digest
	var mimeType string
	if instanceDigest == nil {
		dig = digest.Digest(s.descriptor.Digest)
		mimeType = s.descriptor.MediaType
	} else {
		dig = *instanceDigest
		// XXX: instanceDigest means that we don't immediately have the context of what
		//      mediaType the manifest has. In OCI this means that we don't know
		//      what reference it came from, so we just *assume* that its
		//      MediaTypeImageManifest.
		// FIXME: We should actually be able to look up the manifest in the index,
		// and see the MIME type there.
		mimeType = imgspecv1.MediaTypeImageManifest
	}

	manifestPath, err := s.ref.blobPath(dig, s.sharedBlobDir)
	if err != nil {
		return nil, "", err
	}
	m, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return nil, "", err
	}

	return m, mimeType, nil
}

// HasThreadSafeGetBlob indicates whether GetBlob can be executed concurrently.
func (s *ociImageSource) HasThreadSafeGetBlob() bool {
	return false
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *ociImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	if len(info.URLs) != 0 {
		return s.getExternalBlob(ctx, info.URLs)
	}

	path, err := s.ref.blobPath(info.Digest, s.sharedBlobDir)
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

// GetSignatures returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (s *ociImageSource) GetSignatures(ctx context.Context, instanceDigest *digest.Digest) ([][]byte, error) {
	return [][]byte{}, nil
}

func (s *ociImageSource) getExternalBlob(ctx context.Context, urls []string) (io.ReadCloser, int64, error) {
	errWrap := errors.New("failed fetching external blob from all urls")
	for _, url := range urls {

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			errWrap = errors.Wrapf(errWrap, "fetching %s failed %s", url, err.Error())
			continue
		}

		resp, err := s.client.Do(req.WithContext(ctx))
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

// LayerInfosForCopy() returns updated layer info that should be used when reading, in preference to values in the manifest, if specified.
func (s *ociImageSource) LayerInfosForCopy(ctx context.Context) ([]types.BlobInfo, error) {
	return nil, nil
}

func getBlobSize(resp *http.Response) int64 {
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	return size
}
