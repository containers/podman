package detach

import (
	"bytes"
	"errors"
	"io"
)

// ErrDetach indicates that an attach session was manually detached by
// the user.
var ErrDetach = errors.New("detached from container")

// Copy is similar to io.Copy but support a detach key sequence to break out.
func Copy(dst io.Writer, src io.Reader, keys []byte) (written int64, err error) {
	// if no key sequence we can use the fast std lib implementation
	if len(keys) == 0 {
		return io.Copy(dst, src)
	}
	buf := make([]byte, 32*1024)

	// When key 1 is on one read and the key2 on the second read we cannot do a normal full match in the buffer.
	// Thus we use this index to store where in keys we matched on the end of the previous buffer.
	keySequenceIndex := 0
outer:
	for {
		nr, er := src.Read(buf)
		// Do not check error right away, i.e. if we have EOF this code still must flush the last partial key sequence first.
		// Previous key index, if the last buffer ended with the start of the key sequence
		// then we must continue looking here.
		if keySequenceIndex > 0 {
			bytesToCheck := min(nr, len(keys)-keySequenceIndex)
			if bytes.Equal(buf[:bytesToCheck], keys[keySequenceIndex:keySequenceIndex+bytesToCheck]) {
				if keySequenceIndex+bytesToCheck == len(keys) {
					// we are done
					return written, ErrDetach
				}
				// still not at the end of the sequence, must continue to read
				keySequenceIndex += bytesToCheck
				continue outer
			}
			// No match, write buffered keys now
			nw, ew := dst.Write(keys[:keySequenceIndex])
			if ew != nil {
				return written, ew
			}
			written += int64(nw)
			keySequenceIndex = 0
		}

		// Now we can handle and return the error.
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, err
		}

		// Check buffer from 0 to end - sequence length (after that there cannot be a full match),
		// then walk the entire buffer and try to perform a full sequence match.
		readMinusKeys := nr - len(keys)
		for i := range readMinusKeys {
			if bytes.Equal(buf[i:i+len(keys)], keys) {
				if i > 0 {
					nw, ew := dst.Write(buf[:i])
					if ew != nil {
						return written, ew
					}
					written += int64(nw)
				}
				return written, ErrDetach
			}
		}

		// Now read the rest of the buffer to the end and perform a partial match on the sequence.
		// Note that readMinusKeys can be < 0 on reads smaller than sequence length. Thus we must
		// ensure it is at least 0 otherwise the index access will cause a panic.
		for i := max(readMinusKeys, 0); i < nr; i++ {
			if bytes.Equal(buf[i:nr], keys[:nr-i]) {
				nw, ew := dst.Write(buf[:i])
				if ew != nil {
					return written, ew
				}
				written += int64(nw)
				keySequenceIndex = nr - i
				continue outer
			}
		}

		nw, ew := dst.Write(buf[:nr])
		if ew != nil {
			return written, ew
		}
		written += int64(nw)
	}
}
