package parser

import (
	"bytes"

	"github.com/gomarkdown/markdown/ast"
)

// attribute parses a (potential) block attribute and adds it to p.
func (p *Parser) attribute(data []byte) []byte {
	if len(data) < 3 {
		return data
	}
	i := 0
	if data[i] != '{' {
		return data
	}
	i++

	// last character must be a } otherwise it's not an attribute
	end := skipUntilChar(data, i, '\n')
	if data[end-1] != '}' {
		return data
	}

	i = skipSpace(data, i)
	b := &ast.Attribute{Attrs: make(map[string][]byte)}

	esc := false
	quote := false
	trail := 0
Loop:
	for ; i < len(data); i++ {
		switch data[i] {
		case ' ', '\t', '\f', '\v':
			if quote {
				continue
			}
			chunk := data[trail+1 : i]
			if len(chunk) == 0 {
				trail = i
				continue
			}
			switch {
			case chunk[0] == '.':
				b.Classes = append(b.Classes, chunk[1:])
			case chunk[0] == '#':
				b.ID = chunk[1:]
			default:
				k, v := keyValue(chunk)
				if k != nil && v != nil {
					b.Attrs[string(k)] = v
				} else {
					// this is illegal in an attribute
					return data
				}
			}
			trail = i
		case '"':
			if esc {
				esc = !esc
				continue
			}
			quote = !quote
		case '\\':
			esc = !esc
		case '}':
			if esc {
				esc = !esc
				continue
			}
			chunk := data[trail+1 : i]
			if len(chunk) == 0 {
				return data
			}
			switch {
			case chunk[0] == '.':
				b.Classes = append(b.Classes, chunk[1:])
			case chunk[0] == '#':
				b.ID = chunk[1:]
			default:
				k, v := keyValue(chunk)
				if k != nil && v != nil {
					b.Attrs[string(k)] = v
				} else {
					return data
				}
			}
			i++
			break Loop
		default:
			esc = false
		}
	}

	p.attr = b
	return data[i:]
}

// key="value" quotes are mandatory.
func keyValue(data []byte) ([]byte, []byte) {
	chunk := bytes.SplitN(data, []byte{'='}, 2)
	if len(chunk) != 2 {
		return nil, nil
	}
	key := chunk[0]
	value := chunk[1]

	if len(value) < 3 || len(key) == 0 {
		return nil, nil
	}
	if value[0] != '"' || value[len(value)-1] != '"' {
		return key, nil
	}
	return key, value[1 : len(value)-1]
}
