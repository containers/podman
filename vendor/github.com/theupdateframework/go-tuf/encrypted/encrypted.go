// Package encrypted provides a simple, secure system for encrypting data
// symmetrically with a passphrase.
//
// It uses scrypt derive a key from the passphrase and the NaCl secret box
// cipher for authenticated encryption.
package encrypted

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/crypto/scrypt"
)

const saltSize = 32

const (
	boxKeySize   = 32
	boxNonceSize = 24
)

const (
	// N parameter was chosen to be ~100ms of work using the default implementation
	// on the 2.3GHz Core i7 Haswell processor in a late-2013 Apple Retina Macbook
	// Pro (it takes ~113ms).
	scryptN = 32768
	scryptR = 8
	scryptP = 1
)

const (
	nameScrypt    = "scrypt"
	nameSecretBox = "nacl/secretbox"
)

type data struct {
	KDF        scryptKDF       `json:"kdf"`
	Cipher     secretBoxCipher `json:"cipher"`
	Ciphertext []byte          `json:"ciphertext"`
}

type scryptParams struct {
	N int `json:"N"`
	R int `json:"r"`
	P int `json:"p"`
}

func newScryptKDF() (scryptKDF, error) {
	salt := make([]byte, saltSize)
	if err := fillRandom(salt); err != nil {
		return scryptKDF{}, err
	}
	return scryptKDF{
		Name: nameScrypt,
		Params: scryptParams{
			N: scryptN,
			R: scryptR,
			P: scryptP,
		},
		Salt: salt,
	}, nil
}

type scryptKDF struct {
	Name   string       `json:"name"`
	Params scryptParams `json:"params"`
	Salt   []byte       `json:"salt"`
}

func (s *scryptKDF) Key(passphrase []byte) ([]byte, error) {
	return scrypt.Key(passphrase, s.Salt, s.Params.N, s.Params.R, s.Params.P, boxKeySize)
}

// CheckParams checks that the encoded KDF parameters are what we expect them to
// be. If we do not do this, an attacker could cause a DoS by tampering with
// them.
func (s *scryptKDF) CheckParams() error {
	if s.Params.N != scryptN || s.Params.R != scryptR || s.Params.P != scryptP {
		return errors.New("encrypted: unexpected kdf parameters")
	}
	return nil
}

func newSecretBoxCipher() (secretBoxCipher, error) {
	nonce := make([]byte, boxNonceSize)
	if err := fillRandom(nonce); err != nil {
		return secretBoxCipher{}, err
	}
	return secretBoxCipher{
		Name:  nameSecretBox,
		Nonce: nonce,
	}, nil
}

type secretBoxCipher struct {
	Name  string `json:"name"`
	Nonce []byte `json:"nonce"`

	encrypted bool
}

func (s *secretBoxCipher) Encrypt(plaintext, key []byte) []byte {
	var keyBytes [boxKeySize]byte
	var nonceBytes [boxNonceSize]byte

	if len(key) != len(keyBytes) {
		panic("incorrect key size")
	}
	if len(s.Nonce) != len(nonceBytes) {
		panic("incorrect nonce size")
	}

	copy(keyBytes[:], key)
	copy(nonceBytes[:], s.Nonce)

	// ensure that we don't re-use nonces
	if s.encrypted {
		panic("Encrypt must only be called once for each cipher instance")
	}
	s.encrypted = true

	return secretbox.Seal(nil, plaintext, &nonceBytes, &keyBytes)
}

func (s *secretBoxCipher) Decrypt(ciphertext, key []byte) ([]byte, error) {
	var keyBytes [boxKeySize]byte
	var nonceBytes [boxNonceSize]byte

	if len(key) != len(keyBytes) {
		panic("incorrect key size")
	}
	if len(s.Nonce) != len(nonceBytes) {
		// return an error instead of panicking since the nonce is user input
		return nil, errors.New("encrypted: incorrect nonce size")
	}

	copy(keyBytes[:], key)
	copy(nonceBytes[:], s.Nonce)

	res, ok := secretbox.Open(nil, ciphertext, &nonceBytes, &keyBytes)
	if !ok {
		return nil, errors.New("encrypted: decryption failed")
	}
	return res, nil
}

// Encrypt takes a passphrase and plaintext, and returns a JSON object
// containing ciphertext and the details necessary to decrypt it.
func Encrypt(plaintext, passphrase []byte) ([]byte, error) {
	k, err := newScryptKDF()
	if err != nil {
		return nil, err
	}
	key, err := k.Key(passphrase)
	if err != nil {
		return nil, err
	}

	c, err := newSecretBoxCipher()
	if err != nil {
		return nil, err
	}

	data := &data{
		KDF:    k,
		Cipher: c,
	}
	data.Ciphertext = c.Encrypt(plaintext, key)

	return json.Marshal(data)
}

// Marshal encrypts the JSON encoding of v using passphrase.
func Marshal(v interface{}, passphrase []byte) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return nil, err
	}
	return Encrypt(data, passphrase)
}

// Decrypt takes a JSON-encoded ciphertext object encrypted using Encrypt and
// tries to decrypt it using passphrase. If successful, it returns the
// plaintext.
func Decrypt(ciphertext, passphrase []byte) ([]byte, error) {
	data := &data{}
	if err := json.Unmarshal(ciphertext, data); err != nil {
		return nil, err
	}

	if data.KDF.Name != nameScrypt {
		return nil, fmt.Errorf("encrypted: unknown kdf name %q", data.KDF.Name)
	}
	if data.Cipher.Name != nameSecretBox {
		return nil, fmt.Errorf("encrypted: unknown cipher name %q", data.Cipher.Name)
	}
	if err := data.KDF.CheckParams(); err != nil {
		return nil, err
	}

	key, err := data.KDF.Key(passphrase)
	if err != nil {
		return nil, err
	}

	return data.Cipher.Decrypt(data.Ciphertext, key)
}

// Unmarshal decrypts the data using passphrase and unmarshals the resulting
// plaintext into the value pointed to by v.
func Unmarshal(data []byte, v interface{}, passphrase []byte) error {
	decrypted, err := Decrypt(data, passphrase)
	if err != nil {
		return err
	}
	return json.Unmarshal(decrypted, v)
}

func fillRandom(b []byte) error {
	_, err := io.ReadFull(rand.Reader, b)
	return err
}
