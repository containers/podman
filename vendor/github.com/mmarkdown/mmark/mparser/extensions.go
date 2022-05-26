package mparser

import (
	"github.com/gomarkdown/markdown/parser"
)

// Extensions is the default set of extensions mmark requires.
var Extensions = parser.Tables | parser.FencedCode | parser.Autolink | parser.Strikethrough |
	parser.SpaceHeadings | parser.HeadingIDs | parser.BackslashLineBreak | parser.SuperSubscript |
	parser.DefinitionLists | parser.MathJax | parser.AutoHeadingIDs | parser.Footnotes |
	parser.Strikethrough | parser.OrderedListStart | parser.Attributes | parser.Mmark | parser.Includes
