package parser

import (
	"bytes"
	"html"
	"regexp"
	"strconv"
	"unicode"

	"github.com/gomarkdown/markdown/ast"
)

// Parsing block-level elements.

const (
	charEntity = "&(?:#x[a-f0-9]{1,8}|#[0-9]{1,8}|[a-z][a-z0-9]{1,31});"
	escapable  = "[!\"#$%&'()*+,./:;<=>?@[\\\\\\]^_`{|}~-]"
)

var (
	reBackslashOrAmp      = regexp.MustCompile("[\\&]")
	reEntityOrEscapedChar = regexp.MustCompile("(?i)\\\\" + escapable + "|" + charEntity)

	// blockTags is a set of tags that are recognized as HTML block tags.
	// Any of these can be included in markdown text without special escaping.
	blockTags = map[string]struct{}{
		"blockquote": {},
		"del":        {},
		"dd":         {},
		"div":        {},
		"dl":         {},
		"dt":         {},
		"fieldset":   {},
		"form":       {},
		"h1":         {},
		"h2":         {},
		"h3":         {},
		"h4":         {},
		"h5":         {},
		"h6":         {},
		// TODO: technically block but breaks Inline HTML (Simple).text
		//"hr":         {},
		"iframe":   {},
		"ins":      {},
		"li":       {},
		"math":     {},
		"noscript": {},
		"ol":       {},
		"pre":      {},
		"p":        {},
		"script":   {},
		"style":    {},
		"table":    {},
		"ul":       {},

		// HTML5
		"address":    {},
		"article":    {},
		"aside":      {},
		"canvas":     {},
		"details":    {},
		"dialog":     {},
		"figcaption": {},
		"figure":     {},
		"footer":     {},
		"header":     {},
		"hgroup":     {},
		"main":       {},
		"nav":        {},
		"output":     {},
		"progress":   {},
		"section":    {},
		"video":      {},
	}
)

// sanitizeAnchorName returns a sanitized anchor name for the given text.
// Taken from https://github.com/shurcooL/sanitized_anchor_name/blob/master/main.go#L14:1
func sanitizeAnchorName(text string) string {
	var anchorName []rune
	var futureDash = false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if futureDash && len(anchorName) > 0 {
				anchorName = append(anchorName, '-')
			}
			futureDash = false
			anchorName = append(anchorName, unicode.ToLower(r))
		default:
			futureDash = true
		}
	}
	return string(anchorName)
}

// Parse block-level data.
// Note: this function and many that it calls assume that
// the input buffer ends with a newline.
func (p *Parser) block(data []byte) {
	// this is called recursively: enforce a maximum depth
	if p.nesting >= p.maxNesting {
		return
	}
	p.nesting++

	// parse out one block-level construct at a time
	for len(data) > 0 {
		// attributes that can be specific before a block element:
		//
		// {#id .class1 .class2 key="value"}
		if p.extensions&Attributes != 0 {
			data = p.attribute(data)
		}

		if p.extensions&Includes != 0 {
			f := p.readInclude
			path, address, consumed := p.isInclude(data)
			if consumed == 0 {
				path, address, consumed = p.isCodeInclude(data)
				f = p.readCodeInclude
			}
			if consumed > 0 {
				included := f(p.includeStack.Last(), path, address)
				p.includeStack.Push(path)
				p.block(included)
				p.includeStack.Pop()
				data = data[consumed:]
				continue
			}
		}

		// user supplied parser function
		if p.Opts.ParserHook != nil {
			node, blockdata, consumed := p.Opts.ParserHook(data)
			if consumed > 0 {
				data = data[consumed:]

				if node != nil {
					p.addBlock(node)
					if blockdata != nil {
						p.block(blockdata)
						p.finalize(node)
					}
				}
				continue
			}
		}

		// prefixed heading:
		//
		// # Heading 1
		// ## Heading 2
		// ...
		// ###### Heading 6
		if p.isPrefixHeading(data) {
			data = data[p.prefixHeading(data):]
			continue
		}

		// prefixed special heading:
		// (there are no levels.)
		//
		// .# Abstract
		if p.isPrefixSpecialHeading(data) {
			data = data[p.prefixSpecialHeading(data):]
			continue
		}

		// block of preformatted HTML:
		//
		// <div>
		//     ...
		// </div>
		if data[0] == '<' {
			if i := p.html(data, true); i > 0 {
				data = data[i:]
				continue
			}
		}

		// title block
		//
		// % stuff
		// % more stuff
		// % even more stuff
		if p.extensions&Titleblock != 0 {
			if data[0] == '%' {
				if i := p.titleBlock(data, true); i > 0 {
					data = data[i:]
					continue
				}
			}
		}

		// blank lines.  note: returns the # of bytes to skip
		if i := p.isEmpty(data); i > 0 {
			data = data[i:]
			continue
		}

		// indented code block:
		//
		//     func max(a, b int) int {
		//         if a > b {
		//             return a
		//         }
		//         return b
		//      }
		if p.codePrefix(data) > 0 {
			data = data[p.code(data):]
			continue
		}

		// fenced code block:
		//
		// ``` go
		// func fact(n int) int {
		//     if n <= 1 {
		//         return n
		//     }
		//     return n * fact(n-1)
		// }
		// ```
		if p.extensions&FencedCode != 0 {
			if i := p.fencedCodeBlock(data, true); i > 0 {
				data = data[i:]
				continue
			}
		}

		// horizontal rule:
		//
		// ------
		// or
		// ******
		// or
		// ______
		if p.isHRule(data) {
			i := skipUntilChar(data, 0, '\n')
			hr := ast.HorizontalRule{}
			hr.Literal = bytes.Trim(data[:i], " \n")
			p.addBlock(&hr)
			data = data[i:]
			continue
		}

		// block quote:
		//
		// > A big quote I found somewhere
		// > on the web
		if p.quotePrefix(data) > 0 {
			data = data[p.quote(data):]
			continue
		}

		// aside:
		//
		// A> The proof is too large to fit
		// A> in the margin.
		if p.extensions&Mmark != 0 {
			if p.asidePrefix(data) > 0 {
				data = data[p.aside(data):]
				continue
			}
		}

		// figure block:
		//
		// !---
		// ![Alt Text](img.jpg "This is an image")
		// ![Alt Text](img2.jpg "This is a second image")
		// !---
		if p.extensions&Mmark != 0 {
			if i := p.figureBlock(data, true); i > 0 {
				data = data[i:]
				continue
			}
		}

		// table:
		//
		// Name  | Age | Phone
		// ------|-----|---------
		// Bob   | 31  | 555-1234
		// Alice | 27  | 555-4321
		if p.extensions&Tables != 0 {
			if i := p.table(data); i > 0 {
				data = data[i:]
				continue
			}
		}

		// an itemized/unordered list:
		//
		// * Item 1
		// * Item 2
		//
		// also works with + or -
		if p.uliPrefix(data) > 0 {
			data = data[p.list(data, 0, 0):]
			continue
		}

		// a numbered/ordered list:
		//
		// 1. Item 1
		// 2. Item 2
		if i := p.oliPrefix(data); i > 0 {
			start := 0
			if i > 2 && p.extensions&OrderedListStart != 0 {
				s := string(data[:i-2])
				start, _ = strconv.Atoi(s)
				if start == 1 {
					start = 0
				}
			}
			data = data[p.list(data, ast.ListTypeOrdered, start):]
			continue
		}

		// definition lists:
		//
		// Term 1
		// :   Definition a
		// :   Definition b
		//
		// Term 2
		// :   Definition c
		if p.extensions&DefinitionLists != 0 {
			if p.dliPrefix(data) > 0 {
				data = data[p.list(data, ast.ListTypeDefinition, 0):]
				continue
			}
		}

		if p.extensions&MathJax != 0 {
			if i := p.blockMath(data); i > 0 {
				data = data[i:]
				continue
			}
		}

		// document matters:
		//
		// {frontmatter}/{mainmatter}/{backmatter}
		if p.extensions&Mmark != 0 {
			if i := p.documentMatter(data); i > 0 {
				data = data[i:]
				continue
			}
		}

		// anything else must look like a normal paragraph
		// note: this finds underlined headings, too
		idx := p.paragraph(data)
		data = data[idx:]
	}

	p.nesting--
}

