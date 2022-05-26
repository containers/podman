/*
Package parser implements parser for markdown text that generates AST (abstract syntax tree).
*/
package parser

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gomarkdown/markdown/ast"
)

// Extensions is a bitmask of enabled parser extensions.
type Extensions int

// Bit flags representing markdown parsing extensions.
// Use | (or) to specify multiple extensions.
const (
	NoExtensions           Extensions = 0
	NoIntraEmphasis        Extensions = 1 << iota // Ignore emphasis markers inside words
	Tables                                        // Parse tables
	FencedCode                                    // Parse fenced code blocks
	Autolink                                      // Detect embedded URLs that are not explicitly marked
	Strikethrough                                 // Strikethrough text using ~~test~~
	LaxHTMLBlocks                                 // Loosen up HTML block parsing rules
	SpaceHeadings                                 // Be strict about prefix heading rules
	HardLineBreak                                 // Translate newlines into line breaks
	NonBlockingSpace                              // Translate backspace spaces into line non-blocking spaces
	TabSizeEight                                  // Expand tabs to eight spaces instead of four
	Footnotes                                     // Pandoc-style footnotes
	NoEmptyLineBeforeBlock                        // No need to insert an empty line to start a (code, quote, ordered list, unordered list) block
	HeadingIDs                                    // specify heading IDs  with {#id}
	Titleblock                                    // Titleblock ala pandoc
	AutoHeadingIDs                                // Create the heading ID from the text
	BackslashLineBreak                            // Translate trailing backslashes into line breaks
	DefinitionLists                               // Parse definition lists
	MathJax                                       // Parse MathJax
	OrderedListStart                              // Keep track of the first number used when starting an ordered list.
	Attributes                                    // Block Attributes
	SuperSubscript                                // Super- and subscript support: 2^10^, H~2~O.
	EmptyLinesBreakList                           // 2 empty lines break out of list
	Includes                                      // Support including other files.
	Mmark                                         // Support Mmark syntax, see https://mmark.nl/syntax

	CommonExtensions Extensions = NoIntraEmphasis | Tables | FencedCode |
		Autolink | Strikethrough | SpaceHeadings | HeadingIDs |
		BackslashLineBreak | DefinitionLists | MathJax
)

// The size of a tab stop.
const (
	tabSizeDefault = 4
	tabSizeDouble  = 8
)

// for each character that triggers a response when parsing inline data.
type inlineParser func(p *Parser, data []byte, offset int) (int, ast.Node)

// ReferenceOverrideFunc is expected to be called with a reference string and
// return either a valid Reference type that the reference string maps to or
// nil. If overridden is false, the default reference logic will be executed.
// See the documentation in Options for more details on use-case.
type ReferenceOverrideFunc func(reference string) (ref *Reference, overridden bool)

// Parser is a type that holds extensions and the runtime state used by
// Parse, and the renderer. You can not use it directly, construct it with New.
type Parser struct {

	// ReferenceOverride is an optional function callback that is called every
	// time a reference is resolved. It can be set before starting parsing.
	//
	// In Markdown, the link reference syntax can be made to resolve a link to
	// a reference instead of an inline URL, in one of the following ways:
	//
	//  * [link text][refid]
	//  * [refid][]
	//
	// Usually, the refid is defined at the bottom of the Markdown document. If
	// this override function is provided, the refid is passed to the override
	// function first, before consulting the defined refids at the bottom. If
	// the override function indicates an override did not occur, the refids at
	// the bottom will be used to fill in the link details.
	ReferenceOverride ReferenceOverrideFunc

	Opts Options

	// after parsing, this is AST root of parsed markdown text
	Doc ast.Node

	extensions Extensions

	refs           map[string]*reference
	refsRecord     map[string]struct{}
	inlineCallback [256]inlineParser
	nesting        int
	maxNesting     int
	insideLink     bool
	indexCnt       int // incremented after every index

	// Footnotes need to be ordered as well as available to quickly check for
	// presence. If a ref is also a footnote, it's stored both in refs and here
	// in notes. Slice is nil if footnotes not enabled.
	notes []*reference

	tip                  ast.Node // = doc
	oldTip               ast.Node
	lastMatchedContainer ast.Node // = doc
	allClosed            bool

	// Attributes are attached to block level elements.
	attr *ast.Attribute

	includeStack *incStack
}

