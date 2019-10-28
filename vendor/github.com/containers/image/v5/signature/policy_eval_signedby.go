// Policy evaluation for prSignedBy.

package signature

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func (pr *prSignedBy) isSignatureAuthorAccepted(ctx context.Context, image types.UnparsedImage, sig []byte) (signatureAcceptanceResult, *Signature, error) {
	switch pr.KeyType {
	case SBKeyTypeGPGKeys:
	case SBKeyTypeSignedByGPGKeys, SBKeyTypeX509Certificates, SBKeyTypeSignedByX509CAs:
		// FIXME? Reject this at policy parsing time already?
		return sarRejected, nil, errors.Errorf(`"Unimplemented "keyType" value "%s"`, string(pr.KeyType))
	default:
		// This should never happen, newPRSignedBy ensures KeyType.IsValid()
		return sarRejected, nil, errors.Errorf(`"Unknown "keyType" value "%s"`, string(pr.KeyType))
	}

	if pr.KeyPath != "" && pr.KeyData != nil {
		return sarRejected, nil, errors.New(`Internal inconsistency: both "keyPath" and "keyData" specified`)
	}
	// FIXME: move this to per-context initialization
	var data []byte
	if pr.KeyData != nil {
		data = pr.KeyData
	} else {
		d, err := ioutil.ReadFile(pr.KeyPath)
		if err != nil {
			return sarRejected, nil, err
		}
		data = d
	}

	// FIXME: move this to per-context initialization
	mech, trustedIdentities, err := NewEphemeralGPGSigningMechanism(data)
	if err != nil {
		return sarRejected, nil, err
	}
	defer mech.Close()
	if len(trustedIdentities) == 0 {
		return sarRejected, nil, PolicyRequirementError("No public keys imported")
	}

	signature, err := verifyAndExtractSignature(mech, sig, signatureAcceptanceRules{
		validateKeyIdentity: func(keyIdentity string) error {
			for _, trustedIdentity := range trustedIdentities {
				if keyIdentity == trustedIdentity {
					return nil
				}
			}
			// Coverage: We use a private GPG home directory and only import trusted keys, so this should
			// not be reachable.
			return PolicyRequirementError(fmt.Sprintf("Signature by key %s is not accepted", keyIdentity))
		},
		validateSignedDockerReference: func(ref string) error {
			if !pr.SignedIdentity.matchesDockerReference(image, ref) {
				return PolicyRequirementError(fmt.Sprintf("Signature for identity %s is not accepted", ref))
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

func (pr *prSignedBy) isRunningImageAllowed(ctx context.Context, image types.UnparsedImage) (bool, error) {
	// FIXME: pass context.Context
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
			reason = errors.Errorf(`Internal error: Unexpected signature verification result "%s"`, string(res))
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
		var msgs []string
		for _, e := range rejections {
			msgs = append(msgs, e.Error())
		}
		summary = PolicyRequirementError(fmt.Sprintf("None of the signatures were accepted, reasons: %s",
			strings.Join(msgs, "; ")))
	}
	return false, summary
}