func (p *Parser) addBlock(n ast.Node) ast.Node {
	p.closeUnmatchedBlocks()

	if p.attr != nil {
		if c := n.AsContainer(); c != nil {
			c.Attribute = p.attr
		}
		if l := n.AsLeaf(); l != nil {
			l.Attribute = p.attr
		}
		p.attr = nil
	}
	return p.addChild(n)
}

func (p *Parser) isPrefixHeading(data []byte) bool {
	if data[0] != '#' {
		return false
	}

	if p.extensions&SpaceHeadings != 0 {
		level := skipCharN(data, 0, '#', 6)
		if level == len(data) || data[level] != ' ' {
			return false
		}
	}
	return true
}

func (p *Parser) prefixHeading(data []byte) int {
	level := skipCharN(data, 0, '#', 6)
	i := skipChar(data, level, ' ')
	end := skipUntilChar(data, i, '\n')
	skip := end
	id := ""
	if p.extensions&HeadingIDs != 0 {
		j, k := 0, 0
		// find start/end of heading id
		for j = i; j < end-1 && (data[j] != '{' || data[j+1] != '#'); j++ {
		}
		for k = j + 1; k < end && data[k] != '}'; k++ {
		}
		// extract heading id iff found
		if j < end && k < end {
			id = string(data[j+2 : k])
			end = j
			skip = k + 1
			for end > 0 && data[end-1] == ' ' {
				end--
			}
		}
	}
	for end > 0 && data[end-1] == '#' {
		if isBackslashEscaped(data, end-1) {
			break
		}
		end--
	}
	for end > 0 && data[end-1] == ' ' {
		end--
	}
	if end > i {
		if id == "" && p.extensions&AutoHeadingIDs != 0 {
			id = sanitizeAnchorName(string(data[i:end]))
		}
		block := &ast.Heading{
			HeadingID: id,
			Level:     level,
		}
		block.Content = data[i:end]
		p.addBlock(block)
	}
	return skip
}

func (p *Parser) isPrefixSpecialHeading(data []byte) bool {
	if p.extensions|Mmark == 0 {
		return false
	}
	if len(data) < 4 {
		return false
	}
	if data[0] != '.' {
		return false
	}
	if data[1] != '#' {
		return false
	}
	if data[2] == '#' { // we don't support level, so nack this.
		return false
	}

	if p.extensions&SpaceHeadings != 0 {
		if data[2] != ' ' {
			return false
		}
	}
	return true
}

func (p *Parser) prefixSpecialHeading(data []byte) int {
	i := skipChar(data, 2, ' ') // ".#" skipped
	end := skipUntilChar(data, i, '\n')
	skip := end
	id := ""
	if p.extensions&HeadingIDs != 0 {
		j, k := 0, 0
		// find start/end of heading id
		for j = i; j < end-1 && (data[j] != '{' || data[j+1] != '#'); j++ {
		}
		for k = j + 1; k < end && data[k] != '}'; k++ {
		}
		// extract heading id iff found
		if j < end && k < end {
			id = string(data[j+2 : k])
			end = j
			skip = k + 1
			for end > 0 && data[end-1] == ' ' {
				end--
			}
		}
	}
	for end > 0 && data[end-1] == '#' {
		if isBackslashEscaped(data, end-1) {
			break
		}
		end--
	}
	for end > 0 && data[end-1] == ' ' {
		end--
	}
	if end > i {
		if id == "" && p.extensions&AutoHeadingIDs != 0 {
			id = sanitizeAnchorName(string(data[i:end]))
		}
		block := &ast.Heading{
			HeadingID: id,
			IsSpecial: true,
			Level:     1, // always level 1.
		}
		block.Literal = data[i:end]
		block.Content = data[i:end]
		p.addBlock(block)
	}
	return skip
}

func (p *Parser) isUnderlinedHeading(data []byte) int {
	// test of level 1 heading
	if data[0] == '=' {
		i := skipChar(data, 1, '=')
		i = skipChar(data, i, ' ')
		if i < len(data) && data[i] == '\n' {
			return 1
		}
		return 0
	}

	// test of level 2 heading
	if data[0] == '-' {
		i := skipChar(data, 1, '-')
		i = skipChar(data, i, ' ')
		if i < len(data) && data[i] == '\n' {
			return 2
		}
		return 0
	}

	return 0
}

