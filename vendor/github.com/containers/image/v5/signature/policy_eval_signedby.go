// Policy evaluation for prSignedBy.

package signature

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/containers/image/v5/internal/multierr"
	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/manifest"
	digest "github.com/opencontainers/go-digest"
)

func (pr *prSignedBy) isSignatureAuthorAccepted(ctx context.Context, image private.UnparsedImage, sig []byte) (signatureAcceptanceResult, *Signature, error) {
	switch pr.KeyType {
	case SBKeyTypeGPGKeys:
	case SBKeyTypeSignedByGPGKeys, SBKeyTypeX509Certificates, SBKeyTypeSignedByX509CAs:
		// FIXME? Reject this at policy parsing time already?
		return sarRejected, nil, fmt.Errorf(`Unimplemented "keyType" value %q`, string(pr.KeyType))
	default:
		// This should never happen, newPRSignedBy ensures KeyType.IsValid()
		return sarRejected, nil, fmt.Errorf(`Unknown "keyType" value %q`, string(pr.KeyType))
	}

	// FIXME: move this to per-context initialization
	const notOneSourceErrorText = `Internal inconsistency: not exactly one of "keyPath", "keyPaths" and "keyData" specified`
	data, err := loadBytesFromConfigSources(configBytesSources{
		inconsistencyErrorMessage: notOneSourceErrorText,
		path:                      pr.KeyPath,
		paths:                     pr.KeyPaths,
		data:                      pr.KeyData,
	})
	if err != nil {
		return sarRejected, nil, err
	}
	if data == nil {
		return sarRejected, nil, errors.New(notOneSourceErrorText)
	}

	// FIXME: move this to per-context initialization
	mech, trustedIdentities, err := newEphemeralGPGSigningMechanism(data)
	if err != nil {
		return sarRejected, nil, err
	}
	defer mech.Close()
	if len(trustedIdentities) == 0 {
		return sarRejected, nil, PolicyRequirementError("No public keys imported")
	}

	signature, err := verifyAndExtractSignature(mech, sig, signatureAcceptanceRules{
		validateKeyIdentity: func(keyIdentity string) error {
			if slices.Contains(trustedIdentities, keyIdentity) {
				return nil
			}
			// Coverage: We use a private GPG home directory and only import trusted keys, so this should
			// not be reachable.
			return PolicyRequirementError(fmt.Sprintf("Signature by key %s is not accepted", keyIdentity))
		},
		validateSignedDockerReference: func(ref string) error {
			if !pr.SignedIdentity.matchesDockerReference(image, ref) {
				return PolicyRequirementError(fmt.Sprintf("Signature for identity %q is not accepted", ref))
			}
			return nil
		},
		validateSignedDockerManifestDigest: func(digest digest.Digest) error {
			m, _, err := image.Manifest(ctx)
			if err != nil {
				return err
			}
			digestMatches, err := manifest.MatchesDigest(m, digest)
			if err != nil {
				return err
			}
			if !digestMatches {
				return PolicyRequirementError(fmt.Sprintf("Signature for digest %s does not match", digest))
			}
			return nil
		},
	})
	if err != nil {
		return sarRejected, nil, err
	}

	return sarAccepted, signature, nil
}

func (pr *prSignedBy) isRunningImageAllowed(ctx context.Context, image private.UnparsedImage) (bool, error) {
	// FIXME: Use image.UntrustedSignatures, use that to improve error messages
	// (needs tests!)
	sigs, err := image.Signatures(ctx)
	if err != nil {
		return false, err
	}
	var rejections []error
	for _, s := range sigs {
		var reason error
		switch res, _, err := pr.isSignatureAuthorAccepted(ctx, image, s); res {
		case sarAccepted:
			// One accepted signature is enough.
			return true, nil
		case sarRejected:
			reason = err
		case sarUnknown:
			// Huh?! This should not happen at all; treat it as any other invalid value.
			fallthrough
		default:
			reason = fmt.Errorf(`Internal error: Unexpected signature verification result %q`, string(res))
		}
		rejections = append(rejections, reason)
	}
	var summary error
	switch len(rejections) {
	case 0:
		summary = PolicyRequirementError("A signature was required, but no signature exists")
	case 1:
		summary = rejections[0]
	default:
		summary = PolicyRequirementError(multierr.Format("None of the signatures were accepted, reasons: ", "; ", "", rejections).Error())
	}
	return false, summary
}
