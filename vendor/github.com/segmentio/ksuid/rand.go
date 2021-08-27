package ksuid

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"io"
	"math/rand"
)

// FastRander is an io.Reader that uses math/rand and is optimized for
// generating 16 bytes KSUID payloads. It is intended to be used as a
// performance improvements for programs that have no need for
// cryptographically secure KSUIDs and are generating a lot of them.
var FastRander = newRBG()

func newRBG() io.Reader {
	r, err := newRandomBitsGenerator()
	if err != nil {
		panic(err)
	}
	return r
}

func newRandomBitsGenerator() (r io.Reader, err error) {
	var seed int64

	if seed, err = readCryptoRandomSeed(); err != nil {
		return
	}

	r = &randSourceReader{source: rand.NewSource(seed).(rand.Source64)}
	return
}

func readCryptoRandomSeed() (seed int64, err error) {
	var b [8]byte

	if _, err = io.ReadFull(cryptoRand.Reader, b[:]); err != nil {
		return
	}

	seed = int64(binary.LittleEndian.Uint64(b[:]))
	return
}

type randSourceReader struct {
	source rand.Source64
}

func (r *randSourceReader) Read(b []byte) (int, error) {
	// optimized for generating 16 bytes payloads
	binary.LittleEndian.PutUint64(b[:8], r.source.Uint64())
	binary.LittleEndian.PutUint64(b[8:], r.source.Uint64())
	return 16, nil
}