func (p *Parser) titleBlock(data []byte, doRender bool) int {
	if data[0] != '%' {
		return 0
	}
	splitData := bytes.Split(data, []byte("\n"))
	var i int
	for idx, b := range splitData {
		if !bytes.HasPrefix(b, []byte("%")) {
			i = idx // - 1
			break
		}
	}

	data = bytes.Join(splitData[0:i], []byte("\n"))
	consumed := len(data)
	data = bytes.TrimPrefix(data, []byte("% "))
	data = bytes.Replace(data, []byte("\n% "), []byte("\n"), -1)
	block := &ast.Heading{
		Level:        1,
		IsTitleblock: true,
	}
	block.Content = data
	p.addBlock(block)

	return consumed
}

func (p *Parser) html(data []byte, doRender bool) int {
	var i, j int

	// identify the opening tag
	if data[0] != '<' {
		return 0
	}
	curtag, tagfound := p.htmlFindTag(data[1:])

	// handle special cases
	if !tagfound {
		// check for an HTML comment
		if size := p.htmlComment(data, doRender); size > 0 {
			return size
		}

		// check for an <hr> tag
		if size := p.htmlHr(data, doRender); size > 0 {
			return size
		}

		// no special case recognized
		return 0
	}

	// look for an unindented matching closing tag
	// followed by a blank line
	found := false
	/*
		closetag := []byte("\n</" + curtag + ">")
		j = len(curtag) + 1
		for !found {
			// scan for a closing tag at the beginning of a line
			if skip := bytes.Index(data[j:], closetag); skip >= 0 {
				j += skip + len(closetag)
			} else {
				break
			}

			// see if it is the only thing on the line
			if skip := p.isEmpty(data[j:]); skip > 0 {
				// see if it is followed by a blank line/eof
				j += skip
				if j >= len(data) {
					found = true
					i = j
				} else {
					if skip := p.isEmpty(data[j:]); skip > 0 {
						j += skip
						found = true
						i = j
					}
				}
			}
		}
	*/

	// if not found, try a second pass looking for indented match
	// but not if tag is "ins" or "del" (following original Markdown.pl)
	if !found && curtag != "ins" && curtag != "del" {
		i = 1
		for i < len(data) {
			i++
			for i < len(data) && !(data[i-1] == '<' && data[i] == '/') {
				i++
			}

			if i+2+len(curtag) >= len(data) {
				break
			}

			j = p.htmlFindEnd(curtag, data[i-1:])

			if j > 0 {
				i += j - 1
				found = true
				break
			}
		}
	}

	if !found {
		return 0
	}

	// the end of the block has been found
	if doRender {
		// trim newlines
		end := backChar(data, i, '\n')
		htmlBLock := &ast.HTMLBlock{ast.Leaf{Content: data[:end]}}
		p.addBlock(htmlBLock)
		finalizeHTMLBlock(htmlBLock)
	}

	return i
}

func finalizeHTMLBlock(block *ast.HTMLBlock) {
	block.Literal = block.Content
	block.Content = nil
}

// HTML comment, lax form
func (p *Parser) htmlComment(data []byte, doRender bool) int {
	i := p.inlineHTMLComment(data)
	// needs to end with a blank line
	if j := p.isEmpty(data[i:]); j > 0 {
		size := i + j
		if doRender {
			// trim trailing newlines
			end := backChar(data, size, '\n')
			htmlBLock := &ast.HTMLBlock{ast.Leaf{Content: data[:end]}}
			p.addBlock(htmlBLock)
			finalizeHTMLBlock(htmlBLock)
		}
		return size
	}
	return 0
}

// HR, which is the only self-closing block tag considered
func (p *Parser) htmlHr(data []byte, doRender bool) int {
	if len(data) < 4 {
		return 0
	}
	if data[0] != '<' || (data[1] != 'h' && data[1] != 'H') || (data[2] != 'r' && data[2] != 'R') {
		return 0
	}
	if data[3] != ' ' && data[3] != '/' && data[3] != '>' {
		// not an <hr> tag after all; at least not a valid one
		return 0
	}
	i := 3
	for i < len(data) && data[i] != '>' && data[i] != '\n' {
		i++
	}
	if i < len(data) && data[i] == '>' {
		i++
		if j := p.isEmpty(data[i:]); j > 0 {
			size := i + j
			if doRender {
				// trim newlines
				end := backChar(data, size, '\n')
				htmlBlock := &ast.HTMLBlock{ast.Leaf{Content: data[:end]}}
				p.addBlock(htmlBlock)
				finalizeHTMLBlock(htmlBlock)
			}
			return size
		}
	}
	return 0
}

func (p *Parser) htmlFindTag(data []byte) (string, bool) {
	i := skipAlnum(data, 0)
	key := string(data[:i])
	if _, ok := blockTags[key]; ok {
		return key, true
	}
	return "", false
}

func (p *Parser) htmlFindEnd(tag string, data []byte) int {
	// assume data[0] == '<' && data[1] == '/' already tested
	if tag == "hr" {
		return 2
	}
	// check if tag is a match
	closetag := []byte("</" + tag + ">")
	if !bytes.HasPrefix(data, closetag) {
		return 0
	}
	i := len(closetag)

	// check that the rest of the line is blank
	skip := 0
	if skip = p.isEmpty(data[i:]); skip == 0 {
		return 0
	}
	i += skip
	skip = 0

	if i >= len(data) {
		return i
	}

	if p.extensions&LaxHTMLBlocks != 0 {
		return i
	}
	if skip = p.isEmpty(data[i:]); skip == 0 {
		// following line must be blank
		return 0
	}

	return i + skip
}

func (*Parser) isEmpty(data []byte) int {
	// it is okay to call isEmpty on an empty buffer
	if len(data) == 0 {
		return 0
	}

	var i int
	for i = 0; i < len(data) && data[i] != '\n'; i++ {
		if data[i] != ' ' && data[i] != '\t' {
			return 0
		}
	}
	i = skipCharN(data, i, '\n', 1)
	return i
}

func (*Parser) isHRule(data []byte) bool {
	i := 0

	// skip up to three spaces
	for i < 3 && data[i] == ' ' {
		i++
	}

	// look at the hrule char
	if data[i] != '*' && data[i] != '-' && data[i] != '_' {
		return false
	}
	c := data[i]

	// the whole line must be the char or whitespace
	n := 0
	for i < len(data) && data[i] != '\n' {
		switch {
		case data[i] == c:
			n++
		case data[i] != ' ':
			return false
		}
		i++
	}

	return n >= 3
}

