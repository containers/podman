//go:build containers_image_rekor_stub
// +build containers_image_rekor_stub

package rekor

import (
	"fmt"
	"net/url"

	signerInternal "github.com/containers/image/v5/signature/sigstore/internal"
)

func WithRekor(rekorURL *url.URL) signerInternal.Option {
	return func(s *signerInternal.SigstoreSigner) error {
		return fmt.Errorf("rekor disabled at build time")
	}
}
