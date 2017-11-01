package common

// SigningOptions encapsulates settings that control whether or not we strip or
// add signatures to images when writing them.
type SigningOptions struct {
	// RemoveSignatures directs us to remove any signatures which are already present.
	RemoveSignatures bool
	// SignBy is a key identifier of some kind, indicating that a signature should be generated using the specified private key and stored with the image.
	SignBy string
}
