package mparser

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/gomarkdown/markdown/ast"
	"github.com/mmarkdown/mmark/mast"
)

// IndexToDocumentIndex crawls the entire doc searching for indices, it will then return
// an mast.DocumentIndex that contains a tree:
//
// IndexLetter
// - IndexItem
//   - IndexLink
//   - IndexSubItem
//     - IndexLink
//     - IndexLink
//
// Which can then be rendered by the renderer.
func IndexToDocumentIndex(doc ast.Node) *mast.DocumentIndex {
	main := map[string]*mast.IndexItem{}
	subitem := map[string][]*mast.IndexSubItem{} // gather these so we can add them in one swoop at the end

	// Gather all indexes.
	ast.WalkFunc(doc, func(node ast.Node, entering bool) ast.WalkStatus {
		switch i := node.(type) {
		case *ast.Index:
			item := string(i.Item)

			if _, ok := main[item]; !ok {
				main[item] = &mast.IndexItem{Index: i}
			}
			// only the main item
			if i.Subitem == nil {
				ast.AppendChild(main[item], newLink(i.ID, len(main[item].GetChildren()), i.Primary))
				return ast.GoToNext
			}
			// check if we already have a child with the subitem and then just add the link
			for _, sub := range subitem[item] {
				if bytes.Compare(sub.Subitem, i.Subitem) == 0 {
					ast.AppendChild(sub, newLink(i.ID, len(sub.GetChildren()), i.Primary))
					return ast.GoToNext
				}
			}

			sub := &mast.IndexSubItem{Index: i}
			ast.AppendChild(sub, newLink(i.ID, len(subitem[item]), i.Primary))
			subitem[item] = append(subitem[item], sub)
		}
		return ast.GoToNext
	})
	if len(main) == 0 {
		return nil
	}

	// Now add a subitem children to the correct main item.
	for k, sub := range subitem {
		// sort sub here ideally
		for j := range sub {
			ast.AppendChild(main[k], sub[j])
		}
	}

	keys := []string{}
	for k := range main {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	letters := []*mast.IndexLetter{}
	var prevLetter byte
	var il *mast.IndexLetter
	for _, k := range keys {
		letter := k[0]
		if letter != prevLetter {
			il = &mast.IndexLetter{}
			il.Literal = []byte{letter}
			letters = append(letters, il)
		}
		ast.AppendChild(il, main[k])
		prevLetter = letter
	}
	docIndex := &mast.DocumentIndex{}
	for i := range letters {
		ast.AppendChild(docIndex, letters[i])
	}

	return docIndex
}

func newLink(id string, number int, primary bool) *mast.IndexLink {
	link := &ast.Link{Destination: []byte(id)}
	il := &mast.IndexLink{Link: link, Primary: primary}
	il.Literal = []byte(fmt.Sprintf("%d", number))
	return il
}

// AddIndex adds an index to the end of the current document. If not indices can be found
// this returns false and no index will be added.
func AddIndex(doc ast.Node) bool {
	idx := IndexToDocumentIndex(doc)
	if idx == nil {
		return false
	}

	ast.AppendChild(doc, idx)
	return true
}