// New creates a markdown parser with CommonExtensions.
//
// You can then call `doc := p.Parse(markdown)` to parse markdown document
// and `markdown.Render(doc, renderer)` to convert it to another format with
// a renderer.
func New() *Parser {
	return NewWithExtensions(CommonExtensions)
}

// NewWithExtensions creates a markdown parser with given extensions.
func NewWithExtensions(extension Extensions) *Parser {
	p := Parser{
		refs:         make(map[string]*reference),
		refsRecord:   make(map[string]struct{}),
		maxNesting:   16,
		insideLink:   false,
		Doc:          &ast.Document{},
		extensions:   extension,
		allClosed:    true,
		includeStack: newIncStack(),
	}
	p.tip = p.Doc
	p.oldTip = p.Doc
	p.lastMatchedContainer = p.Doc

	p.inlineCallback[' '] = maybeLineBreak
	p.inlineCallback['*'] = emphasis
	p.inlineCallback['_'] = emphasis
	if p.extensions&Strikethrough != 0 {
		p.inlineCallback['~'] = emphasis
	}
	p.inlineCallback['`'] = codeSpan
	p.inlineCallback['\n'] = lineBreak
	p.inlineCallback['['] = link
	p.inlineCallback['<'] = leftAngle
	p.inlineCallback['\\'] = escape
	p.inlineCallback['&'] = entity
	p.inlineCallback['!'] = maybeImage
	if p.extensions&Mmark != 0 {
		p.inlineCallback['('] = maybeShortRefOrIndex
	}
	p.inlineCallback['^'] = maybeInlineFootnoteOrSuper
	if p.extensions&Autolink != 0 {
		p.inlineCallback['h'] = maybeAutoLink
		p.inlineCallback['m'] = maybeAutoLink
		p.inlineCallback['f'] = maybeAutoLink
		p.inlineCallback['H'] = maybeAutoLink
		p.inlineCallback['M'] = maybeAutoLink
		p.inlineCallback['F'] = maybeAutoLink
	}
	if p.extensions&MathJax != 0 {
		p.inlineCallback['$'] = math
	}

	return &p
}

func (p *Parser) getRef(refid string) (ref *reference, found bool) {
	if p.ReferenceOverride != nil {
		r, overridden := p.ReferenceOverride(refid)
		if overridden {
			if r == nil {
				return nil, false
			}
			return &reference{
				link:     []byte(r.Link),
				title:    []byte(r.Title),
				noteID:   0,
				hasBlock: false,
				text:     []byte(r.Text)}, true
		}
	}
	// refs are case insensitive
	ref, found = p.refs[strings.ToLower(refid)]
	return ref, found
}

func (p *Parser) isFootnote(ref *reference) bool {
	_, ok := p.refsRecord[string(ref.link)]
	return ok
}

func (p *Parser) finalize(block ast.Node) {
	p.tip = block.GetParent()
}

func (p *Parser) addChild(node ast.Node) ast.Node {
	for !canNodeContain(p.tip, node) {
		p.finalize(p.tip)
	}
	ast.AppendChild(p.tip, node)
	p.tip = node
	return node
}

func canNodeContain(n ast.Node, v ast.Node) bool {
	switch n.(type) {
	case *ast.List:
		return isListItem(v)
	case *ast.Document, *ast.BlockQuote, *ast.Aside, *ast.ListItem, *ast.CaptionFigure:
		return !isListItem(v)
	case *ast.Table:
		switch v.(type) {
		case *ast.TableHeader, *ast.TableBody, *ast.TableFooter:
			return true
		default:
			return false
		}
	case *ast.TableHeader, *ast.TableBody, *ast.TableFooter:
		_, ok := v.(*ast.TableRow)
		return ok
	case *ast.TableRow:
		_, ok := v.(*ast.TableCell)
		return ok
	}
	return false
}