// isFenceLine checks if there's a fence line (e.g., ``` or ``` go) at the beginning of data,
// and returns the end index if so, or 0 otherwise. It also returns the marker found.
// If syntax is not nil, it gets set to the syntax specified in the fence line.
func isFenceLine(data []byte, syntax *string, oldmarker string) (end int, marker string) {
	i, size := 0, 0

	n := len(data)
	// skip up to three spaces
	for i < n && i < 3 && data[i] == ' ' {
		i++
	}

	// check for the marker characters: ~ or `
	if i >= n {
		return 0, ""
	}
	if data[i] != '~' && data[i] != '`' {
		return 0, ""
	}

	c := data[i]

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

	// TODO(shurcooL): It's probably a good idea to simplify the 2 code paths here
	// into one, always get the syntax, and discard it if the caller doesn't care.
	if syntax != nil {
		syn := 0
		i = skipChar(data, i, ' ')

		if i >= n {
			if i == n {
				return i, marker
			}
			return 0, ""
		}

		syntaxStart := i

		if data[i] == '{' {
			i++
			syntaxStart++

			for i < n && data[i] != '}' && data[i] != '\n' {
				syn++
				i++
			}

			if i >= n || data[i] != '}' {
				return 0, ""
			}

			// strip all whitespace at the beginning and the end
			// of the {} block
			for syn > 0 && isSpace(data[syntaxStart]) {
				syntaxStart++
				syn--
			}

			for syn > 0 && isSpace(data[syntaxStart+syn-1]) {
				syn--
			}

			i++
		} else {
			for i < n && !isSpace(data[i]) {
				syn++
				i++
			}
		}

		*syntax = string(data[syntaxStart : syntaxStart+syn])
	}

	i = skipChar(data, i, ' ')
	if i >= n || data[i] != '\n' {
		if i == n {
			return i, marker
		}
		return 0, ""
	}
	return i + 1, marker // Take newline into account.
}

// fencedCodeBlock returns the end index if data contains a fenced code block at the beginning,
// or 0 otherwise. It writes to out if doRender is true, otherwise it has no side effects.
// If doRender is true, a final newline is mandatory to recognize the fenced code block.
func (p *Parser) fencedCodeBlock(data []byte, doRender bool) int {
	var syntax string
	beg, marker := isFenceLine(data, &syntax, "")
	if beg == 0 || beg >= len(data) {
		return 0
	}

	var work bytes.Buffer
	work.WriteString(syntax)
	work.WriteByte('\n')

	for {
		// safe to assume beg < len(data)

		// check for the end of the code block
		fenceEnd, _ := isFenceLine(data[beg:], nil, marker)
		if fenceEnd != 0 {
			beg += fenceEnd
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
			work.Write(data[beg:end])
		}
		beg = end
	}

	if doRender {
		codeBlock := &ast.CodeBlock{
			IsFenced: true,
		}
		codeBlock.Content = work.Bytes() // TODO: get rid of temp buffer

		if p.extensions&Mmark == 0 {
			p.addBlock(codeBlock)
			finalizeCodeBlock(codeBlock)
			return beg
		}

		// Check for caption and if found make it a figure.
		if captionContent, id, consumed := p.caption(data[beg:], []byte("Figure: ")); consumed > 0 {
			figure := &ast.CaptionFigure{}
			caption := &ast.Caption{}
			figure.HeadingID = id
			p.Inline(caption, captionContent)

			p.addBlock(figure)
			codeBlock.AsLeaf().Attribute = figure.AsContainer().Attribute
			p.addChild(codeBlock)
			finalizeCodeBlock(codeBlock)
			p.addChild(caption)
			p.finalize(figure)

			beg += consumed

			return beg
		}

		// Still here, normal block
		p.addBlock(codeBlock)
		finalizeCodeBlock(codeBlock)
	}

	return beg
}

func unescapeChar(str []byte) []byte {
	if str[0] == '\\' {
		return []byte{str[1]}
	}
	return []byte(html.UnescapeString(string(str)))
}

func unescapeString(str []byte) []byte {
	if reBackslashOrAmp.Match(str) {
		return reEntityOrEscapedChar.ReplaceAllFunc(str, unescapeChar)
	}
	return str
}

func finalizeCodeBlock(code *ast.CodeBlock) {
	c := code.Content
	if code.IsFenced {
		newlinePos := bytes.IndexByte(c, '\n')
		firstLine := c[:newlinePos]
		rest := c[newlinePos+1:]
		code.Info = unescapeString(bytes.Trim(firstLine, "\n"))
		code.Literal = rest
	} else {
		code.Literal = c
	}
	code.Content = nil
}

func (p *Parser) table(data []byte) int {
	i, columns, table := p.tableHeader(data)
	if i == 0 {
		return 0
	}

	p.addBlock(&ast.TableBody{})

	for i < len(data) {
		pipes, rowStart := 0, i
		for ; i < len(data) && data[i] != '\n'; i++ {
			if data[i] == '|' {
				pipes++
			}
		}

		if pipes == 0 {
			i = rowStart
			break
		}

		// include the newline in data sent to tableRow
		i = skipCharN(data, i, '\n', 1)

		if p.tableFooter(data[rowStart:i]) {
			continue
		}

		p.tableRow(data[rowStart:i], columns, false)
	}
	if captionContent, id, consumed := p.caption(data[i:], []byte("Table: ")); consumed > 0 {
		caption := &ast.Caption{}
		p.Inline(caption, captionContent)

		// Some switcheroo to re-insert the parsed table as a child of the captionfigure.
		figure := &ast.CaptionFigure{}
		figure.HeadingID = id
		table2 := &ast.Table{}
		// Retain any block level attributes.
		table2.AsContainer().Attribute = table.AsContainer().Attribute
		children := table.GetChildren()
		ast.RemoveFromTree(table)

		table2.SetChildren(children)
		ast.AppendChild(figure, table2)
		ast.AppendChild(figure, caption)

		p.addChild(figure)
		p.finalize(figure)

		i += consumed
	}

	return i
}

