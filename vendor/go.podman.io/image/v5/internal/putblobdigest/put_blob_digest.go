package putblobdigest

import (
	"io"

	"github.com/opencontainers/go-digest"
	"go.podman.io/image/v5/types"
)

// Digester computes a digest of the provided stream, if not known yet.
type Digester struct {
	knownDigest digest.Digest   // Or ""
	digester    digest.Digester // Or nil
}

// newDigester initiates computation of a digest of stream using the same algorithm as knownDigest if available,
// if !validDigest; otherwise it just records knownDigest to be returned later.
// The caller MUST use the returned stream instead of the original value.
func newDigester(stream io.Reader, knownDigest digest.Digest, validDigest bool) (Digester, io.Reader) {
	if validDigest {
		return Digester{knownDigest: knownDigest}, stream
	} else {
		// Use the algorithm from knownDigest if available and valid, otherwise fall back to canonical
		algorithm := digest.Canonical
		if knownDigest != "" {
			if algo := knownDigest.Algorithm(); algo.Available() {
				algorithm = algo
			}
		}
		res := Digester{
			digester: algorithm.Digester(),
		}
		stream = io.TeeReader(stream, res.digester.Hash())
		return res, stream
	}
}

// DigestIfUnknown initiates computation of a digest of stream using the same algorithm as the provided digest,
// if no digest is supplied in the provided blobInfo; otherwise blobInfo.Digest will
// be used (accepting any algorithm).
// The caller MUST use the returned stream instead of the original value.
func DigestIfUnknown(stream io.Reader, blobInfo types.BlobInfo) (Digester, io.Reader) {
	d := blobInfo.Digest
	return newDigester(stream, d, d != "")
}

// DigestIfCanonicalUnknown initiates computation of a digest using the same algorithm as the provided digest,
// if a digest.Canonical digest is not supplied in the provided blobInfo;
// otherwise blobInfo.Digest will be used.
// The caller MUST use the returned stream instead of the original value.
func DigestIfCanonicalUnknown(stream io.Reader, blobInfo types.BlobInfo) (Digester, io.Reader) {
	d := blobInfo.Digest
	return newDigester(stream, d, d != "" && d.Algorithm() == digest.Canonical)
}

// Digest() returns a digest value possibly computed by Digester.
// This must be called only after all of the stream returned by a Digester constructor
// has been successfully read.
func (d Digester) Digest() digest.Digest {
	if d.digester != nil {
		return d.digester.Digest()
	}
	return d.knownDigest
}
