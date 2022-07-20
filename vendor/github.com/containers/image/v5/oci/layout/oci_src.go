package layout

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/containers/image/v5/internal/imagesource/impl"
	"github.com/containers/image/v5/internal/imagesource/stubs"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/pkg/tlsclientconfig"
	"github.com/containers/image/v5/types"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type ociImageSource struct {
	impl.Compat
	impl.PropertyMethodsInitialize
	impl.NoSignatures
	impl.DoesNotAffectLayerInfosForCopy
	stubs.NoGetBlobAtInitialize

	ref           ociReference
	index         *imgspecv1.Index
	descriptor    imgspecv1.Descriptor
	client        *http.Client
	sharedBlobDir string
}

// newImageSource returns an ImageSource for reading from an existing directory.
func newImageSource(sys *types.SystemContext, ref ociReference) (private.ImageSource, error) {
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
	index, err := ref.getIndex()
	if err != nil {
		return nil, err
	}
	s := &ociImageSource{
		PropertyMethodsInitialize: impl.PropertyMethods(impl.Properties{
			HasThreadSafeGetBlob: false,
		}),
		NoGetBlobAtInitialize: stubs.NoGetBlobAt(ref),

		ref:        ref,
		index:      index,
		descriptor: descriptor,
		client:     client,
	}
	if sys != nil {
		// TODO(jonboulle): check dir existence?
		s.sharedBlobDir = sys.OCISharedBlobDirPath
	}
	s.Compat = impl.AddCompat(s)
	return s, nil
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
	var err error

	if instanceDigest == nil {
		dig = digest.Digest(s.descriptor.Digest)
		mimeType = s.descriptor.MediaType
	} else {
		dig = *instanceDigest
		for _, md := range s.index.Manifests {
			if md.Digest == dig {
				mimeType = md.MediaType
				break
			}
		}
	}

	manifestPath, err := s.ref.blobPath(dig, s.sharedBlobDir)
	if err != nil {
		return nil, "", err
	}

	m, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, "", err
	}
	if mimeType == "" {
		mimeType = manifest.GuessMIMEType(m)
	}

	return m, mimeType, nil
}

// GetBlob returns a stream for the specified blob, and the blob’s size (or -1 if unknown).
// The Digest field in BlobInfo is guaranteed to be provided, Size may be -1 and MediaType may be optionally provided.
// May update BlobInfoCache, preferably after it knows for certain that a blob truly exists at a specific location.
func (s *ociImageSource) GetBlob(ctx context.Context, info types.BlobInfo, cache types.BlobInfoCache) (io.ReadCloser, int64, error) {
	if len(info.URLs) != 0 {
		r, s, err := s.getExternalBlob(ctx, info.URLs)
		if err != nil {
			return nil, 0, err
		} else if r != nil {
			return r, s, nil
		}
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

// getExternalBlob returns the reader of the first available blob URL from urls, which must not be empty.
// This function can return nil reader when no url is supported by this function. In this case, the caller
// should fallback to fetch the non-external blob (i.e. pull from the registry).
func (s *ociImageSource) getExternalBlob(ctx context.Context, urls []string) (io.ReadCloser, int64, error) {
	if len(urls) == 0 {
		return nil, 0, errors.New("internal error: getExternalBlob called with no URLs")
	}

	errWrap := errors.New("failed fetching external blob from all urls")
	hasSupportedURL := false
	for _, u := range urls {
		if u, err := url.Parse(u); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			continue // unsupported url. skip this url.
		}
		hasSupportedURL = true
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			errWrap = fmt.Errorf("fetching %s failed %s: %w", u, err.Error(), errWrap)
			continue
		}

		resp, err := s.client.Do(req)
		if err != nil {
			errWrap = fmt.Errorf("fetching %s failed %s: %w", u, err.Error(), errWrap)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			errWrap = fmt.Errorf("fetching %s failed, response code not 200: %w", u, errWrap)
			continue
		}

		return resp.Body, getBlobSize(resp), nil
	}
	if !hasSupportedURL {
		return nil, 0, nil // fallback to non-external blob
	}

	return nil, 0, errWrap
}

func getBlobSize(resp *http.Response) int64 {
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		size = -1
	}
	return size
}
