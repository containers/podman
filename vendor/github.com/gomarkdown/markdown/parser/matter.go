package parser

import (
	"bytes"

	"github.com/gomarkdown/markdown/ast"
)

func (p *Parser) documentMatter(data []byte) int {
	if data[0] != '{' {
		return 0
	}

	consumed := 0
	matter := ast.DocumentMatterNone
	if bytes.HasPrefix(data, []byte("{frontmatter}")) {
		consumed = len("{frontmatter}")
		matter = ast.DocumentMatterFront
	}
	if bytes.HasPrefix(data, []byte("{mainmatter}")) {
		consumed = len("{mainmatter}")
		matter = ast.DocumentMatterMain
	}
	if bytes.HasPrefix(data, []byte("{backmatter}")) {
		consumed = len("{backmatter}")
		matter = ast.DocumentMatterBack
	}
	if consumed == 0 {
		return 0
	}
	node := &ast.DocumentMatter{Matter: matter}
	p.addBlock(node)
	p.finalize(node)

	return consumed
}
