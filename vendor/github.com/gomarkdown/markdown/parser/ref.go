package parser

import (
	"bytes"
	"fmt"

	"github.com/gomarkdown/markdown/ast"
)

// parse '(#r)', where r does not contain spaces. Or.
// (!item) (!item, subitem), for an index, (!!item) signals primary.
func maybeShortRefOrIndex(p *Parser, data []byte, offset int) (int, ast.Node) {
	if len(data[offset:]) < 4 {
		return 0, nil
	}
	// short ref first
	data = data[offset:]
	i := 1
	switch data[i] {
	case '#': // cross ref
		i++
	Loop:
		for i < len(data) {
			c := data[i]
			switch {
			case c == ')':
				break Loop
			case !isAlnum(c):
				if c == '_' || c == '-' || c == ':' {
					i++
					continue
				}
				i = 0
				break Loop
			}
			i++
		}
		if i >= len(data) {
			return 0, nil
		}
		if data[i] != ')' {
			return 0, nil
		}

		id := data[2:i]
		node := &ast.CrossReference{}
		node.Destination = id

		return i + 1, node

	case '!': // index
		i++
		start := i
		i = skipUntilChar(data, start, ')')

		// did we reach the end of the buffer without a closing marker?
		if i >= len(data) {
			return 0, nil
		}

		if len(data[start:i]) < 1 {
			return 0, nil
		}

		idx := &ast.Index{}

		idx.ID = fmt.Sprintf("idxref:%d", p.indexCnt)
		p.indexCnt++

		idx.Primary = data[start] == '!'
		buf := data[start:i]

		if idx.Primary {
			buf = buf[1:]
		}
		items := bytes.Split(buf, []byte(","))
		switch len(items) {
		case 1:
			idx.Item = bytes.TrimSpace(items[0])
			return i + 1, idx
		case 2:
			idx.Item = bytes.TrimSpace(items[0])
			idx.Subitem = bytes.TrimSpace(items[1])
			return i + 1, idx
		}
	}

	return 0, nil
}
