package copy

import (
	"fmt"
	"strings"

	"github.com/containers/image/v5/types"
	"github.com/containers/ocicrypt"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// isOciEncrypted returns a bool indicating if a mediatype is encrypted
// This function will be moved to be part of OCI spec when adopted.
func isOciEncrypted(mediatype string) bool {
	return strings.HasSuffix(mediatype, "+encrypted")
}

// isEncrypted checks if an image is encrypted
func isEncrypted(i types.Image) bool {
	layers := i.LayerInfos()
	for _, l := range layers {
		if isOciEncrypted(l.MediaType) {
			return true
		}
	}
	return false
}

// bpDecryptionStepData contains data that the copy pipeline needs about the decryption step.
type bpDecryptionStepData struct {
	decrypting bool // We are actually decrypting the stream
}

// blobPipelineDecryptionStep updates *stream to decrypt if, it necessary.
// srcInfo is only used for error messages.
// Returns data for other steps; the caller should eventually use updateCryptoOperation.
func (c *copier) blobPipelineDecryptionStep(stream *sourceStream, srcInfo types.BlobInfo) (*bpDecryptionStepData, error) {
	if isOciEncrypted(stream.info.MediaType) && c.ociDecryptConfig != nil {
		desc := imgspecv1.Descriptor{
			Annotations: stream.info.Annotations,
		}
		reader, decryptedDigest, err := ocicrypt.DecryptLayer(c.ociDecryptConfig, stream.reader, desc, false)
		if err != nil {
			return nil, fmt.Errorf("decrypting layer %s: %w", srcInfo.Digest, err)
		}

		stream.reader = reader
		stream.info.Digest = decryptedDigest
		stream.info.Size = -1
		for k := range stream.info.Annotations {
			if strings.HasPrefix(k, "org.opencontainers.image.enc") {
				delete(stream.info.Annotations, k)
			}
		}
		return &bpDecryptionStepData{
			decrypting: true,
		}, nil
	}
	return &bpDecryptionStepData{
		decrypting: false,
	}, nil
}

// updateCryptoOperation sets *operation, if necessary.
func (d *bpDecryptionStepData) updateCryptoOperation(operation *types.LayerCrypto) {
	if d.decrypting {
		*operation = types.Decrypt
	}
}

// bpdData contains data that the copy pipeline needs about the encryption step.
type bpEncryptionStepData struct {
	encrypting bool // We are actually encrypting the stream
	finalizer  ocicrypt.EncryptLayerFinalizer
}

// blobPipelineEncryptionStep updates *stream to encrypt if, it required by toEncrypt.
// srcInfo is primarily used for error messages.
// Returns data for other steps; the caller should eventually call updateCryptoOperationAndAnnotations.
func (c *copier) blobPipelineEncryptionStep(stream *sourceStream, toEncrypt bool, srcInfo types.BlobInfo,
	decryptionStep *bpDecryptionStepData) (*bpEncryptionStepData, error) {
	if toEncrypt && !isOciEncrypted(srcInfo.MediaType) && c.ociEncryptConfig != nil {
		var annotations map[string]string
		if !decryptionStep.decrypting {
			annotations = srcInfo.Annotations
		}
		desc := imgspecv1.Descriptor{
			MediaType:   srcInfo.MediaType,
			Digest:      srcInfo.Digest,
			Size:        srcInfo.Size,
			Annotations: annotations,
		}
		reader, finalizer, err := ocicrypt.EncryptLayer(c.ociEncryptConfig, stream.reader, desc)
		if err != nil {
			return nil, fmt.Errorf("encrypting blob %s: %w", srcInfo.Digest, err)
		}

		stream.reader = reader
		stream.info.Digest = ""
		stream.info.Size = -1
		return &bpEncryptionStepData{
			encrypting: true,
			finalizer:  finalizer,
		}, nil
	}
	return &bpEncryptionStepData{
		encrypting: false,
	}, nil
}

// updateCryptoOperationAndAnnotations sets *operation and updates *annotations, if necessary.
func (d *bpEncryptionStepData) updateCryptoOperationAndAnnotations(operation *types.LayerCrypto, annotations *map[string]string) error {
	if !d.encrypting {
		return nil
	}

	encryptAnnotations, err := d.finalizer()
	if err != nil {
		return fmt.Errorf("Unable to finalize encryption: %w", err)
	}
	*operation = types.Encrypt
	if *annotations == nil {
		*annotations = map[string]string{}
	}
	for k, v := range encryptAnnotations {
		(*annotations)[k] = v
	}
	return nil
}
