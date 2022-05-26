package parser

import (
	"bytes"

	"github.com/gomarkdown/markdown/ast"
)

// citation parses a citation. In its most simple form [@ref], we allow multiple
// being separated by semicolons and a sub reference inside ala pandoc: [@ref, p. 23].
// Each citation can have a modifier: !, ? or - wich mean:
//
// ! - normative
// ? - formative
// - - suppressed
//
// The suffix starts after a comma, we strip any whitespace before and after. If the output
// allows for it, this can be rendered.
func citation(p *Parser, data []byte, offset int) (int, ast.Node) {
	// look for the matching closing bracket
	i := offset + 1
	for level := 1; level > 0 && i < len(data); i++ {
		switch {
		case data[i] == '\n':
			// no newlines allowed.
			return 0, nil

		case data[i-1] == '\\':
			continue

		case data[i] == '[':
			level++

		case data[i] == ']':
			level--
			if level <= 0 {
				i-- // compensate for extra i++ in for loop
			}
		}
	}

	if i >= len(data) {
		return 0, nil
	}

	node := &ast.Citation{}

	citations := bytes.Split(data[1:i], []byte(";"))
	for _, citation := range citations {
		var suffix []byte
		citation = bytes.TrimSpace(citation)
		j := 0
		if citation[j] != '@' {
			// not a citation, drop out entirely.
			return 0, nil
		}
		if c := bytes.Index(citation, []byte(",")); c > 0 {
			part := citation[:c]
			suff := citation[c+1:]
			part = bytes.TrimSpace(part)
			suff = bytes.TrimSpace(suff)

			citation = part
			suffix = suff
		}

		citeType := ast.CitationTypeInformative
		j = 1
		switch citation[j] {
		case '!':
			citeType = ast.CitationTypeNormative
			j++
		case '?':
			citeType = ast.CitationTypeInformative
			j++
		case '-':
			citeType = ast.CitationTypeSuppressed
			j++
		}
		node.Destination = append(node.Destination, citation[j:])
		node.Type = append(node.Type, citeType)
		node.Suffix = append(node.Suffix, suffix)
	}

	return i + 1, node
}
