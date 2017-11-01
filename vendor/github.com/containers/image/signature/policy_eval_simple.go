// Policy evaluation for the various simple PolicyRequirement types.

package signature

import (
	"fmt"

	"github.com/containers/image/transports"
	"github.com/containers/image/types"
)

func (pr *prInsecureAcceptAnything) isSignatureAuthorAccepted(image types.UnparsedImage, sig []byte) (signatureAcceptanceResult, *Signature, error) {
	// prInsecureAcceptAnything semantics: Every image is allowed to run,
	// but this does not consider the signature as verified.
	return sarUnknown, nil, nil
}

func (pr *prInsecureAcceptAnything) isRunningImageAllowed(image types.UnparsedImage) (bool, error) {
	return true, nil
}

func (pr *prReject) isSignatureAuthorAccepted(image types.UnparsedImage, sig []byte) (signatureAcceptanceResult, *Signature, error) {
	return sarRejected, nil, PolicyRequirementError(fmt.Sprintf("Any signatures for image %s are rejected by policy.", transports.ImageName(image.Reference())))
}

func (pr *prReject) isRunningImageAllowed(image types.UnparsedImage) (bool, error) {
	return false, PolicyRequirementError(fmt.Sprintf("Running image %s is rejected by policy.", transports.ImageName(image.Reference())))
}