func (p *Parser) closeUnmatchedBlocks() {
	if p.allClosed {
		return
	}
	for p.oldTip != p.lastMatchedContainer {
		parent := p.oldTip.GetParent()
		p.finalize(p.oldTip)
		p.oldTip = parent
	}
	p.allClosed = true
}

// Reference represents the details of a link.
// See the documentation in Options for more details on use-case.
type Reference struct {
	// Link is usually the URL the reference points to.
	Link string
	// Title is the alternate text describing the link in more detail.
	Title string
	// Text is the optional text to override the ref with if the syntax used was
	// [refid][]
	Text string
}

// Parse generates AST (abstract syntax tree) representing markdown document.
//
// The result is a root of the tree whose underlying type is *ast.Document
//
// You can then convert AST to html using html.Renderer, to some other format
// using a custom renderer or transform the tree.
func (p *Parser) Parse(input []byte) ast.Node {
	p.block(input)
	// Walk the tree and finish up some of unfinished blocks
	for p.tip != nil {
		p.finalize(p.tip)
	}
	// Walk the tree again and process inline markdown in each block
	ast.WalkFunc(p.Doc, func(node ast.Node, entering bool) ast.WalkStatus {
		switch node.(type) {
		case *ast.Paragraph, *ast.Heading, *ast.TableCell:
			p.Inline(node, node.AsContainer().Content)
			node.AsContainer().Content = nil
		}
		return ast.GoToNext
	})

	if p.Opts.Flags&SkipFootnoteList == 0 {
		p.parseRefsToAST()
	}
	return p.Doc
}

func (p *Parser) parseRefsToAST() {
	if p.extensions&Footnotes == 0 || len(p.notes) == 0 {
		return
	}
	p.tip = p.Doc
	list := &ast.List{
		IsFootnotesList: true,
		ListFlags:       ast.ListTypeOrdered,
	}
	p.addBlock(&ast.Footnotes{})
	block := p.addBlock(list)
	flags := ast.ListItemBeginningOfList
	// Note: this loop is intentionally explicit, not range-form. This is
	// because the body of the loop will append nested footnotes to p.notes and
	// we need to process those late additions. Range form would only walk over
	// the fixed initial set.
	for i := 0; i < len(p.notes); i++ {
		ref := p.notes[i]
		p.addChild(ref.footnote)
		block := ref.footnote
		listItem := block.(*ast.ListItem)
		listItem.ListFlags = flags | ast.ListTypeOrdered
		listItem.RefLink = ref.link
		if ref.hasBlock {
			flags |= ast.ListItemContainsBlock
			p.block(ref.title)
		} else {
			p.Inline(block, ref.title)
		}
		flags &^= ast.ListItemBeginningOfList | ast.ListItemContainsBlock
	}
	above := list.Parent
	finalizeList(list)
	p.tip = above

	ast.WalkFunc(block, func(node ast.Node, entering bool) ast.WalkStatus {
		switch node.(type) {
		case *ast.Paragraph, *ast.Heading:
			p.Inline(node, node.AsContainer().Content)
			node.AsContainer().Content = nil
		}
		return ast.GoToNext
	})
}

//
// Link references
//
// This section implements support for references that (usually) appear
// as footnotes in a document, and can be referenced anywhere in the document.
// The basic format is:
//
//    [1]: http://www.google.com/ "Google"
//    [2]: http://www.github.com/ "Github"
//
// Anywhere in the document, the reference can be linked by referring to its
// label, i.e., 1 and 2 in this example, as in:
//
//    This library is hosted on [Github][2], a git hosting site.
//
// Actual footnotes as specified in Pandoc and supported by some other Markdown
// libraries such as php-markdown are also taken care of. They look like this:
//
//    This sentence needs a bit of further explanation.[^note]
//
//    [^note]: This is the explanation.
//
// Footnotes should be placed at the end of the document in an ordered list.
// Inline footnotes such as:
//
//    Inline footnotes^[Not supported.] also exist.
//
// are not yet supported.

