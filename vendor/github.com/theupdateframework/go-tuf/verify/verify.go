package verify

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/secure-systems-lab/go-securesystemslib/cjson"
	"github.com/theupdateframework/go-tuf/data"
	"github.com/theupdateframework/go-tuf/internal/roles"
	"github.com/theupdateframework/go-tuf/pkg/keys"
)

type signedMeta struct {
	Type    string    `json:"_type"`
	Expires time.Time `json:"expires"`
	Version int64     `json:"version"`
}

// VerifySignature takes a signed JSON message, a signature, and a
// verifier and verifies the given signature on the JSON message
// using the verifier. It returns an error if verification fails.
func VerifySignature(signed json.RawMessage, sig data.HexBytes,
	verifier keys.Verifier) error {
	var decoded map[string]interface{}
	if err := json.Unmarshal(signed, &decoded); err != nil {
		return err
	}
	msg, err := cjson.EncodeCanonical(decoded)
	if err != nil {
		return err
	}
	return verifier.Verify(msg, sig)
}

func (db *DB) VerifyIgnoreExpiredCheck(s *data.Signed, role string, minVersion int64) error {
	if err := db.VerifySignatures(s, role); err != nil {
		return err
	}

	sm := &signedMeta{}
	if err := json.Unmarshal(s.Signed, sm); err != nil {
		return err
	}

	if roles.IsTopLevelRole(role) {
		// Top-level roles can only sign metadata of the same type (e.g. snapshot
		// metadata must be signed by the snapshot role).
		if !strings.EqualFold(sm.Type, role) {
			return ErrWrongMetaType
		}
	} else {
		// Delegated (non-top-level) roles may only sign targets metadata.
		if strings.ToLower(sm.Type) != "targets" {
			return ErrWrongMetaType
		}
	}

	if sm.Version < minVersion {
		return ErrLowVersion{sm.Version, minVersion}
	}

	return nil
}

func (db *DB) Verify(s *data.Signed, role string, minVersion int64) error {
	// Verify signatures and versions
	err := db.VerifyIgnoreExpiredCheck(s, role, minVersion)

	if err != nil {
		return err
	}

	sm := &signedMeta{}
	if err := json.Unmarshal(s.Signed, sm); err != nil {
		return err
	}
	// Verify expiration
	if IsExpired(sm.Expires) {
		return ErrExpired{sm.Expires}
	}

	return nil
}

var IsExpired = func(t time.Time) bool {
	return time.Until(t) <= 0
}

func (db *DB) VerifySignatures(s *data.Signed, role string) error {
	if len(s.Signatures) == 0 {
		return ErrNoSignatures
	}

	roleData := db.GetRole(role)
	if roleData == nil {
		return ErrUnknownRole{role}
	}

	// Verify that a threshold of keys signed the data. Since keys can have
	// multiple key ids, we need to protect against multiple attached
	// signatures that just differ on the key id.
	verifiedKeyIDs := make(map[string]struct{})
	numVerifiedKeys := 0
	for _, sig := range s.Signatures {
		if !roleData.ValidKey(sig.KeyID) {
			continue
		}
		verifier, err := db.GetVerifier(sig.KeyID)
		if err != nil {
			continue
		}

		if err := VerifySignature(s.Signed, sig.Signature, verifier); err != nil {
			// If a signature fails verification, don't count it towards the
			// threshold but also return early and error out immediately.
			// Note: Because of this, it is impossible to distinguish between
			// an error of an invalid signature and a threshold not achieved.
			// Invalid signatures lead to not achieving the threshold.
			continue
		}

		// Only consider this key valid if we haven't seen any of it's
		// key ids before.
		// Careful: we must not rely on the key IDs _declared in the file_,
		// instead we get to decide what key IDs this key correspond to.
		// XXX dangerous; better stop supporting multiple key IDs altogether.
		keyIDs := verifier.MarshalPublicKey().IDs()
		wasKeySeen := false
		for _, keyID := range keyIDs {
			if _, present := verifiedKeyIDs[keyID]; present {
				wasKeySeen = true
			}
		}
		if !wasKeySeen {
			for _, id := range keyIDs {
				verifiedKeyIDs[id] = struct{}{}
			}

			numVerifiedKeys++
		}
	}
	if numVerifiedKeys < roleData.Threshold {
		return ErrRoleThreshold{roleData.Threshold, numVerifiedKeys}
	}

	return nil
}

func (db *DB) Unmarshal(b []byte, v interface{}, role string, minVersion int64) error {
	s := &data.Signed{}
	if err := json.Unmarshal(b, s); err != nil {
		return err
	}
	if err := db.Verify(s, role, minVersion); err != nil {
		return err
	}
	return json.Unmarshal(s.Signed, v)
}

// UnmarshalExpired is exactly like Unmarshal except ignores expired timestamp error.
func (db *DB) UnmarshalIgnoreExpired(b []byte, v interface{}, role string, minVersion int64) error {
	s := &data.Signed{}
	if err := json.Unmarshal(b, s); err != nil {
		return err
	}
	// Note: If verification fails, then we wont attempt to unmarshal
	// unless when verification error is errExpired.
	verifyErr := db.Verify(s, role, minVersion)
	if verifyErr != nil {
		if _, ok := verifyErr.(ErrExpired); !ok {
			return verifyErr
		}
	}
	return json.Unmarshal(s.Signed, v)
}

func (db *DB) UnmarshalTrusted(b []byte, v interface{}, role string) error {
	s := &data.Signed{}
	if err := json.Unmarshal(b, s); err != nil {
		return err
	}
	if err := db.VerifySignatures(s, role); err != nil {
		return err
	}
	return json.Unmarshal(s.Signed, v)
}
