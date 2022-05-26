package parser

import (
	"bytes"
)

// caption checks for a caption, it returns the caption data and a potential "headingID".
func (p *Parser) caption(data, caption []byte) ([]byte, string, int) {
	if !bytes.HasPrefix(data, caption) {
		return nil, "", 0
	}
	j := len(caption)
	data = data[j:]
	end := p.linesUntilEmpty(data)

	data = data[:end]

	id, start := captionID(data)
	if id != "" {
		return data[:start], id, end + j
	}

	return data, "", end + j
}

// linesUntilEmpty scans lines up to the first empty line.
func (p *Parser) linesUntilEmpty(data []byte) int {
	line, i := 0, 0

	for line < len(data) {
		i++

		// find the end of this line
		for i < len(data) && data[i-1] != '\n' {
			i++
		}

		if p.isEmpty(data[line:i]) == 0 {
			line = i
			continue
		}

		break
	}
	return i
}

// captionID checks if the caption *ends* in {#....}. If so the text after {# is taken to be
// the ID/anchor of the entire figure block.
func captionID(data []byte) (string, int) {
	end := len(data)

	j, k := 0, 0
	// find start/end of heading id
	for j = 0; j < end-1 && (data[j] != '{' || data[j+1] != '#'); j++ {
	}
	for k = j + 1; k < end && data[k] != '}'; k++ {
	}
	// remains must be whitespace.
	for l := k + 1; l < end; l++ {
		if !isSpace(data[l]) {
			return "", 0
		}
	}

	if j > 0 && k > 0 && j+2 < k {
		return string(data[j+2 : k]), j
	}
	return "", 0
}
