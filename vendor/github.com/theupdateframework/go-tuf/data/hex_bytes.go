package data

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

type HexBytes []byte

func (b *HexBytes) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || len(data)%2 != 0 || data[0] != '"' || data[len(data)-1] != '"' {
		return errors.New("tuf: invalid JSON hex bytes")
	}
	res := make([]byte, hex.DecodedLen(len(data)-2))
	_, err := hex.Decode(res, data[1:len(data)-1])
	if err != nil {
		return err
	}
	*b = res
	return nil
}

func (b HexBytes) MarshalJSON() ([]byte, error) {
	res := make([]byte, hex.EncodedLen(len(b))+2)
	res[0] = '"'
	res[len(res)-1] = '"'
	hex.Encode(res[1:], b)
	return res, nil
}

func (b HexBytes) String() string {
	return hex.EncodeToString(b)
}

// 4.5. File formats: targets.json and delegated target roles:
// ...each target path, when hashed with the SHA-256 hash function to produce
// a 64-byte hexadecimal digest (HEX_DIGEST)...
func PathHexDigest(s string) string {
	b := sha256.Sum256([]byte(s))
	return hex.EncodeToString(b[:])
}