// reference holds all information necessary for a reference-style links or
// footnotes.
//
// Consider this markdown with reference-style links:
//
//     [link][ref]
//
//     [ref]: /url/ "tooltip title"
//
// It will be ultimately converted to this HTML:
//
//     <p><a href=\"/url/\" title=\"title\">link</a></p>
//
// And a reference structure will be populated as follows:
//
//     p.refs["ref"] = &reference{
//         link: "/url/",
//         title: "tooltip title",
//     }
//
// Alternatively, reference can contain information about a footnote. Consider
// this markdown:
//
//     Text needing a footnote.[^a]
//
//     [^a]: This is the note
//
// A reference structure will be populated as follows:
//
//     p.refs["a"] = &reference{
//         link: "a",
//         title: "This is the note",
//         noteID: <some positive int>,
//     }
//
// TODO: As you can see, it begs for splitting into two dedicated structures
// for refs and for footnotes.
type reference struct {
	link     []byte
	title    []byte
	noteID   int // 0 if not a footnote ref
	hasBlock bool
	footnote ast.Node // a link to the Item node within a list of footnotes

	text []byte // only gets populated by refOverride feature with Reference.Text
}

func (r *reference) String() string {
	return fmt.Sprintf("{link: %q, title: %q, text: %q, noteID: %d, hasBlock: %v}",
		r.link, r.title, r.text, r.noteID, r.hasBlock)
}

// Check whether or not data starts with a reference link.
// If so, it is parsed and stored in the list of references
// (in the render struct).
// Returns the number of bytes to skip to move past it,
// or zero if the first line is not a reference.
func isReference(p *Parser, data []byte, tabSize int) int {
	// up to 3 optional leading spaces
	if len(data) < 4 {
		return 0
	}
	i := 0
	for i < 3 && data[i] == ' ' {
		i++
	}

	noteID := 0

	// id part: anything but a newline between brackets
	if data[i] != '[' {
		return 0
	}
	i++
	if p.extensions&Footnotes != 0 {
		if i < len(data) && data[i] == '^' {
			// we can set it to anything here because the proper noteIds will
			// be assigned later during the second pass. It just has to be != 0
			noteID = 1
			i++
		}
	}
	idOffset := i
	for i < len(data) && data[i] != '\n' && data[i] != '\r' && data[i] != ']' {
		i++
	}
	if i >= len(data) || data[i] != ']' {
		return 0
	}
	idEnd := i
	// footnotes can have empty ID, like this: [^], but a reference can not be
	// empty like this: []. Break early if it's not a footnote and there's no ID
	if noteID == 0 && idOffset == idEnd {
		return 0
	}
	// spacer: colon (space | tab)* newline? (space | tab)*
	i++
	if i >= len(data) || data[i] != ':' {
		return 0
	}
	i++
	for i < len(data) && (data[i] == ' ' || data[i] == '\t') {
		i++
	}
	if i < len(data) && (data[i] == '\n' || data[i] == '\r') {
		i++
		if i < len(data) && data[i] == '\n' && data[i-1] == '\r' {
			i++
		}
	}
	for i < len(data) && (data[i] == ' ' || data[i] == '\t') {
		i++
	}
	if i >= len(data) {
		return 0
	}

	var (
		linkOffset, linkEnd   int
		titleOffset, titleEnd int
		lineEnd               int
		raw                   []byte
		hasBlock              bool
	)

	if p.extensions&Footnotes != 0 && noteID != 0 {
		linkOffset, linkEnd, raw, hasBlock = scanFootnote(p, data, i, tabSize)
		lineEnd = linkEnd
	} else {
		linkOffset, linkEnd, titleOffset, titleEnd, lineEnd = scanLinkRef(p, data, i)
	}
	if lineEnd == 0 {
		return 0
	}

	// a valid ref has been found

	ref := &reference{
		noteID:   noteID,
		hasBlock: hasBlock,
	}

	if noteID > 0 {
		// reusing the link field for the id since footnotes don't have links
		ref.link = data[idOffset:idEnd]
		// if footnote, it's not really a title, it's the contained text
		ref.title = raw
	} else {
		ref.link = data[linkOffset:linkEnd]
		ref.title = data[titleOffset:titleEnd]
	}

	// id matches are case-insensitive
	id := string(bytes.ToLower(data[idOffset:idEnd]))

	p.refs[id] = ref

	return lineEnd
}

