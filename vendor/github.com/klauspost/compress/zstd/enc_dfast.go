// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

const (
	dFastLongTableBits = 17                      // Bits used in the long match table
	dFastLongTableSize = 1 << dFastLongTableBits // Size of the table
	dFastLongTableMask = dFastLongTableSize - 1  // Mask for table indices. Redundant, but can eliminate bounds checks.

	dFastShortTableBits = tableBits                // Bits used in the short match table
	dFastShortTableSize = 1 << dFastShortTableBits // Size of the table
	dFastShortTableMask = dFastShortTableSize - 1  // Mask for table indices. Redundant, but can eliminate bounds checks.
)

type doubleFastEncoder struct {
	fastEncoder
	longTable [dFastLongTableSize]tableEntry
}

// Encode mimmics functionality in zstd_dfast.c
func (e *doubleFastEncoder) Encode(blk *blockEnc, src []byte) {
	const (
		// Input margin is the number of bytes we read (8)
		// and the maximum we will read ahead (2)
		inputMargin            = 8 + 2
		minNonLiteralBlockSize = 16
	)

	// Protect against e.cur wraparound.
	for e.cur > (1<<30)+e.maxMatchOff {
		if len(e.hist) == 0 {
			for i := range e.table[:] {
				e.table[i] = tableEntry{}
			}
			for i := range e.longTable[:] {
				e.longTable[i] = tableEntry{}
			}
			e.cur = e.maxMatchOff
			break
		}
		// Shift down everything in the table that isn't already too far away.
		minOff := e.cur + int32(len(e.hist)) - e.maxMatchOff
		for i := range e.table[:] {
			v := e.table[i].offset
			if v < minOff {
				v = 0
			} else {
				v = v - e.cur + e.maxMatchOff
			}
			e.table[i].offset = v
		}
		for i := range e.longTable[:] {
			v := e.longTable[i].offset
			if v < minOff {
				v = 0
			} else {
				v = v - e.cur + e.maxMatchOff
			}
			e.longTable[i].offset = v
		}
		e.cur = e.maxMatchOff
	}

	s := e.addBlock(src)
	blk.size = len(src)
	if len(src) < minNonLiteralBlockSize {
		blk.extraLits = len(src)
		blk.literals = blk.literals[:len(src)]
		copy(blk.literals, src)
		return
	}

	// Override src
	src = e.hist
	sLimit := int32(len(src)) - inputMargin
	// stepSize is the number of bytes to skip on every main loop iteration.
	// It should be >= 1.
	stepSize := int32(e.o.targetLength)
	if stepSize == 0 {
		stepSize++
	}

	// TEMPLATE

	const kSearchStrength = 8

	// nextEmit is where in src the next emitLiteral should start from.
	nextEmit := s
	cv := load6432(src, s)
	// nextHash is the hash at s
	nextHashS := hash5(cv, dFastShortTableBits)
	nextHashL := hash8(cv, dFastLongTableBits)

	// Relative offsets
	offset1 := int32(blk.recentOffsets[0])
	offset2 := int32(blk.recentOffsets[1])

	addLiterals := func(s *seq, until int32) {
		if until == nextEmit {
			return
		}
		blk.literals = append(blk.literals, src[nextEmit:until]...)
		s.litLen = uint32(until - nextEmit)
	}
	if debug {
		println("recent offsets:", blk.recentOffsets)
	}

encodeLoop:
	for {
		var t int32
		// We allow the encoder to optionally turn off repeat offsets across blocks
		canRepeat := len(blk.sequences) > 2

		for {
			if debug && canRepeat && offset1 == 0 {
				panic("offset0 was 0")
			}

			nextHashS = nextHashS & dFastShortTableMask
			nextHashL = nextHashL & dFastLongTableMask
			candidateL := e.longTable[nextHashL]
			candidateS := e.table[nextHashS]

			const repOff = 1
			repIndex := s - offset1 + repOff
			entry := tableEntry{offset: s + e.cur, val: uint32(cv)}
			e.longTable[nextHashL] = entry
			e.table[nextHashS] = entry

			if canRepeat {
				if repIndex >= 0 && load3232(src, repIndex) == uint32(cv>>(repOff*8)) {
					// Consider history as well.
					var seq seq
					lenght := 4 + e.matchlen(s+4+repOff, repIndex+4, src)

					seq.matchLen = uint32(lenght - zstdMinMatch)

					// We might be able to match backwards.
					// Extend as long as we can.
					start := s + repOff
					// We end the search early, so we don't risk 0 literals
					// and have to do special offset treatment.
					startLimit := nextEmit + 1

					tMin := s - e.maxMatchOff
					if tMin < 0 {
						tMin = 0
					}
					for repIndex > tMin && start > startLimit && src[repIndex-1] == src[start-1] && seq.matchLen < maxMatchLength-zstdMinMatch-1 {
						repIndex--
						start--
						seq.matchLen++
					}
					addLiterals(&seq, start)

					// rep 0
					seq.offset = 1
					if debugSequences {
						println("repeat sequence", seq, "next s:", s)
					}
					blk.sequences = append(blk.sequences, seq)
					s += lenght + repOff
					nextEmit = s
					if s >= sLimit {
						if debug {
							println("repeat ended", s, lenght)

						}
						break encodeLoop
					}
					cv = load6432(src, s)
					nextHashS = hash5(cv, dFastShortTableBits)
					nextHashL = hash8(cv, dFastLongTableBits)
					continue
				}
				const repOff2 = 1
				// We deviate from the reference encoder and also check offset 2.
				// Slower and not consistently better, so disabled.
				// repIndex = s - offset2 + repOff2
				if false && repIndex >= 0 && load3232(src, repIndex) == uint32(cv>>(repOff2*8)) {
					// Consider history as well.
					var seq seq
					lenght := 4 + e.matchlen(s+4+repOff2, repIndex+4, src)

					seq.matchLen = uint32(lenght - zstdMinMatch)

					// We might be able to match backwards.
					// Extend as long as we can.
					start := s + repOff2
					// We end the search early, so we don't risk 0 literals
					// and have to do special offset treatment.
					startLimit := nextEmit + 1

					tMin := s - e.maxMatchOff
					if tMin < 0 {
						tMin = 0
					}
					for repIndex > tMin && start > startLimit && src[repIndex-1] == src[start-1] && seq.matchLen < maxMatchLength-zstdMinMatch-1 {
						repIndex--
						start--
						seq.matchLen++
					}
					addLiterals(&seq, start)

					// rep 2
					seq.offset = 2
					if debugSequences {
						println("repeat sequence 2", seq, "next s:", s)
					}
					blk.sequences = append(blk.sequences, seq)
					s += lenght + repOff2
					nextEmit = s
					if s >= sLimit {
						if debug {
							println("repeat ended", s, lenght)

						}
						break encodeLoop
					}
					cv = load6432(src, s)
					nextHashS = hash5(cv, dFastShortTableBits)
					nextHashL = hash8(cv, dFastLongTableBits)
					// Swap offsets
					offset1, offset2 = offset2, offset1
					continue
				}
			}
			// Find the offsets of our two matches.
			coffsetL := s - (candidateL.offset - e.cur)
			coffsetS := s - (candidateS.offset - e.cur)

			// Check if we have a long match.
			if coffsetL < e.maxMatchOff && uint32(cv) == candidateL.val {
				// Found a long match, likely at least 8 bytes.
				// Reference encoder checks all 8 bytes, we only check 4,
				// but the likelihood of both the first 4 bytes and the hash matching should be enough.
				t = candidateL.offset - e.cur
				if debug && s <= t {
					panic("s <= t")
				}
				if debug && s-t > e.maxMatchOff {
					panic("s - t >e.maxMatchOff")
				}
				if debug {
					println("long match")
				}
				break
			}

			// Check if we have a short match.
			if coffsetS < e.maxMatchOff && uint32(cv) == candidateS.val {
				// found a regular match
				// See if we can find a long match at s+1
				const checkAt = 1
				cv := load6432(src, s+checkAt)
				nextHashL = hash8(cv, dFastLongTableBits)
				candidateL = e.longTable[nextHashL]
				coffsetL = s - (candidateL.offset - e.cur) + checkAt

				// We can store it, since we have at least a 4 byte match.
				e.longTable[nextHashL] = tableEntry{offset: s + checkAt + e.cur, val: uint32(cv)}
				if coffsetL < e.maxMatchOff && uint32(cv) == candidateL.val {
					// Found a long match, likely at least 8 bytes.
					// Reference encoder checks all 8 bytes, we only check 4,
					// but the likelihood of both the first 4 bytes and the hash matching should be enough.
					t = candidateL.offset - e.cur
					s += checkAt
					if debug {
						println("long match (after short)")
					}
					break
				}

				t = candidateS.offset - e.cur
				if debug && s <= t {
					panic("s <= t")
				}
				if debug && s-t > e.maxMatchOff {
					panic("s - t >e.maxMatchOff")
				}
				if debug && t < 0 {
					panic("t<0")
				}
				if debug {
					println("short match")
				}
				break
			}

			// No match found, move forward in input.
			s += stepSize + ((s - nextEmit) >> (kSearchStrength - 1))
			if s >= sLimit {
				break encodeLoop
			}
			cv = load6432(src, s)
			nextHashS = hash5(cv, dFastShortTableBits)
			nextHashL = hash8(cv, dFastLongTableBits)
		}

		// A 4-byte match has been found. Update recent offsets.
		// We'll later see if more than 4 bytes.
		offset2 = offset1
		offset1 = s - t

		if debug && s <= t {
			panic("s <= t")
		}

		if debug && canRepeat && int(offset1) > len(src) {
			panic("invalid offset")
		}

		// Extend the 4-byte match as long as possible.
		l := e.matchlen(s+4, t+4, src) + 4

		// Extend backwards
		tMin := s - e.maxMatchOff
		if tMin < 0 {
			tMin = 0
		}
		for t > tMin && s > nextEmit && src[t-1] == src[s-1] && l < maxMatchLength {
			s--
			t--
			l++
		}

		// Write our sequence
		var seq seq
		seq.litLen = uint32(s - nextEmit)
		seq.matchLen = uint32(l - zstdMinMatch)
		if seq.litLen > 0 {
			blk.literals = append(blk.literals, src[nextEmit:s]...)
		}
		seq.offset = uint32(s-t) + 3
		s += l
		if debugSequences {
			println("sequence", seq, "next s:", s)
		}
		blk.sequences = append(blk.sequences, seq)
		nextEmit = s
		if s >= sLimit {
			break encodeLoop
		}

		// Index match start + 2 and end - 2
		index0 := s - l + 2
		index1 := s - 2
		if l == 4 {
			// if l is 4, we would check the same place twice, so index s-1 instead.
			index1++
		}

		cv0 := load6432(src, index0)
		cv1 := load6432(src, index1)
		entry0 := tableEntry{offset: index0 + e.cur, val: uint32(cv0)}
		entry1 := tableEntry{offset: index1 + e.cur, val: uint32(cv1)}
		e.table[hash5(cv0, dFastShortTableBits)&dFastShortTableMask] = entry0
		e.longTable[hash8(cv0, dFastLongTableBits)&dFastLongTableMask] = entry0
		e.table[hash5(cv1, dFastShortTableBits)&dFastShortTableMask] = entry1
		e.longTable[hash8(cv1, dFastLongTableBits)&dFastLongTableMask] = entry1

		cv = load6432(src, s)
		nextHashS = hash5(cv, dFastShortTableBits)
		nextHashL = hash8(cv, dFastLongTableBits)

		// Check offset 2
		if o2 := s - offset2; canRepeat && o2 > 0 && load3232(src, o2) == uint32(cv) {
			// We have at least 4 byte match.
			// No need to check backwards. We come straight from a match
			l := 4 + e.matchlen(s+4, o2+4, src)
			// Store this, since we have it.
			entry := tableEntry{offset: s + e.cur, val: uint32(cv)}
			e.longTable[nextHashL&dFastLongTableMask] = entry
			e.table[nextHashS&dFastShortTableMask] = entry
			seq.matchLen = uint32(l) - zstdMinMatch
			seq.litLen = 0
			// Since litlen is always 0, this is offset 1.
			seq.offset = 1
			s += l
			nextEmit = s
			if debugSequences {
				println("sequence", seq, "next s:", s)
			}
			blk.sequences = append(blk.sequences, seq)

			// Swap offset 1 and 2.
			offset1, offset2 = offset2, offset1
			if s >= sLimit {
				break encodeLoop
			}
			// Prepare next loop.
			cv = load6432(src, s)
			nextHashS = hash5(cv, dFastShortTableBits)
			nextHashL = hash8(cv, dFastLongTableBits)
		}
	}

	if int(nextEmit) < len(src) {
		blk.literals = append(blk.literals, src[nextEmit:]...)
		blk.extraLits = len(src) - int(nextEmit)
	}
	blk.recentOffsets[0] = uint32(offset1)
	blk.recentOffsets[1] = uint32(offset2)
	if debug {
		println("returning, recent offsets:", blk.recentOffsets, "extra literals:", blk.extraLits)
	}
}
