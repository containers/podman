//go:build gofuzz
// +build gofuzz

// Fuzz test harness.  To run:
// go-fuzz-build capnproto.org/go/capnp/v3/internal/packed
// go-fuzz -bin=packed-fuzz.zip -workdir=internal/packed/testdata

package packed

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
)

func Fuzz(data []byte) int {
	result := 0

	// Unpacked
	if unpacked, err := Unpack(nil, data); err == nil {
		checkRepack(unpacked)
		result = 1
	}

	// Read
	{
		r := NewReader(bufio.NewReader(bytes.NewReader(data)))
		if unpacked, err := ioutil.ReadAll(r); err == nil {
			checkRepack(unpacked)
			result = 1
		}
	}

	// ReadWord
	{
		r := NewReader(bufio.NewReader(bytes.NewReader(data)))
		var unpacked []byte
		var err error
		for {
			n := len(unpacked)
			unpacked = append(unpacked, 0, 0, 0, 0, 0, 0, 0, 0)
			if err = r.ReadWord(unpacked[n:]); err != nil {
				unpacked = unpacked[:n]
				break
			}
		}
		if err == io.EOF {
			checkRepack(unpacked)
			result = 1
		}
	}

	return result
}

func checkRepack(unpacked []byte) {
	packed := Pack(nil, unpacked)
	unpacked2, err := Unpack(nil, packed)
	if err != nil {
		panic("correctness: unpack, pack, unpack gives error: " + err.Error())
	}
	if !bytes.Equal(unpacked, unpacked2) {
		panic("correctness: unpack, pack, unpack gives different results")
	}
}
