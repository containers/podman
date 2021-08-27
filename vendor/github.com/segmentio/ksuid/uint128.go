package ksuid

import "fmt"

// uint128 represents an unsigned 128 bits little endian integer.
type uint128 [2]uint64

func uint128Payload(ksuid KSUID) uint128 {
	return makeUint128FromPayload(ksuid[timestampLengthInBytes:])
}

func makeUint128(high uint64, low uint64) uint128 {
	return uint128{low, high}
}

func makeUint128FromPayload(payload []byte) uint128 {
	return uint128{
		// low
		uint64(payload[8])<<56 |
			uint64(payload[9])<<48 |
			uint64(payload[10])<<40 |
			uint64(payload[11])<<32 |
			uint64(payload[12])<<24 |
			uint64(payload[13])<<16 |
			uint64(payload[14])<<8 |
			uint64(payload[15]),
		// high
		uint64(payload[0])<<56 |
			uint64(payload[1])<<48 |
			uint64(payload[2])<<40 |
			uint64(payload[3])<<32 |
			uint64(payload[4])<<24 |
			uint64(payload[5])<<16 |
			uint64(payload[6])<<8 |
			uint64(payload[7]),
	}
}

func (v uint128) ksuid(timestamp uint32) KSUID {
	return KSUID{
		// time
		byte(timestamp >> 24),
		byte(timestamp >> 16),
		byte(timestamp >> 8),
		byte(timestamp),

		// high
		byte(v[1] >> 56),
		byte(v[1] >> 48),
		byte(v[1] >> 40),
		byte(v[1] >> 32),
		byte(v[1] >> 24),
		byte(v[1] >> 16),
		byte(v[1] >> 8),
		byte(v[1]),

		// low
		byte(v[0] >> 56),
		byte(v[0] >> 48),
		byte(v[0] >> 40),
		byte(v[0] >> 32),
		byte(v[0] >> 24),
		byte(v[0] >> 16),
		byte(v[0] >> 8),
		byte(v[0]),
	}
}

func (v uint128) bytes() [16]byte {
	return [16]byte{
		// high
		byte(v[1] >> 56),
		byte(v[1] >> 48),
		byte(v[1] >> 40),
		byte(v[1] >> 32),
		byte(v[1] >> 24),
		byte(v[1] >> 16),
		byte(v[1] >> 8),
		byte(v[1]),

		// low
		byte(v[0] >> 56),
		byte(v[0] >> 48),
		byte(v[0] >> 40),
		byte(v[0] >> 32),
		byte(v[0] >> 24),
		byte(v[0] >> 16),
		byte(v[0] >> 8),
		byte(v[0]),
	}
}

func (v uint128) String() string {
	return fmt.Sprintf("0x%016X%016X", v[0], v[1])
}

const wordBitSize = 64

func cmp128(x, y uint128) int {
	if x[1] < y[1] {
		return -1
	}
	if x[1] > y[1] {
		return 1
	}
	if x[0] < y[0] {
		return -1
	}
	if x[0] > y[0] {
		return 1
	}
	return 0
}

func add128(x, y uint128) (z uint128) {
	x0 := x[0]
	y0 := y[0]
	z0 := x0 + y0
	z[0] = z0

	c := (x0&y0 | (x0|y0)&^z0) >> (wordBitSize - 1)

	z[1] = x[1] + y[1] + c
	return
}

func sub128(x, y uint128) (z uint128) {
	x0 := x[0]
	y0 := y[0]
	z0 := x0 - y0
	z[0] = z0

	c := (y0&^x0 | (y0|^x0)&z0) >> (wordBitSize - 1)

	z[1] = x[1] - y[1] - c
	return
}

func incr128(x uint128) uint128 {
	return add128(x, uint128{1, 0})
}
