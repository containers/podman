//go:build containers_image_rekor_stub
// +build containers_image_rekor_stub

package internal

import (
	"crypto/ecdsa"
	"time"
)

// VerifyRekorSET verifies that unverifiedRekorSET is correctly signed by publicKey and matches the rest of the data.
// Returns bundle upload time on success.
func VerifyRekorSET(publicKey *ecdsa.PublicKey, unverifiedRekorSET []byte, unverifiedKeyOrCertBytes []byte, unverifiedBase64Signature string, unverifiedPayloadBytes []byte) (time.Time, error) {
	return time.Time{}, NewInvalidSignatureError("rekor disabled at compile-time")
}