func scanLinkRef(p *Parser, data []byte, i int) (linkOffset, linkEnd, titleOffset, titleEnd, lineEnd int) {
	// link: whitespace-free sequence, optionally between angle brackets
	if data[i] == '<' {
		i++
	}
	linkOffset = i
	for i < len(data) && data[i] != ' ' && data[i] != '\t' && data[i] != '\n' && data[i] != '\r' {
		i++
	}
	linkEnd = i
	if linkEnd < len(data) && data[linkOffset] == '<' && data[linkEnd-1] == '>' {
		linkOffset++
		linkEnd--
	}

	// optional spacer: (space | tab)* (newline | '\'' | '"' | '(' )
	for i < len(data) && (data[i] == ' ' || data[i] == '\t') {
		i++
	}
	if i < len(data) && data[i] != '\n' && data[i] != '\r' && data[i] != '\'' && data[i] != '"' && data[i] != '(' {
		return
	}

	// compute end-of-line
	if i >= len(data) || data[i] == '\r' || data[i] == '\n' {
		lineEnd = i
	}
	if i+1 < len(data) && data[i] == '\r' && data[i+1] == '\n' {
		lineEnd++
	}

	// optional (space|tab)* spacer after a newline
	if lineEnd > 0 {
		i = lineEnd + 1
		for i < len(data) && (data[i] == ' ' || data[i] == '\t') {
			i++
		}
	}

	// optional title: any non-newline sequence enclosed in '"() alone on its line
	if i+1 < len(data) && (data[i] == '\'' || data[i] == '"' || data[i] == '(') {
		i++
		titleOffset = i

		// look for EOL
		for i < len(data) && data[i] != '\n' && data[i] != '\r' {
			i++
		}
		if i+1 < len(data) && data[i] == '\n' && data[i+1] == '\r' {
			titleEnd = i + 1
		} else {
			titleEnd = i
		}

		// step back
		i--
		for i > titleOffset && (data[i] == ' ' || data[i] == '\t') {
			i--
		}
		if i > titleOffset && (data[i] == '\'' || data[i] == '"' || data[i] == ')') {
			lineEnd = titleEnd
			titleEnd = i
		}
	}

	return
}

