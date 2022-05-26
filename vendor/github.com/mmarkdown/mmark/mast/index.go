package mast

import "github.com/gomarkdown/markdown/ast"

// DocumentIndex represents markdown document index node.
type DocumentIndex struct {
	ast.Container
}

// IndexItem contains an index for the indices section.
type IndexItem struct {
	ast.Container

	*ast.Index
}

// IndexSubItem contains an sub item index for the indices section.
type IndexSubItem struct {
	ast.Container

	*ast.Index
}

// IndexLetter has the Letter of this index item.
type IndexLetter struct {
	ast.Container
}

// IndexLink links to the index in the document.
type IndexLink struct {
	*ast.Link

	Primary bool
}