// check if the specified position is preceded by an odd number of backslashes
func isBackslashEscaped(data []byte, i int) bool {
	backslashes := 0
	for i-backslashes-1 >= 0 && data[i-backslashes-1] == '\\' {
		backslashes++
	}
	return backslashes&1 == 1
}

// tableHeaders parses the header. If recognized it will also add a table.
func (p *Parser) tableHeader(data []byte) (size int, columns []ast.CellAlignFlags, table ast.Node) {
	i := 0
	colCount := 1
	headerIsUnderline := true
	for i = 0; i < len(data) && data[i] != '\n'; i++ {
		if data[i] == '|' && !isBackslashEscaped(data, i) {
			colCount++
		}
		if data[i] != '-' && data[i] != ' ' && data[i] != ':' && data[i] != '|' {
			headerIsUnderline = false
		}
	}

	// doesn't look like a table header
	if colCount == 1 {
		return
	}

	// include the newline in the data sent to tableRow
	j := skipCharN(data, i, '\n', 1)
	header := data[:j]

	// column count ignores pipes at beginning or end of line
	if data[0] == '|' {
		colCount--
	}
	if i > 2 && data[i-1] == '|' && !isBackslashEscaped(data, i-1) {
		colCount--
	}

	// if the header looks like a underline, then we omit the header
	// and parse the first line again as underline
	if headerIsUnderline {
		header = nil
		i = 0
	} else {
		i++ // move past newline
	}

	columns = make([]ast.CellAlignFlags, colCount)

	// move on to the header underline
	if i >= len(data) {
		return
	}

	if data[i] == '|' && !isBackslashEscaped(data, i) {
		i++
	}
	i = skipChar(data, i, ' ')

	// each column header is of form: / *:?-+:? *|/ with # dashes + # colons >= 3
	// and trailing | optional on last column
	col := 0
	n := len(data)
	for i < n && data[i] != '\n' {
		dashes := 0

		if data[i] == ':' {
			i++
			columns[col] |= ast.TableAlignmentLeft
			dashes++
		}
		for i < n && data[i] == '-' {
			i++
			dashes++
		}
		if i < n && data[i] == ':' {
			i++
			columns[col] |= ast.TableAlignmentRight
			dashes++
		}
		for i < n && data[i] == ' ' {
			i++
		}
		if i == n {
			return
		}
		// end of column test is messy
		switch {
		case dashes < 3:
			// not a valid column
			return

		case data[i] == '|' && !isBackslashEscaped(data, i):
			// marker found, now skip past trailing whitespace
			col++
			i++
			for i < n && data[i] == ' ' {
				i++
			}

			// trailing junk found after last column
			if col >= colCount && i < len(data) && data[i] != '\n' {
				return
			}

		case (data[i] != '|' || isBackslashEscaped(data, i)) && col+1 < colCount:
			// something else found where marker was required
			return

		case data[i] == '\n':
			// marker is optional for the last column
			col++

		default:
			// trailing junk found after last column
			return
		}
	}
	if col != colCount {
		return
	}

	table = &ast.Table{}
	p.addBlock(table)
	if header != nil {
		p.addBlock(&ast.TableHeader{})
		p.tableRow(header, columns, true)
	}
	size = skipCharN(data, i, '\n', 1)
	return
}

func (p *Parser) tableRow(data []byte, columns []ast.CellAlignFlags, header bool) {
	p.addBlock(&ast.TableRow{})
	i, col := 0, 0

	if data[i] == '|' && !isBackslashEscaped(data, i) {
		i++
	}

	n := len(data)
	colspans := 0 // keep track of total colspan in this row.
	for col = 0; col < len(columns) && i < n; col++ {
		colspan := 0
		for i < n && data[i] == ' ' {
			i++
		}

		cellStart := i

		for i < n && (data[i] != '|' || isBackslashEscaped(data, i)) && data[i] != '\n' {
			i++
		}

		cellEnd := i

		// skip the end-of-cell marker, possibly taking us past end of buffer
		// each _extra_ | means a colspan
		for i < len(data) && data[i] == '|' && !isBackslashEscaped(data, i) {
			i++
			colspan++
		}
		// only colspan > 1 make sense.
		if colspan < 2 {
			colspan = 0
		}

		for cellEnd > cellStart && cellEnd-1 < n && data[cellEnd-1] == ' ' {
			cellEnd--
		}

		block := &ast.TableCell{
			IsHeader: header,
			Align:    columns[col],
			ColSpan:  colspan,
		}
		block.Content = data[cellStart:cellEnd]
		if cellStart == cellEnd && colspans > 0 {
			// an empty cell that we should ignore, it exists because of colspan
			colspans--
		} else {
			p.addBlock(block)
		}

		if colspan > 0 {
			colspans += colspan - 1
		}
	}

	// pad it out with empty columns to get the right number
	for ; col < len(columns); col++ {
		block := &ast.TableCell{
			IsHeader: header,
			Align:    columns[col],
		}
		p.addBlock(block)
	}

	// silently ignore rows with too many cells
}

// tableFooter parses the (optional) table footer.
func (p *Parser) tableFooter(data []byte) bool {
	colCount := 1
	i := 0
	n := len(data)
	for i < 3 && i < n && data[i] == ' ' { // ignore up to 3 spaces
		i++
	}
	for ; i < n && data[i] != '\n'; i++ {
		if data[i] == '|' && !isBackslashEscaped(data, i) {
			colCount++
			continue
		}
		// remaining data must be the = character
		if data[i] != '=' {
			return false
		}
	}

	// doesn't look like a table footer
	if colCount == 1 {
		return false
	}

	p.addBlock(&ast.TableFooter{})

	return true
}

// returns blockquote prefix length
func (p *Parser) quotePrefix(data []byte) int {
	i := 0
	n := len(data)
	for i < 3 && i < n && data[i] == ' ' {
		i++
	}
	if i < n && data[i] == '>' {
		if i+1 < n && data[i+1] == ' ' {
			return i + 2
		}
		return i + 1
	}
	return 0
}

// blockquote ends with at least one blank line
// followed by something without a blockquote prefix
func (p *Parser) terminateBlockquote(data []byte, beg, end int) bool {
	if p.isEmpty(data[beg:]) <= 0 {
		return false
	}
	if end >= len(data) {
		return true
	}
	return p.quotePrefix(data[end:]) == 0 && p.isEmpty(data[end:]) == 0
}

