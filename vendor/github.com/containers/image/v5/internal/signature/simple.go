package signature

// SimpleSigning is a “simple signing” signature.
type SimpleSigning struct {
	untrustedSignature []byte
}

// SimpleSigningFromBlob converts a “simple signing” signature into a SimpleSigning object.
func SimpleSigningFromBlob(blobChunk []byte) SimpleSigning {
	return SimpleSigning{
		untrustedSignature: copyByteSlice(blobChunk),
	}
}

func (s SimpleSigning) FormatID() FormatID {
	return SimpleSigningFormat
}

// blobChunk returns a representation of signature as a []byte, suitable for long-term storage.
// Almost everyone should use signature.Blob() instead.
func (s SimpleSigning) blobChunk() ([]byte, error) {
	return copyByteSlice(s.untrustedSignature), nil
}

func (s SimpleSigning) UntrustedSignature() []byte {
	return copyByteSlice(s.untrustedSignature)
}