// The first bit of this logic is the same as Parser.listItem, but the rest
// is much simpler. This function simply finds the entire block and shifts it
// over by one tab if it is indeed a block (just returns the line if it's not).
// blockEnd is the end of the section in the input buffer, and contents is the
// extracted text that was shifted over one tab. It will need to be rendered at
// the end of the document.
func scanFootnote(p *Parser, data []byte, i, indentSize int) (blockStart, blockEnd int, contents []byte, hasBlock bool) {
	if i == 0 || len(data) == 0 {
		return
	}

	// skip leading whitespace on first line
	for i < len(data) && data[i] == ' ' {
		i++
	}

	blockStart = i

	// find the end of the line
	blockEnd = i
	for i < len(data) && data[i-1] != '\n' {
		i++
	}

	// get working buffer
	var raw bytes.Buffer

	// put the first line into the working buffer
	raw.Write(data[blockEnd:i])
	blockEnd = i

	// process the following lines
	containsBlankLine := false

gatherLines:
	for blockEnd < len(data) {
		i++

		// find the end of this line
		for i < len(data) && data[i-1] != '\n' {
			i++
		}

		// if it is an empty line, guess that it is part of this item
		// and move on to the next line
		if p.isEmpty(data[blockEnd:i]) > 0 {
			containsBlankLine = true
			blockEnd = i
			continue
		}

		n := 0
		if n = isIndented(data[blockEnd:i], indentSize); n == 0 {
			// this is the end of the block.
			// we don't want to include this last line in the index.
			break gatherLines
		}

		// if there were blank lines before this one, insert a new one now
		if containsBlankLine {
			raw.WriteByte('\n')
			containsBlankLine = false
		}

		// get rid of that first tab, write to buffer
		raw.Write(data[blockEnd+n : i])
		hasBlock = true

		blockEnd = i
	}

	if data[blockEnd-1] != '\n' {
		raw.WriteByte('\n')
	}

	contents = raw.Bytes()

	return
}

// isPunctuation returns true if c is a punctuation symbol.
func isPunctuation(c byte) bool {
	for _, r := range []byte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~") {
		if c == r {
			return true
		}
	}
	return false
}

// isSpace returns true if c is a white-space charactr
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}

// isLetter returns true if c is ascii letter
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isAlnum returns true if c is a digit or letter
// TODO: check when this is looking for ASCII alnum and when it should use unicode
func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || isLetter(c)
}

// TODO: this is not used
// Replace tab characters with spaces, aligning to the next TAB_SIZE column.
// always ends output with a newline
func expandTabs(out *bytes.Buffer, line []byte, tabSize int) {
	// first, check for common cases: no tabs, or only tabs at beginning of line
	i, prefix := 0, 0
	slowcase := false
	for i = 0; i < len(line); i++ {
		if line[i] == '\t' {
			if prefix == i {
				prefix++
			} else {
				slowcase = true
				break
			}
		}
	}

	// no need to decode runes if all tabs are at the beginning of the line
	if !slowcase {
		for i = 0; i < prefix*tabSize; i++ {
			out.WriteByte(' ')
		}
		out.Write(line[prefix:])
		return
	}

	// the slow case: we need to count runes to figure out how
	// many spaces to insert for each tab
	column := 0
	i = 0
	for i < len(line) {
		start := i
		for i < len(line) && line[i] != '\t' {
			_, size := utf8.DecodeRune(line[i:])
			i += size
			column++
		}

		if i > start {
			out.Write(line[start:i])
		}

		if i >= len(line) {
			break
		}

		for {
			out.WriteByte(' ')
			column++
			if column%tabSize == 0 {
				break
			}
		}

		i++
	}
}

// Find if a line counts as indented or not.
// Returns number of characters the indent is (0 = not indented).
func isIndented(data []byte, indentSize int) int {
	if len(data) == 0 {
		return 0
	}
	if data[0] == '\t' {
		return 1
	}
	if len(data) < indentSize {
		return 0
	}
	for i := 0; i < indentSize; i++ {
		if data[i] != ' ' {
			return 0
		}
	}
	return indentSize
}

// Create a url-safe slug for fragments
func slugify(in []byte) []byte {
	if len(in) == 0 {
		return in
	}
	out := make([]byte, 0, len(in))
	sym := false

	for _, ch := range in {
		if isAlnum(ch) {
			sym = false
			out = append(out, ch)
		} else if sym {
			continue
		} else {
			out = append(out, '-')
			sym = true
		}
	}
	var a, b int
	var ch byte
	for a, ch = range out {
		if ch != '-' {
			break
		}
	}
	for b = len(out) - 1; b > 0; b-- {
		if out[b] != '-' {
			break
		}
	}
	return out[a : b+1]
}

func isListItem(d ast.Node) bool {
	_, ok := d.(*ast.ListItem)
	return ok
}
