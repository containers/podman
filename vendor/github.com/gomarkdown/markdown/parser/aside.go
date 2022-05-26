package parser

import (
	"bytes"

	"github.com/gomarkdown/markdown/ast"
)

// returns aisde prefix length
func (p *Parser) asidePrefix(data []byte) int {
	i := 0
	n := len(data)
	for i < 3 && i < n && data[i] == ' ' {
		i++
	}
	if i+1 < n && data[i] == 'A' && data[i+1] == '>' {
		if i+2 < n && data[i+2] == ' ' {
			return i + 3
		}
		return i + 2
	}
	return 0
}

// aside ends with at least one blank line
// followed by something without a aside prefix
func (p *Parser) terminateAside(data []byte, beg, end int) bool {
	if p.isEmpty(data[beg:]) <= 0 {
		return false
	}
	if end >= len(data) {
		return true
	}
	return p.asidePrefix(data[end:]) == 0 && p.isEmpty(data[end:]) == 0
}

// parse a aside fragment
func (p *Parser) aside(data []byte) int {
	var raw bytes.Buffer
	beg, end := 0, 0
	// identical to quote
	for beg < len(data) {
		end = beg
		// Step over whole lines, collecting them. While doing that, check for
		// fenced code and if one's found, incorporate it altogether,
		// irregardless of any contents inside it
		for end < len(data) && data[end] != '\n' {
			if p.extensions&FencedCode != 0 {
				if i := p.fencedCodeBlock(data[end:], false); i > 0 {
					// -1 to compensate for the extra end++ after the loop:
					end += i - 1
					break
				}
			}
			end++
		}
		end = skipCharN(data, end, '\n', 1)
		if pre := p.asidePrefix(data[beg:]); pre > 0 {
			// skip the prefix
			beg += pre
		} else if p.terminateAside(data, beg, end) {
			break
		}
		// this line is part of the aside
		raw.Write(data[beg:end])
		beg = end
	}

	block := p.addBlock(&ast.Aside{})
	p.block(raw.Bytes())
	p.finalize(block)
	return end
}