// parse a blockquote fragment
func (p *Parser) quote(data []byte) int {
	var raw bytes.Buffer
	beg, end := 0, 0
	for beg < len(data) {
		end = beg
		// Step over whole lines, collecting them. While doing that, check for
		// fenced code and if one's found, incorporate it altogether,
		// irregardless of any contents inside it
		for end < len(data) && data[end] != '\n' {
			if p.extensions&FencedCode != 0 {
				if i := p.fencedCodeBlock(data[end:], false); i > 0 {
					// -1 to compensate for the extra end++ after the loop:
					end += i - 1
					break
				}
			}
			end++
		}
		end = skipCharN(data, end, '\n', 1)
		if pre := p.quotePrefix(data[beg:]); pre > 0 {
			// skip the prefix
			beg += pre
		} else if p.terminateBlockquote(data, beg, end) {
			break
		}
		// this line is part of the blockquote
		raw.Write(data[beg:end])
		beg = end
	}

	if p.extensions&Mmark == 0 {
		block := p.addBlock(&ast.BlockQuote{})
		p.block(raw.Bytes())
		p.finalize(block)
		return end
	}

	if captionContent, id, consumed := p.caption(data[end:], []byte("Quote: ")); consumed > 0 {
		figure := &ast.CaptionFigure{}
		caption := &ast.Caption{}
		figure.HeadingID = id
		p.Inline(caption, captionContent)

		p.addBlock(figure) // this discard any attributes
		block := &ast.BlockQuote{}
		block.AsContainer().Attribute = figure.AsContainer().Attribute
		p.addChild(block)
		p.block(raw.Bytes())
		p.finalize(block)

		p.addChild(caption)
		p.finalize(figure)

		end += consumed

		return end
	}

	block := p.addBlock(&ast.BlockQuote{})
	p.block(raw.Bytes())
	p.finalize(block)

	return end
}

// returns prefix length for block code
func (p *Parser) codePrefix(data []byte) int {
	n := len(data)
	if n >= 1 && data[0] == '\t' {
		return 1
	}
	if n >= 4 && data[3] == ' ' && data[2] == ' ' && data[1] == ' ' && data[0] == ' ' {
		return 4
	}
	return 0
}

func (p *Parser) code(data []byte) int {
	var work bytes.Buffer

	i := 0
	for i < len(data) {
		beg := i

		i = skipUntilChar(data, i, '\n')
		i = skipCharN(data, i, '\n', 1)

		blankline := p.isEmpty(data[beg:i]) > 0
		if pre := p.codePrefix(data[beg:i]); pre > 0 {
			beg += pre
		} else if !blankline {
			// non-empty, non-prefixed line breaks the pre
			i = beg
			break
		}

		// verbatim copy to the working buffer
		if blankline {
			work.WriteByte('\n')
		} else {
			work.Write(data[beg:i])
		}
	}

	// trim all the \n off the end of work
	workbytes := work.Bytes()

	eol := backChar(workbytes, len(workbytes), '\n')

	if eol != len(workbytes) {
		work.Truncate(eol)
	}

	work.WriteByte('\n')

	codeBlock := &ast.CodeBlock{
		IsFenced: false,
	}
	// TODO: get rid of temp buffer
	codeBlock.Content = work.Bytes()
	p.addBlock(codeBlock)
	finalizeCodeBlock(codeBlock)

	return i
}

// returns unordered list item prefix
func (p *Parser) uliPrefix(data []byte) int {
	// start with up to 3 spaces
	i := skipCharN(data, 0, ' ', 3)

	if i >= len(data)-1 {
		return 0
	}
	// need one of {'*', '+', '-'} followed by a space or a tab
	if (data[i] != '*' && data[i] != '+' && data[i] != '-') ||
		(data[i+1] != ' ' && data[i+1] != '\t') {
		return 0
	}
	return i + 2
}

// returns ordered list item prefix
func (p *Parser) oliPrefix(data []byte) int {
	// start with up to 3 spaces
	i := skipCharN(data, 0, ' ', 3)

	// count the digits
	start := i
	for i < len(data) && data[i] >= '0' && data[i] <= '9' {
		i++
	}
	if start == i || i >= len(data)-1 {
		return 0
	}

	// we need >= 1 digits followed by a dot and a space or a tab
	if data[i] != '.' || !(data[i+1] == ' ' || data[i+1] == '\t') {
		return 0
	}
	return i + 2
}

// returns definition list item prefix
func (p *Parser) dliPrefix(data []byte) int {
	if len(data) < 2 {
		return 0
	}
	// need a ':' followed by a space or a tab
	if data[0] != ':' || !(data[1] == ' ' || data[1] == '\t') {
		return 0
	}
	i := skipChar(data, 0, ' ')
	return i + 2
}

// parse ordered or unordered list block
func (p *Parser) list(data []byte, flags ast.ListType, start int) int {
	i := 0
	flags |= ast.ListItemBeginningOfList
	list := &ast.List{
		ListFlags: flags,
		Tight:     true,
		Start:     start,
	}
	block := p.addBlock(list)

	for i < len(data) {
		skip := p.listItem(data[i:], &flags)
		if flags&ast.ListItemContainsBlock != 0 {
			list.Tight = false
		}
		i += skip
		if skip == 0 || flags&ast.ListItemEndOfList != 0 {
			break
		}
		flags &= ^ast.ListItemBeginningOfList
	}

	above := block.GetParent()
	finalizeList(list)
	p.tip = above
	return i
}

// Returns true if the list item is not the same type as its parent list
func (p *Parser) listTypeChanged(data []byte, flags *ast.ListType) bool {
	if p.dliPrefix(data) > 0 && *flags&ast.ListTypeDefinition == 0 {
		return true
	} else if p.oliPrefix(data) > 0 && *flags&ast.ListTypeOrdered == 0 {
		return true
	} else if p.uliPrefix(data) > 0 && (*flags&ast.ListTypeOrdered != 0 || *flags&ast.ListTypeDefinition != 0) {
		return true
	}
	return false
}

// Returns true if block ends with a blank line, descending if needed
// into lists and sublists.
func endsWithBlankLine(block ast.Node) bool {
	// TODO: figure this out. Always false now.
	for block != nil {
		//if block.lastLineBlank {
		//return true
		//}
		switch block.(type) {
		case *ast.List, *ast.ListItem:
			block = ast.GetLastChild(block)
		default:
			return false
		}
	}
	return false
}

