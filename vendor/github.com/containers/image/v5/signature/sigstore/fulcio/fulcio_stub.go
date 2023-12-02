//go:build containers_image_fulcio_stub
// +build containers_image_fulcio_stub

package fulcio

import (
	"fmt"
	"io"
	"net/url"

	"github.com/containers/image/v5/signature/sigstore/internal"
)

func WithFulcioAndPreexistingOIDCIDToken(fulcioURL *url.URL, oidcIDToken string) internal.Option {
	return func(s *internal.SigstoreSigner) error {
		return fmt.Errorf("fulcio disabled at compile time")
	}
}

// WithFulcioAndDeviceAuthorizationGrantOIDC sets up signing to use a short-lived key and a Fulcio-issued certificate
// based on an OIDC ID token obtained using a device authorization grant (RFC 8628).
//
// interactiveOutput must be directly accessible to a human user in real time (i.e. not be just a log file).
func WithFulcioAndDeviceAuthorizationGrantOIDC(fulcioURL *url.URL, oidcIssuerURL *url.URL, oidcClientID, oidcClientSecret string,
	interactiveOutput io.Writer) internal.Option {
	return func(s *internal.SigstoreSigner) error {
		return fmt.Errorf("fulcio disabled at compile time")
	}
}

// WithFulcioAndInterativeOIDC sets up signing to use a short-lived key and a Fulcio-issued certificate
// based on an interactively-obtained OIDC ID token.
// The token is obtained
//   - directly using a browser, listening on localhost, automatically opening a browser to the OIDC issuer,
//     to be redirected on localhost. (I.e. the current environment must allow launching a browser that connect back to the current process;
//     either or both may be impossible in a container or a remote VM).
//   - or by instructing the user to manually open a browser, obtain the OIDC code, and interactively input it as text.
//
// interactiveInput and interactiveOutput must both be directly operable by a human user in real time (i.e. not be just a log file).
func WithFulcioAndInteractiveOIDC(fulcioURL *url.URL, oidcIssuerURL *url.URL, oidcClientID, oidcClientSecret string,
	interactiveInput io.Reader, interactiveOutput io.Writer) internal.Option {
	return func(s *internal.SigstoreSigner) error {
		return fmt.Errorf("fulcio disabled at compile time")
	}
}
