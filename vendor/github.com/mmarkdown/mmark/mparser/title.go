package mparser

import (
	"log"

	"github.com/BurntSushi/toml"
	"github.com/gomarkdown/markdown/ast"
	"github.com/mmarkdown/mmark/mast"
)

// TitleHook will parse a title and returns it. The start and ending can
// be signalled with %%% or --- (the later to more inline with Hugo and other markdown dialects.
func TitleHook(data []byte) (ast.Node, []byte, int) {
	i := 0
	if len(data) < 3 {
		return nil, nil, 0
	}

	c := data[i] // first char can either be % or -
	if c != '%' && c != '-' {
		return nil, nil, 0
	}

	if data[i] != c || data[i+1] != c || data[i+2] != c {
		return nil, nil, 0
	}

	i += 3
	beg := i
	found := false
	// search for end.
	for i < len(data) {
		if data[i] == c && data[i+1] == c && data[i+2] == c {
			found = true
			break
		}
		i++
	}
	if !found {
		return nil, nil, 0
	}

	node := mast.NewTitle(c)
	buf := data[beg:i]

	if c == '-' {
		node.Content = buf
		return node, nil, i + 3
	}

	if _, err := toml.Decode(string(buf), node.TitleData); err != nil {
		log.Printf("Failure parsing title block: %s", err)
	}
	node.Content = buf

	return node, nil, i + 3
}
