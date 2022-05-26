package parser

import (
	"bytes"

	"github.com/gomarkdown/markdown/ast"
)

// sFigureLine checks if there's a figure line (e.g., !--- ) at the beginning of data,
// and returns the end index if so, or 0 otherwise.
func sFigureLine(data []byte, oldmarker string) (end int, marker string) {
	i, size := 0, 0

	n := len(data)
	// skip up to three spaces
	for i < n && i < 3 && data[i] == ' ' {
		i++
	}

	// check for the marker characters: !
	if i+1 >= n {
		return 0, ""
	}
	if data[i] != '!' || data[i+1] != '-' {
		return 0, ""
	}
	i++

	c := data[i] // i.e. the -

	// the whole line must be the same char or whitespace
	for i < n && data[i] == c {
		size++
		i++
	}

	// the marker char must occur at least 3 times
	if size < 3 {
		return 0, ""
	}
	marker = string(data[i-size : i])

	// if this is the end marker, it must match the beginning marker
	if oldmarker != "" && marker != oldmarker {
		return 0, ""
	}

	// there is no syntax modifier although it might be an idea to re-use this space for something?

	i = skipChar(data, i, ' ')
	if i >= n || data[i] != '\n' {
		if i == n {
			return i, marker
		}
		return 0, ""
	}
	return i + 1, marker // Take newline into account.
}

// figureBlock returns the end index if data contains a figure block at the beginning,
// or 0 otherwise. It writes to out if doRender is true, otherwise it has no side effects.
// If doRender is true, a final newline is mandatory to recognize the figure block.
func (p *Parser) figureBlock(data []byte, doRender bool) int {
	beg, marker := sFigureLine(data, "")
	if beg == 0 || beg >= len(data) {
		return 0
	}

	var raw bytes.Buffer

	for {
		// safe to assume beg < len(data)

		// check for the end of the code block
		figEnd, _ := sFigureLine(data[beg:], marker)
		if figEnd != 0 {
			beg += figEnd
			break
		}

		// copy the current line
		end := skipUntilChar(data, beg, '\n') + 1

		// did we reach the end of the buffer without a closing marker?
		if end >= len(data) {
			return 0
		}

		// verbatim copy to the working buffer
		if doRender {
			raw.Write(data[beg:end])
		}
		beg = end
	}

	if !doRender {
		return beg
	}

	figure := &ast.CaptionFigure{}
	p.addBlock(figure)
	p.block(raw.Bytes())

	defer p.finalize(figure)

	if captionContent, id, consumed := p.caption(data[beg:], []byte("Figure: ")); consumed > 0 {
		caption := &ast.Caption{}
		p.Inline(caption, captionContent)

		figure.HeadingID = id

		p.addChild(caption)

		beg += consumed
	}

	p.finalize(figure)
	return beg
}
