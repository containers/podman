package compression

import (
	"bytes"
	"errors"
	"io"
)

type state int

const (
	zerosThreshold = 1024

	stateData = iota
	stateZeros
)

type WriteSeekCloser interface {
	io.Closer
	io.WriteSeeker
}

type sparseWriter struct {
	state      state
	file       WriteSeekCloser
	zeros      int64
	lastIsZero bool
}

func NewSparseWriter(file WriteSeekCloser) *sparseWriter {
	return &sparseWriter{
		file:       file,
		state:      stateData,
		zeros:      0,
		lastIsZero: false,
	}
}

func (sw *sparseWriter) createHole() error {
	zeros := sw.zeros
	if zeros == 0 {
		return nil
	}
	sw.zeros = 0
	sw.lastIsZero = true
	_, err := sw.file.Seek(zeros, io.SeekCurrent)
	return err
}

func findFirstNotZero(b []byte) int {
	for i, v := range b {
		if v != 0 {
			return i
		}
	}
	return -1
}

// Write writes data to the file, creating holes for long sequences of zeros.
func (sw *sparseWriter) Write(data []byte) (int, error) {
	written, current := 0, 0
	totalLen := len(data)
	for current < len(data) {
		switch sw.state {
		case stateData:
			nextZero := bytes.IndexByte(data[current:], 0)
			if nextZero < 0 {
				_, err := sw.file.Write(data[written:])
				sw.lastIsZero = false
				return totalLen, err
			} else {
				current += nextZero
				sw.state = stateZeros
			}
		case stateZeros:
			nextNonZero := findFirstNotZero(data[current:])
			if nextNonZero < 0 {
				// finish with a zero, flush any data and keep track of the zeros
				if written != current {
					if _, err := sw.file.Write(data[written:current]); err != nil {
						return -1, err
					}
					sw.lastIsZero = false
				}
				sw.zeros += int64(len(data) - current)
				return totalLen, nil
			}
			// do not bother with too short sequences
			if sw.zeros == 0 && nextNonZero < zerosThreshold {
				sw.state = stateData
				current += nextNonZero
				continue
			}
			if written != current {
				if _, err := sw.file.Write(data[written:current]); err != nil {
					return -1, err
				}
				sw.lastIsZero = false
			}
			sw.zeros += int64(nextNonZero)
			current += nextNonZero
			if err := sw.createHole(); err != nil {
				return -1, err
			}
			written = current
		}
	}
	return totalLen, nil
}

// Close closes the SparseWriter's underlying file.
func (sw *sparseWriter) Close() error {
	if sw.file == nil {
		return errors.New("file is already closed")
	}
	if err := sw.createHole(); err != nil {
		sw.file.Close()
		return err
	}
	if sw.lastIsZero {
		if _, err := sw.file.Seek(-1, io.SeekCurrent); err != nil {
			sw.file.Close()
			return err
		}
		if _, err := sw.file.Write([]byte{0}); err != nil {
			sw.file.Close()
			return err
		}
	}
	err := sw.file.Close()
	sw.file = nil
	return err
}