func finalizeList(list *ast.List) {
	items := list.Parent.GetChildren()
	lastItemIdx := len(items) - 1
	for i, item := range items {
		isLastItem := i == lastItemIdx
		// check for non-final list item ending with blank line:
		if !isLastItem && endsWithBlankLine(item) {
			list.Tight = false
			break
		}
		// recurse into children of list item, to see if there are spaces
		// between any of them:
		subItems := item.GetParent().GetChildren()
		lastSubItemIdx := len(subItems) - 1
		for j, subItem := range subItems {
			isLastSubItem := j == lastSubItemIdx
			if (!isLastItem || !isLastSubItem) && endsWithBlankLine(subItem) {
				list.Tight = false
				break
			}
		}
	}
}

// Parse a single list item.
// Assumes initial prefix is already removed if this is a sublist.
func (p *Parser) listItem(data []byte, flags *ast.ListType) int {
	// keep track of the indentation of the first line
	itemIndent := 0
	if data[0] == '\t' {
		itemIndent += 4
	} else {
		for itemIndent < 3 && data[itemIndent] == ' ' {
			itemIndent++
		}
	}

	var bulletChar byte = '*'
	i := p.uliPrefix(data)
	if i == 0 {
		i = p.oliPrefix(data)
	} else {
		bulletChar = data[i-2]
	}
	if i == 0 {
		i = p.dliPrefix(data)
		// reset definition term flag
		if i > 0 {
			*flags &= ^ast.ListTypeTerm
		}
	}
	if i == 0 {
		// if in definition list, set term flag and continue
		if *flags&ast.ListTypeDefinition != 0 {
			*flags |= ast.ListTypeTerm
		} else {
			return 0
		}
	}

	// skip leading whitespace on first line
	i = skipChar(data, i, ' ')

	// find the end of the line
	line := i
	for i > 0 && i < len(data) && data[i-1] != '\n' {
		i++
	}

	// get working buffer
	var raw bytes.Buffer

	// put the first line into the working buffer
	raw.Write(data[line:i])
	line = i

	// process the following lines
	containsBlankLine := false
	sublist := 0

gatherlines:
	for line < len(data) {
		i++

		// find the end of this line
		for i < len(data) && data[i-1] != '\n' {
			i++
		}

		// if it is an empty line, guess that it is part of this item
		// and move on to the next line
		if p.isEmpty(data[line:i]) > 0 {
			containsBlankLine = true
			line = i
			continue
		}

		// calculate the indentation
		indent := 0
		indentIndex := 0
		if data[line] == '\t' {
			indentIndex++
			indent += 4
		} else {
			for indent < 4 && line+indent < i && data[line+indent] == ' ' {
				indent++
				indentIndex++
			}
		}

		chunk := data[line+indentIndex : i]

		// evaluate how this line fits in
		switch {
		// is this a nested list item?
		case (p.uliPrefix(chunk) > 0 && !p.isHRule(chunk)) || p.oliPrefix(chunk) > 0 || p.dliPrefix(chunk) > 0:

			// to be a nested list, it must be indented more
			// if not, it is either a different kind of list
			// or the next item in the same list
			if indent <= itemIndent {
				if p.listTypeChanged(chunk, flags) {
					*flags |= ast.ListItemEndOfList
				} else if containsBlankLine {
					*flags |= ast.ListItemContainsBlock
				}

				break gatherlines
			}

			if containsBlankLine {
				*flags |= ast.ListItemContainsBlock
			}

			// is this the first item in the nested list?
			if sublist == 0 {
				sublist = raw.Len()
				// in the case of dliPrefix we are too late and need to search back for the definition item, which
				// should be on the previous line, we then adjust sublist to start there.
				if p.dliPrefix(chunk) > 0 {
					sublist = backUntilChar(raw.Bytes(), raw.Len()-1, '\n')
				}
			}

			// is this a nested prefix heading?
		case p.isPrefixHeading(chunk), p.isPrefixSpecialHeading(chunk):
			// if the heading is not indented, it is not nested in the list
			// and thus ends the list
			if containsBlankLine && indent < 4 {
				*flags |= ast.ListItemEndOfList
				break gatherlines
			}
			*flags |= ast.ListItemContainsBlock

		// anything following an empty line is only part
		// of this item if it is indented 4 spaces
		// (regardless of the indentation of the beginning of the item)
		case containsBlankLine && indent < 4:
			if *flags&ast.ListTypeDefinition != 0 && i < len(data)-1 {
				// is the next item still a part of this list?
				next := i
				for next < len(data) && data[next] != '\n' {
					next++
				}
				for next < len(data)-1 && data[next] == '\n' {
					next++
				}
				if i < len(data)-1 && data[i] != ':' && next < len(data)-1 && data[next] != ':' {
					*flags |= ast.ListItemEndOfList
				}
			} else {
				*flags |= ast.ListItemEndOfList
			}
			break gatherlines

		// a blank line means this should be parsed as a block
		case containsBlankLine:
			raw.WriteByte('\n')
			*flags |= ast.ListItemContainsBlock
		}

		// if this line was preceded by one or more blanks,
		// re-introduce the blank into the buffer
		if containsBlankLine {
			containsBlankLine = false
			raw.WriteByte('\n')
		}

		// add the line into the working buffer without prefix
		raw.Write(data[line+indentIndex : i])

		line = i
	}

	rawBytes := raw.Bytes()

	listItem := &ast.ListItem{
		ListFlags:  *flags,
		Tight:      false,
		BulletChar: bulletChar,
		Delimiter:  '.', // Only '.' is possible in Markdown, but ')' will also be possible in CommonMark
	}
	p.addBlock(listItem)

	// render the contents of the list item
	if *flags&ast.ListItemContainsBlock != 0 && *flags&ast.ListTypeTerm == 0 {
		// intermediate render of block item, except for definition term
		if sublist > 0 {
			p.block(rawBytes[:sublist])
			p.block(rawBytes[sublist:])
		} else {
			p.block(rawBytes)
		}
	} else {
		// intermediate render of inline item
		para := &ast.Paragraph{}
		if sublist > 0 {
			para.Content = rawBytes[:sublist]
		} else {
			para.Content = rawBytes
		}
		p.addChild(para)
		if sublist > 0 {
			p.block(rawBytes[sublist:])
		}
	}
	return line
}

