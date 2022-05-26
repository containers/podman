package parser

import (
	"bytes"
	"strconv"
)

// IsCallout detects a callout in the following format: <<N>> Where N is a integer > 0.
func IsCallout(data []byte) (id []byte, consumed int) {
	if !bytes.HasPrefix(data, []byte("<<")) {
		return nil, 0
	}
	start := 2
	end := bytes.Index(data[start:], []byte(">>"))
	if end < 0 {
		return nil, 0
	}

	b := data[start : start+end]
	b = bytes.TrimSpace(b)
	i, err := strconv.Atoi(string(b))
	if err != nil {
		return nil, 0
	}
	if i <= 0 {
		return nil, 0
	}
	return b, start + end + 2 // 2 for >>
}
