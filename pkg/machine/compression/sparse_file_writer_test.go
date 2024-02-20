package compression

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type memorySparseFile struct {
	buffer bytes.Buffer
	pos    int64
}

func (m *memorySparseFile) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = m.pos + offset
	case io.SeekEnd:
		newPos = int64(m.buffer.Len()) + offset
	default:
		return 0, errors.New("unsupported seek whence")
	}

	if newPos < 0 {
		return 0, errors.New("negative position is not allowed")
	}

	m.pos = newPos
	return newPos, nil
}

func (m *memorySparseFile) Write(b []byte) (n int, err error) {
	if int64(m.buffer.Len()) < m.pos {
		padding := make([]byte, m.pos-int64(m.buffer.Len()))
		_, err := m.buffer.Write(padding)
		if err != nil {
			return 0, err
		}
	}

	m.buffer.Next(int(m.pos) - m.buffer.Len())

	n, err = m.buffer.Write(b)
	m.pos += int64(n)
	return n, err
}

func (m *memorySparseFile) Close() error {
	return nil
}

func testInputWithWriteLen(t *testing.T, input []byte, chunkSize int) {
	m := &memorySparseFile{}
	sparseWriter := NewSparseWriter(m)

	for i := 0; i < len(input); i += chunkSize {
		end := i + chunkSize
		if end > len(input) {
			end = len(input)
		}
		_, err := sparseWriter.Write(input[i:end])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
	}
	err := sparseWriter.Close()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !bytes.Equal(input, m.buffer.Bytes()) {
		t.Fatalf("Incorrect output")
	}
}

func testInput(t *testing.T, inputBytes []byte) {
	currentLen := 1
	for {
		testInputWithWriteLen(t, inputBytes, currentLen)
		currentLen <<= 1
		if currentLen > len(inputBytes) {
			break
		}
	}
}

func TestSparseWriter(t *testing.T) {
	testInput(t, []byte("hello"))
	testInput(t, append(make([]byte, 100), []byte("hello")...))
	testInput(t, []byte(""))

	// add "hello" at the beginning
	largeInput := make([]byte, 1024*1024)
	copy(largeInput, []byte("hello"))
	testInput(t, largeInput)

	// add "hello" at the end
	largeInput = make([]byte, 1024*1024)
	copy(largeInput[1024*1024-5:], []byte("hello"))
	testInput(t, largeInput)

	// add "hello" in the middle
	largeInput = make([]byte, 1024*1024)
	copy(largeInput[len(largeInput)/2:], []byte("hello"))
	testInput(t, largeInput)
}