// render a single paragraph that has already been parsed out
func (p *Parser) renderParagraph(data []byte) {
	if len(data) == 0 {
		return
	}

	// trim leading spaces
	beg := skipChar(data, 0, ' ')

	end := len(data)
	// trim trailing newline
	if data[len(data)-1] == '\n' {
		end--
	}

	// trim trailing spaces
	for end > beg && data[end-1] == ' ' {
		end--
	}
	para := &ast.Paragraph{}
	para.Content = data[beg:end]
	p.addBlock(para)
}

// blockMath handle block surround with $$
func (p *Parser) blockMath(data []byte) int {
	if len(data) <= 4 || data[0] != '$' || data[1] != '$' || data[2] == '$' {
		return 0
	}

	// find next $$
	var end int
	for end = 2; end+1 < len(data) && (data[end] != '$' || data[end+1] != '$'); end++ {
	}

	// $$ not match
	if end+1 == len(data) {
		return 0
	}

	// render the display math
	mathBlock := &ast.MathBlock{}
	mathBlock.Literal = data[2:end]
	p.addBlock(mathBlock)

	return end + 2
}

func (p *Parser) paragraph(data []byte) int {
	// prev: index of 1st char of previous line
	// line: index of 1st char of current line
	// i: index of cursor/end of current line
	var prev, line, i int
	tabSize := tabSizeDefault
	if p.extensions&TabSizeEight != 0 {
		tabSize = tabSizeDouble
	}
	// keep going until we find something to mark the end of the paragraph
	for i < len(data) {
		// mark the beginning of the current line
		prev = line
		current := data[i:]
		line = i

		// did we find a reference or a footnote? If so, end a paragraph
		// preceding it and report that we have consumed up to the end of that
		// reference:
		if refEnd := isReference(p, current, tabSize); refEnd > 0 {
			p.renderParagraph(data[:i])
			return i + refEnd
		}

		// did we find a blank line marking the end of the paragraph?
		if n := p.isEmpty(current); n > 0 {
			// did this blank line followed by a definition list item?
			if p.extensions&DefinitionLists != 0 {
				if i < len(data)-1 && data[i+1] == ':' {
					listLen := p.list(data[prev:], ast.ListTypeDefinition, 0)
					return prev + listLen
				}
			}

			p.renderParagraph(data[:i])
			return i + n
		}

		// an underline under some text marks a heading, so our paragraph ended on prev line
		if i > 0 {
			if level := p.isUnderlinedHeading(current); level > 0 {
				// render the paragraph
				p.renderParagraph(data[:prev])

				// ignore leading and trailing whitespace
				eol := i - 1
				for prev < eol && data[prev] == ' ' {
					prev++
				}
				for eol > prev && data[eol-1] == ' ' {
					eol--
				}

				id := ""
				if p.extensions&AutoHeadingIDs != 0 {
					id = sanitizeAnchorName(string(data[prev:eol]))
				}

				block := &ast.Heading{
					Level:     level,
					HeadingID: id,
				}
				block.Content = data[prev:eol]
				p.addBlock(block)

				// find the end of the underline
				return skipUntilChar(data, i, '\n')
			}
		}

		// if the next line starts a block of HTML, then the paragraph ends here
		if p.extensions&LaxHTMLBlocks != 0 {
			if data[i] == '<' && p.html(current, false) > 0 {
				// rewind to before the HTML block
				p.renderParagraph(data[:i])
				return i
			}
		}

		// if there's a prefixed heading or a horizontal rule after this, paragraph is over
		if p.isPrefixHeading(current) || p.isPrefixSpecialHeading(current) || p.isHRule(current) {
			p.renderParagraph(data[:i])
			return i
		}

		// if there's a fenced code block, paragraph is over
		if p.extensions&FencedCode != 0 {
			if p.fencedCodeBlock(current, false) > 0 {
				p.renderParagraph(data[:i])
				return i
			}
		}

		// if there's a figure block, paragraph is over
		if p.extensions&Mmark != 0 {
			if p.figureBlock(current, false) > 0 {
				p.renderParagraph(data[:i])
				return i
			}
		}

		// if there's a definition list item, prev line is a definition term
		if p.extensions&DefinitionLists != 0 {
			if p.dliPrefix(current) != 0 {
				ret := p.list(data[prev:], ast.ListTypeDefinition, 0)
				return ret + prev
			}
		}

		// if there's a list after this, paragraph is over
		if p.extensions&NoEmptyLineBeforeBlock != 0 {
			if p.uliPrefix(current) != 0 ||
				p.oliPrefix(current) != 0 ||
				p.quotePrefix(current) != 0 ||
				p.codePrefix(current) != 0 {
				p.renderParagraph(data[:i])
				return i
			}
		}

		// otherwise, scan to the beginning of the next line
		nl := bytes.IndexByte(data[i:], '\n')
		if nl >= 0 {
			i += nl + 1
		} else {
			i += len(data[i:])
		}
	}

	p.renderParagraph(data[:i])
	return i
}

// skipChar advances i as long as data[i] == c
func skipChar(data []byte, i int, c byte) int {
	n := len(data)
	for i < n && data[i] == c {
		i++
	}
	return i
}

// like skipChar but only skips up to max characters
func skipCharN(data []byte, i int, c byte, max int) int {
	n := len(data)
	for i < n && max > 0 && data[i] == c {
		i++
		max--
	}
	return i
}

// skipUntilChar advances i as long as data[i] != c
func skipUntilChar(data []byte, i int, c byte) int {
	n := len(data)
	for i < n && data[i] != c {
		i++
	}
	return i
}

func skipAlnum(data []byte, i int) int {
	n := len(data)
	for i < n && isAlnum(data[i]) {
		i++
	}
	return i
}

func skipSpace(data []byte, i int) int {
	n := len(data)
	for i < n && isSpace(data[i]) {
		i++
	}
	return i
}

func backChar(data []byte, i int, c byte) int {
	for i > 0 && data[i-1] == c {
		i--
	}
	return i
}

func backUntilChar(data []byte, i int, c byte) int {
	for i > 0 && data[i-1] != c {
		i--
	}
	return i
}
