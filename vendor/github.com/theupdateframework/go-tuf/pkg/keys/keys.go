package keys

import (
	"errors"
	"fmt"
	"sync"

	"github.com/theupdateframework/go-tuf/data"
)

// MaxJSONKeySize defines the maximum length of a JSON payload.
const MaxJSONKeySize = 512 * 1024 // 512Kb

// SignerMap stores mapping between key type strings and signer constructors.
var SignerMap sync.Map

// Verifier stores mapping between key type strings and verifier constructors.
var VerifierMap sync.Map

var (
	ErrInvalid    = errors.New("tuf: signature verification failed")
	ErrInvalidKey = errors.New("invalid key")
)

// A Verifier verifies public key signatures.
type Verifier interface {
	// UnmarshalPublicKey takes key data to a working verifier implementation for the key type.
	// This performs any validation over the data.PublicKey to ensure that the verifier is usable
	// to verify signatures.
	UnmarshalPublicKey(key *data.PublicKey) error

	// MarshalPublicKey returns the data.PublicKey object associated with the verifier.
	MarshalPublicKey() *data.PublicKey

	// This is the public string used as a unique identifier for the verifier instance.
	Public() string

	// Verify takes a message and signature, all as byte slices,
	// and determines whether the signature is valid for the given
	// key and message.
	Verify(msg, sig []byte) error
}

type Signer interface {
	// MarshalPrivateKey returns the private key data.
	MarshalPrivateKey() (*data.PrivateKey, error)

	// UnmarshalPrivateKey takes private key data to a working Signer implementation for the key type.
	UnmarshalPrivateKey(key *data.PrivateKey) error

	// Returns the public data.PublicKey from the private key
	PublicData() *data.PublicKey

	// Sign returns the signature of the message.
	// The signer is expected to do its own hashing, so the full message will be
	// provided as the message to Sign with a zero opts.HashFunc().
	SignMessage(message []byte) ([]byte, error)
}

func GetVerifier(key *data.PublicKey) (Verifier, error) {
	st, ok := VerifierMap.Load(key.Type)
	if !ok {
		return nil, ErrInvalidKey
	}
	s := st.(func() Verifier)()
	if err := s.UnmarshalPublicKey(key); err != nil {
		return nil, fmt.Errorf("tuf: error unmarshalling key: %w", err)
	}
	return s, nil
}

func GetSigner(key *data.PrivateKey) (Signer, error) {
	st, ok := SignerMap.Load(key.Type)
	if !ok {
		return nil, ErrInvalidKey
	}
	s := st.(func() Signer)()
	if err := s.UnmarshalPrivateKey(key); err != nil {
		return nil, fmt.Errorf("tuf: error unmarshalling key: %w", err)
	}
	return s, nil
}
