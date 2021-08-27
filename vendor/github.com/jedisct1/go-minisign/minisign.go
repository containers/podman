package minisign

import (
	"encoding/base64"
	"errors"
	"io/ioutil"
	"strings"

	"golang.org/x/crypto/ed25519"
)

type PublicKey struct {
	SignatureAlgorithm [2]byte
	KeyId              [8]byte
	PublicKey          [32]byte
}

type Signature struct {
	UntrustedComment   string
	SignatureAlgorithm [2]byte
	KeyId              [8]byte
	Signature          [64]byte
	TrustedComment     string
	GlobalSignature    [64]byte
}

func NewPublicKey(publicKeyStr string) (PublicKey, error) {
	var publicKey PublicKey
	bin, err := base64.StdEncoding.DecodeString(publicKeyStr)
	if err != nil || len(bin) != 42 {
		return publicKey, errors.New("Invalid encoded public key")
	}
	copy(publicKey.SignatureAlgorithm[:], bin[0:2])
	copy(publicKey.KeyId[:], bin[2:10])
	copy(publicKey.PublicKey[:], bin[10:42])
	return publicKey, nil
}

func DecodePublicKey(in string) (PublicKey, error) {
	var publicKey PublicKey
	lines := strings.SplitN(in, "\n", 2)
	if len(lines) < 2 {
		return publicKey, errors.New("Incomplete encoded public key")
	}
	return NewPublicKey(lines[1])
}

func trimCarriageReturn(input string) string {
	return strings.TrimRight(input, "\r")
}

func DecodeSignature(in string) (Signature, error) {
	var signature Signature
	lines := strings.SplitN(in, "\n", 4)
	if len(lines) < 4 {
		return signature, errors.New("Incomplete encoded signature")
	}
	signature.UntrustedComment = trimCarriageReturn(lines[0])
	bin1, err := base64.StdEncoding.DecodeString(lines[1])
	if err != nil || len(bin1) != 74 {
		return signature, errors.New("Invalid encoded signature")
	}
	signature.TrustedComment = trimCarriageReturn(lines[2])
	bin2, err := base64.StdEncoding.DecodeString(lines[3])
	if err != nil || len(bin2) != 64 {
		return signature, errors.New("Invalid encoded signature")
	}
	copy(signature.SignatureAlgorithm[:], bin1[0:2])
	copy(signature.KeyId[:], bin1[2:10])
	copy(signature.Signature[:], bin1[10:74])
	copy(signature.GlobalSignature[:], bin2)
	return signature, nil
}

func NewPublicKeyFromFile(file string) (PublicKey, error) {
	var publicKey PublicKey
	bin, err := ioutil.ReadFile(file)
	if err != nil {
		return publicKey, err
	}
	return DecodePublicKey(string(bin))
}

func NewSignatureFromFile(file string) (Signature, error) {
	var signature Signature
	bin, err := ioutil.ReadFile(file)
	if err != nil {
		return signature, err
	}
	return DecodeSignature(string(bin))
}

func (publicKey *PublicKey) Verify(bin []byte, signature Signature) (bool, error) {
	if publicKey.SignatureAlgorithm != signature.SignatureAlgorithm {
		return false, errors.New("Incompatible signature algorithm")
	}
	if signature.SignatureAlgorithm[0] != 0x45 || signature.SignatureAlgorithm[1] != 0x64 {
		return false, errors.New("Unsupported signature algorithm")
	}
	if publicKey.KeyId != signature.KeyId {
		return false, errors.New("Incompatible key identifiers")
	}
	if !strings.HasPrefix(signature.TrustedComment, "trusted comment: ") {
		return false, errors.New("Unexpected format for the trusted comment")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey.PublicKey[:]), bin, signature.Signature[:]) {
		return false, errors.New("Invalid signature")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey.PublicKey[:]), append(signature.Signature[:], []byte(signature.TrustedComment)[17:]...), signature.GlobalSignature[:]) {
		return false, errors.New("Invalid global signature")
	}
	return true, nil
}

func (publicKey *PublicKey) VerifyFromFile(file string, signature Signature) (bool, error) {
	bin, err := ioutil.ReadFile(file)
	if err != nil {
		return false, err
	}
	return publicKey.Verify(bin, signature)
}
