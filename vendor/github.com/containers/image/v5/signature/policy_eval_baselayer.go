// Policy evaluation for prSignedBaseLayer.

package signature

import (
	"context"

	"github.com/containers/image/v5/internal/private"
	"github.com/sirupsen/logrus"
)

func (pr *prSignedBaseLayer) isSignatureAuthorAccepted(ctx context.Context, image private.UnparsedImage, sig []byte) (signatureAcceptanceResult, *Signature, error) {
	return sarUnknown, nil, nil
}

func (pr *prSignedBaseLayer) isRunningImageAllowed(ctx context.Context, image private.UnparsedImage) (bool, error) {
	// FIXME? Reject this at policy parsing time already?
	logrus.Errorf("signedBaseLayer not implemented yet!")
	return false, PolicyRequirementError("signedBaseLayer not implemented yet!")
}
