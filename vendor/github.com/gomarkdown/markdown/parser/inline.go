package parser

import (
	"bytes"
	"regexp"
	"strconv"

	"github.com/gomarkdown/markdown/ast"
)

// Parsing of inline elements

var (
	urlRe    = `((https?|ftp):\/\/|\/)[-A-Za-z0-9+&@#\/%?=~_|!:,.;\(\)]+`
	anchorRe = regexp.MustCompile(`^(<a\shref="` + urlRe + `"(\stitle="[^"<>]+")?\s?>` + urlRe + `<\/a>)`)

	// TODO: improve this regexp to catch all possible entities:
	htmlEntityRe = regexp.MustCompile(`&[a-z]{2,5};`)
)

// Inline parses text within a block.
// Each function returns the number of consumed chars.
func (p *Parser) Inline(currBlock ast.Node, data []byte) {
	// handlers might call us recursively: enforce a maximum depth
	if p.nesting >= p.maxNesting || len(data) == 0 {
		return
	}
	p.nesting++
	beg, end := 0, 0

	n := len(data)
	for end < n {
		handler := p.inlineCallback[data[end]]
		if handler == nil {
			end++
			continue
		}
		consumed, node := handler(p, data, end)
		if consumed == 0 {
			// no action from the callback
			end++
			continue
		}
		// copy inactive chars into the output
		ast.AppendChild(currBlock, newTextNode(data[beg:end]))
		if node != nil {
			ast.AppendChild(currBlock, node)
		}
		beg = end + consumed
		end = beg
	}

	if beg < n {
		if data[end-1] == '\n' {
			end--
		}
		ast.AppendChild(currBlock, newTextNode(data[beg:end]))
	}
	p.nesting--
}

// single and double emphasis parsing
func emphasis(p *Parser, data []byte, offset int) (int, ast.Node) {
	data = data[offset:]
	c := data[0]

	n := len(data)
	if n > 2 && data[1] != c {
		// whitespace cannot follow an opening emphasis;
		// strikethrough only takes two characters '~~'
		if isSpace(data[1]) {
			return 0, nil
		}
		if p.extensions&SuperSubscript != 0 && c == '~' {
			// potential subscript, no spaces, except when escaped, helperEmphasis does
			// not check that for us, so walk the bytes and check.
			ret := skipUntilChar(data[1:], 0, c)
			if ret == 0 {
				return 0, nil
			}
			ret++ // we started with data[1:] above.
			for i := 1; i < ret; i++ {
				if isSpace(data[i]) && !isEscape(data, i) {
					return 0, nil
				}
			}
			sub := &ast.Subscript{}
			sub.Literal = data[1:ret]
			return ret + 1, sub
		}
		ret, node := helperEmphasis(p, data[1:], c)
		if ret == 0 {
			return 0, nil
		}

		return ret + 1, node
	}

	if n > 3 && data[1] == c && data[2] != c {
		if isSpace(data[2]) {
			return 0, nil
		}
		ret, node := helperDoubleEmphasis(p, data[2:], c)
		if ret == 0 {
			return 0, nil
		}

		return ret + 2, node
	}

	if n > 4 && data[1] == c && data[2] == c && data[3] != c {
		if c == '~' || isSpace(data[3]) {
			return 0, nil
		}
		ret, node := helperTripleEmphasis(p, data, 3, c)
		if ret == 0 {
			return 0, nil
		}

		return ret + 3, node
	}

	return 0, nil
}

func codeSpan(p *Parser, data []byte, offset int) (int, ast.Node) {
	data = data[offset:]

	// count the number of backticks in the delimiter
	nb := skipChar(data, 0, '`')

	// find the next delimiter
	i, end := 0, 0
	for end = nb; end < len(data) && i < nb; end++ {
		if data[end] == '`' {
			i++
		} else {
			i = 0
		}
	}

	// no matching delimiter?
	if i < nb && end >= len(data) {
		return 0, nil
	}

	// trim outside whitespace
	fBegin := nb
	for fBegin < end && data[fBegin] == ' ' {
		fBegin++
	}

	fEnd := end - nb
	for fEnd > fBegin && data[fEnd-1] == ' ' {
		fEnd--
	}

	// render the code span
	if fBegin != fEnd {
		code := &ast.Code{}
		code.Literal = data[fBegin:fEnd]
		return end, code
	}

	return end, nil
}

// newline preceded by two spaces becomes <br>
func maybeLineBreak(p *Parser, data []byte, offset int) (int, ast.Node) {
	origOffset := offset
	offset = skipChar(data, offset, ' ')

	if offset < len(data) && data[offset] == '\n' {
		if offset-origOffset >= 2 {
			return offset - origOffset + 1, &ast.Hardbreak{}
		}
		return offset - origOffset, nil
	}
	return 0, nil
}

// newline without two spaces works when HardLineBreak is enabled
func lineBreak(p *Parser, data []byte, offset int) (int, ast.Node) {
	if p.extensions&HardLineBreak != 0 {
		return 1, &ast.Hardbreak{}
	}
	return 0, nil
}

type linkType int

const (
	linkNormal linkType = iota
	linkImg
	linkDeferredFootnote
	linkInlineFootnote
	linkCitation
)

func isReferenceStyleLink(data []byte, pos int, t linkType) bool {
	if t == linkDeferredFootnote {
		return false
	}
	return pos < len(data)-1 && data[pos] == '[' && data[pos+1] != '^'
}

func maybeImage(p *Parser, data []byte, offset int) (int, ast.Node) {
	if offset < len(data)-1 && data[offset+1] == '[' {
		return link(p, data, offset)
	}
	return 0, nil
}

func maybeInlineFootnoteOrSuper(p *Parser, data []byte, offset int) (int, ast.Node) {
	if offset < len(data)-1 && data[offset+1] == '[' {
		return link(p, data, offset)
	}

	if p.extensions&SuperSubscript != 0 {
		ret := skipUntilChar(data[offset:], 1, '^')
		if ret == 0 {
			return 0, nil
		}
		for i := offset; i < offset+ret; i++ {
			if isSpace(data[i]) && !isEscape(data, i) {
				return 0, nil
			}
		}
		sup := &ast.Superscript{}
		sup.Literal = data[offset+1 : offset+ret]
		return ret + 1, sup
	}

	return 0, nil
}

// '[': parse a link or an image or a footnote or a citation
func link(p *Parser, data []byte, offset int) (int, ast.Node) {
	// no links allowed inside regular links, footnote, and deferred footnotes
	if p.insideLink && (offset > 0 && data[offset-1] == '[' || len(data)-1 > offset && data[offset+1] == '^') {
		return 0, nil
	}

	var t linkType
	switch {
	// special case: ![^text] == deferred footnote (that follows something with
	// an exclamation point)
	case p.extensions&Footnotes != 0 && len(data)-1 > offset && data[offset+1] == '^':
		t = linkDeferredFootnote
	// ![alt] == image
	case offset >= 0 && data[offset] == '!':
		t = linkImg
		offset++
	// [@citation], [@-citation], [@?citation], [@!citation]
	case p.extensions&Mmark != 0 && len(data)-1 > offset && data[offset+1] == '@':
		t = linkCitation
	// [text] == regular link
	// ^[text] == inline footnote
	// [^refId] == deferred footnote
	case p.extensions&Footnotes != 0:
		if offset >= 0 && data[offset] == '^' {
			t = linkInlineFootnote
			offset++
		} else if len(data)-1 > offset && data[offset+1] == '^' {
			t = linkDeferredFootnote
		}
	default:
		t = linkNormal
	}

	data = data[offset:]

	if t == linkCitation {
		return citation(p, data, 0)
	}

	var (
		i                               = 1
		noteID                          int
		title, link, linkID, altContent []byte
		textHasNl                       = false
	)

	if t == linkDeferredFootnote {
		i++
	}

	// look for the matching closing bracket
	for level := 1; level > 0 && i < len(data); i++ {
		switch {
		case data[i] == '\n':
			textHasNl = true

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

	txtE := i
	i++
	var footnoteNode ast.Node

	// skip any amount of whitespace or newline
	// (this is much more lax than original markdown syntax)
	i = skipSpace(data, i)

	// inline style link
	switch {
	case i < len(data) && data[i] == '(':
		// skip initial whitespace
		i++

		i = skipSpace(data, i)

		linkB := i
		brace := 0

		// look for link end: ' " )
	findlinkend:
		for i < len(data) {
			switch {
			case data[i] == '\\':
				i += 2

			case data[i] == '(':
				brace++
				i++

			case data[i] == ')':
				if brace <= 0 {
					break findlinkend
				}
				brace--
				i++

			case data[i] == '\'' || data[i] == '"':
				break findlinkend

			default:
				i++
			}
		}

		if i >= len(data) {
			return 0, nil
		}
		linkE := i

		// look for title end if present
		titleB, titleE := 0, 0
		if data[i] == '\'' || data[i] == '"' {
			i++
			titleB = i
			titleEndCharFound := false

		findtitleend:
			for i < len(data) {
				switch {
				case data[i] == '\\':
					i++

				case data[i] == data[titleB-1]: // matching title delimiter
					titleEndCharFound = true

				case titleEndCharFound && data[i] == ')':
					break findtitleend
				}
				i++
			}

			if i >= len(data) {
				return 0, nil
			}

			// skip whitespace after title
			titleE = i - 1
			for titleE > titleB && isSpace(data[titleE]) {
				titleE--
			}

			// check for closing quote presence
			if data[titleE] != '\'' && data[titleE] != '"' {
				titleB, titleE = 0, 0
				linkE = i
			}
		}

		// remove whitespace at the end of the link
		for linkE > linkB && isSpace(data[linkE-1]) {
			linkE--
		}

		// remove optional angle brackets around the link
		if data[linkB] == '<' {
			linkB++
		}
		if data[linkE-1] == '>' {
			linkE--
		}

		// build escaped link and title
		if linkE > linkB {
			link = data[linkB:linkE]
		}

		if titleE > titleB {
			title = data[titleB:titleE]
		}

		i++

	// reference style link
	case isReferenceStyleLink(data, i, t):
		var id []byte
		altContentConsidered := false

		// look for the id
		i++
		linkB := i
		i = skipUntilChar(data, i, ']')

		if i >= len(data) {
			return 0, nil
		}
		linkE := i

		// find the reference
		if linkB == linkE {
			if textHasNl {
				var b bytes.Buffer

				for j := 1; j < txtE; j++ {
					switch {
					case data[j] != '\n':
						b.WriteByte(data[j])
					case data[j-1] != ' ':
						b.WriteByte(' ')
					}
				}

				id = b.Bytes()
			} else {
				id = data[1:txtE]
				altContentConsidered = true
			}
		} else {
			id = data[linkB:linkE]
		}

		// find the reference with matching id
		lr, ok := p.getRef(string(id))
		if !ok {
			return 0, nil
		}

		// keep link and title from reference
		linkID = id
		link = lr.link
		title = lr.title
		if altContentConsidered {
			altContent = lr.text
		}
		i++

	// shortcut reference style link or reference or inline footnote
	default:
		var id []byte

		// craft the id
		if textHasNl {
			var b bytes.Buffer

			for j := 1; j < txtE; j++ {
				switch {
				case data[j] != '\n':
					b.WriteByte(data[j])
				case data[j-1] != ' ':
					b.WriteByte(' ')
				}
			}

			id = b.Bytes()
		} else {
			if t == linkDeferredFootnote {
				id = data[2:txtE] // get rid of the ^
			} else {
				id = data[1:txtE]
			}
		}

		footnoteNode = &ast.ListItem{}
		if t == linkInlineFootnote {
			// create a new reference
			noteID = len(p.notes) + 1

			var fragment []byte
			if len(id) > 0 {
				if len(id) < 16 {
					fragment = make([]byte, len(id))
				} else {
					fragment = make([]byte, 16)
				}
				copy(fragment, slugify(id))
			} else {
				fragment = append([]byte("footnote-"), []byte(strconv.Itoa(noteID))...)
			}

			ref := &reference{
				noteID:   noteID,
				hasBlock: false,
				link:     fragment,
				title:    id,
				footnote: footnoteNode,
			}

			p.notes = append(p.notes, ref)
			p.refsRecord[string(ref.link)] = struct{}{}

			link = ref.link
			title = ref.title
		} else {
			// find the reference with matching id
			lr, ok := p.getRef(string(id))
			if !ok {
				return 0, nil
			}

			if t == linkDeferredFootnote && !p.isFootnote(lr) {
				lr.noteID = len(p.notes) + 1
				lr.footnote = footnoteNode
				p.notes = append(p.notes, lr)
				p.refsRecord[string(lr.link)] = struct{}{}
			}

			// keep link and title from reference
			link = lr.link
			// if inline footnote, title == footnote contents
			title = lr.title
			noteID = lr.noteID
			if len(lr.text) > 0 {
				altContent = lr.text
			}
		}

		// rewind the whitespace
		i = txtE + 1
	}

	var uLink []byte
	if t == linkNormal || t == linkImg {
		if len(link) > 0 {
			var uLinkBuf bytes.Buffer
			unescapeText(&uLinkBuf, link)
			uLink = uLinkBuf.Bytes()
		}

		// links need something to click on and somewhere to go
		if len(uLink) == 0 || (t == linkNormal && txtE <= 1) {
			return 0, nil
		}
	}

	// call the relevant rendering function
	switch t {
	case linkNormal:
		link := &ast.Link{
			Destination: normalizeURI(uLink),
			Title:       title,
			DeferredID:  linkID,
		}
		if len(altContent) > 0 {
			ast.AppendChild(link, newTextNode(altContent))
		} else {
			// links cannot contain other links, so turn off link parsing
			// temporarily and recurse
			insideLink := p.insideLink
			p.insideLink = true
			p.Inline(link, data[1:txtE])
			p.insideLink = insideLink
		}
		return i, link

	case linkImg:
		image := &ast.Image{
			Destination: uLink,
			Title:       title,
		}
		ast.AppendChild(image, newTextNode(data[1:txtE]))
		return i + 1, image

	case linkInlineFootnote, linkDeferredFootnote:
		link := &ast.Link{
			Destination: link,
			Title:       title,
			NoteID:      noteID,
			Footnote:    footnoteNode,
		}
		if t == linkDeferredFootnote {
			link.DeferredID = data[2:txtE]
		}
		if t == linkInlineFootnote {
			i++
		}
		return i, link

	default:
		return 0, nil
	}
}

func (p *Parser) inlineHTMLComment(data []byte) int {
	if len(data) < 5 {
		return 0
	}
	if data[0] != '<' || data[1] != '!' || data[2] != '-' || data[3] != '-' {
		return 0
	}
	i := 5
	// scan for an end-of-comment marker, across lines if necessary
	for i < len(data) && !(data[i-2] == '-' && data[i-1] == '-' && data[i] == '>') {
		i++
	}
	// no end-of-comment marker
	if i >= len(data) {
		return 0
	}
	return i + 1
}

func stripMailto(link []byte) []byte {
	if bytes.HasPrefix(link, []byte("mailto://")) {
		return link[9:]
	} else if bytes.HasPrefix(link, []byte("mailto:")) {
		return link[7:]
	} else {
		return link
	}
}

// autolinkType specifies a kind of autolink that gets detected.
type autolinkType int

// These are the possible flag values for the autolink renderer.
const (
	notAutolink autolinkType = iota
	normalAutolink
	emailAutolink
)

// '<' when tags or autolinks are allowed
func leftAngle(p *Parser, data []byte, offset int) (int, ast.Node) {
	data = data[offset:]

	if p.extensions&Mmark != 0 {
		id, consumed := IsCallout(data)
		if consumed > 0 {
			node := &ast.Callout{}
			node.ID = id
			return consumed, node
		}
	}

	altype, end := tagLength(data)
	if size := p.inlineHTMLComment(data); size > 0 {
		end = size
	}
	if end <= 2 {
		return end, nil
	}
	if altype == notAutolink {
		htmlTag := &ast.HTMLSpan{}
		htmlTag.Literal = data[:end]
		return end, htmlTag
	}

	var uLink bytes.Buffer
	unescapeText(&uLink, data[1:end+1-2])
	if uLink.Len() <= 0 {
		return end, nil
	}
	link := uLink.Bytes()
	node := &ast.Link{
		Destination: link,
	}
	if altype == emailAutolink {
		node.Destination = append([]byte("mailto:"), link...)
	}
	ast.AppendChild(node, newTextNode(stripMailto(link)))
	return end, node
}

// '\\' backslash escape
var escapeChars = []byte("\\`*_{}[]()#+-.!:|&<>~^")

func escape(p *Parser, data []byte, offset int) (int, ast.Node) {
	data = data[offset:]

	if len(data) <= 1 {
		return 2, nil
	}

	if p.extensions&NonBlockingSpace != 0 && data[1] == ' ' {
		return 2, &ast.NonBlockingSpace{}
	}

	if p.extensions&BackslashLineBreak != 0 && data[1] == '\n' {
		return 2, &ast.Hardbreak{}
	}

	if bytes.IndexByte(escapeChars, data[1]) < 0 {
		return 0, nil
	}

	return 2, newTextNode(data[1:2])
}

func unescapeText(ob *bytes.Buffer, src []byte) {
	i := 0
	for i < len(src) {
		org := i
		for i < len(src) && src[i] != '\\' {
			i++
		}

		if i > org {
			ob.Write(src[org:i])
		}

		if i+1 >= len(src) {
			break
		}

		ob.WriteByte(src[i+1])
		i += 2
	}
}

// '&' escaped when it doesn't belong to an entity
// valid entities are assumed to be anything matching &#?[A-Za-z0-9]+;
func entity(p *Parser, data []byte, offset int) (int, ast.Node) {
	data = data[offset:]

	end := skipCharN(data, 1, '#', 1)
	end = skipAlnum(data, end)

	if end < len(data) && data[end] == ';' {
		end++ // real entity
	} else {
		return 0, nil // lone '&'
	}

	ent := data[:end]
	// undo &amp; escaping or it will be converted to &amp;amp; by another
	// escaper in the renderer
	if bytes.Equal(ent, []byte("&amp;")) {
		ent = []byte{'&'}
	}

	return end, newTextNode(ent)
}

func linkEndsWithEntity(data []byte, linkEnd int) bool {
	entityRanges := htmlEntityRe.FindAllIndex(data[:linkEnd], -1)
	return entityRanges != nil && entityRanges[len(entityRanges)-1][1] == linkEnd
}

// hasPrefixCaseInsensitive is a custom implementation of
//     strings.HasPrefix(strings.ToLower(s), prefix)
// we rolled our own because ToLower pulls in a huge machinery of lowercasing
// anything from Unicode and that's very slow. Since this func will only be
// used on ASCII protocol prefixes, we can take shortcuts.
func hasPrefixCaseInsensitive(s, prefix []byte) bool {
	if len(s) < len(prefix) {
		return false
	}
	delta := byte('a' - 'A')
	for i, b := range prefix {
		if b != s[i] && b != s[i]+delta {
			return false
		}
	}
	return true
}

var protocolPrefixes = [][]byte{
	[]byte("http://"),
	[]byte("https://"),
	[]byte("ftp://"),
	[]byte("file://"),
	[]byte("mailto:"),
}

const shortestPrefix = 6 // len("ftp://"), the shortest of the above

func maybeAutoLink(p *Parser, data []byte, offset int) (int, ast.Node) {
	// quick check to rule out most false hits
	if p.insideLink || len(data) < offset+shortestPrefix {
		return 0, nil
	}
	for _, prefix := range protocolPrefixes {
		endOfHead := offset + 8 // 8 is the len() of the longest prefix
		if endOfHead > len(data) {
			endOfHead = len(data)
		}
		if hasPrefixCaseInsensitive(data[offset:endOfHead], prefix) {
			return autoLink(p, data, offset)
		}
	}
	return 0, nil
}

func autoLink(p *Parser, data []byte, offset int) (int, ast.Node) {
	// Now a more expensive check to see if we're not inside an anchor element
	anchorStart := offset
	offsetFromAnchor := 0
	for anchorStart > 0 && data[anchorStart] != '<' {
		anchorStart--
		offsetFromAnchor++
	}

	anchorStr := anchorRe.Find(data[anchorStart:])
	if anchorStr != nil {
		anchorClose := &ast.HTMLSpan{}
		anchorClose.Literal = anchorStr[offsetFromAnchor:]
		return len(anchorStr) - offsetFromAnchor, anchorClose
	}

	// scan backward for a word boundary
	rewind := 0
	for offset-rewind > 0 && rewind <= 7 && isLetter(data[offset-rewind-1]) {
		rewind++
	}
	if rewind > 6 { // longest supported protocol is "mailto" which has 6 letters
		return 0, nil
	}

	origData := data
	data = data[offset-rewind:]

	if !isSafeLink(data) {
		return 0, nil
	}

	linkEnd := 0
	for linkEnd < len(data) && !isEndOfLink(data[linkEnd]) {
		linkEnd++
	}

	// Skip punctuation at the end of the link
	if (data[linkEnd-1] == '.' || data[linkEnd-1] == ',') && data[linkEnd-2] != '\\' {
		linkEnd--
	}

	// But don't skip semicolon if it's a part of escaped entity:
	if data[linkEnd-1] == ';' && data[linkEnd-2] != '\\' && !linkEndsWithEntity(data, linkEnd) {
		linkEnd--
	}

	// See if the link finishes with a punctuation sign that can be closed.
	var copen byte
	switch data[linkEnd-1] {
	case '"':
		copen = '"'
	case '\'':
		copen = '\''
	case ')':
		copen = '('
	case ']':
		copen = '['
	case '}':
		copen = '{'
	default:
		copen = 0
	}

	if copen != 0 {
		bufEnd := offset - rewind + linkEnd - 2

		openDelim := 1

		/* Try to close the final punctuation sign in this same line;
		 * if we managed to close it outside of the URL, that means that it's
		 * not part of the URL. If it closes inside the URL, that means it
		 * is part of the URL.
		 *
		 * Examples:
		 *
		 *      foo http://www.pokemon.com/Pikachu_(Electric) bar
		 *              => http://www.pokemon.com/Pikachu_(Electric)
		 *
		 *      foo (http://www.pokemon.com/Pikachu_(Electric)) bar
		 *              => http://www.pokemon.com/Pikachu_(Electric)
		 *
		 *      foo http://www.pokemon.com/Pikachu_(Electric)) bar
		 *              => http://www.pokemon.com/Pikachu_(Electric))
		 *
		 *      (foo http://www.pokemon.com/Pikachu_(Electric)) bar
		 *              => foo http://www.pokemon.com/Pikachu_(Electric)
		 */

		for bufEnd >= 0 && origData[bufEnd] != '\n' && openDelim != 0 {
			if origData[bufEnd] == data[linkEnd-1] {
				openDelim++
			}

			if origData[bufEnd] == copen {
				openDelim--
			}

			bufEnd--
		}

		if openDelim == 0 {
			linkEnd--
		}
	}

	var uLink bytes.Buffer
	unescapeText(&uLink, data[:linkEnd])

	if uLink.Len() > 0 {
		node := &ast.Link{
			Destination: uLink.Bytes(),
		}
		ast.AppendChild(node, newTextNode(uLink.Bytes()))
		return linkEnd, node
	}

	return linkEnd, nil
}

func isEndOfLink(char byte) bool {
	return isSpace(char) || char == '<'
}

var validUris = [][]byte{[]byte("http://"), []byte("https://"), []byte("ftp://"), []byte("mailto://")}
var validPaths = [][]byte{[]byte("/"), []byte("./"), []byte("../")}

func isSafeLink(link []byte) bool {
	nLink := len(link)
	for _, path := range validPaths {
		nPath := len(path)
		linkPrefix := link[:nPath]
		if nLink >= nPath && bytes.Equal(linkPrefix, path) {
			if nLink == nPath {
				return true
			} else if isAlnum(link[nPath]) {
				return true
			}
		}
	}

	for _, prefix := range validUris {
		// TODO: handle unicode here
		// case-insensitive prefix test
		nPrefix := len(prefix)
		if nLink > nPrefix {
			linkPrefix := bytes.ToLower(link[:nPrefix])
			if bytes.Equal(linkPrefix, prefix) && isAlnum(link[nPrefix]) {
				return true
			}
		}
	}

	return false
}

// return the length of the given tag, or 0 is it's not valid
func tagLength(data []byte) (autolink autolinkType, end int) {
	var i, j int

	// a valid tag can't be shorter than 3 chars
	if len(data) < 3 {
		return notAutolink, 0
	}

	// begins with a '<' optionally followed by '/', followed by letter or number
	if data[0] != '<' {
		return notAutolink, 0
	}
	if data[1] == '/' {
		i = 2
	} else {
		i = 1
	}

	if !isAlnum(data[i]) {
		return notAutolink, 0
	}

	// scheme test
	autolink = notAutolink

	// try to find the beginning of an URI
	for i < len(data) && (isAlnum(data[i]) || data[i] == '.' || data[i] == '+' || data[i] == '-') {
		i++
	}

	if i > 1 && i < len(data) && data[i] == '@' {
		if j = isMailtoAutoLink(data[i:]); j != 0 {
			return emailAutolink, i + j
		}
	}

	if i > 2 && i < len(data) && data[i] == ':' {
		autolink = normalAutolink
		i++
	}

	// complete autolink test: no whitespace or ' or "
	switch {
	case i >= len(data):
		autolink = notAutolink
	case autolink != notAutolink:
		j = i

		for i < len(data) {
			if data[i] == '\\' {
				i += 2
			} else if data[i] == '>' || data[i] == '\'' || data[i] == '"' || isSpace(data[i]) {
				break
			} else {
				i++
			}

		}

		if i >= len(data) {
			return autolink, 0
		}
		if i > j && data[i] == '>' {
			return autolink, i + 1
		}

		// one of the forbidden chars has been found
		autolink = notAutolink
	}
	i += bytes.IndexByte(data[i:], '>')
	if i < 0 {
		return autolink, 0
	}
	return autolink, i + 1
}

// look for the address part of a mail autolink and '>'
// this is less strict than the original markdown e-mail address matching
func isMailtoAutoLink(data []byte) int {
	nb := 0

	// address is assumed to be: [-@._a-zA-Z0-9]+ with exactly one '@'
	for i, c := range data {
		if isAlnum(c) {
			continue
		}

		switch c {
		case '@':
			nb++

		case '-', '.', '_':
			break

		case '>':
			if nb == 1 {
				return i + 1
			}
			return 0
		default:
			return 0
		}
	}

	return 0
}

// look for the next emph char, skipping other constructs
func helperFindEmphChar(data []byte, c byte) int {
	i := 0

	for i < len(data) {
		for i < len(data) && data[i] != c && data[i] != '`' && data[i] != '[' {
			i++
		}
		if i >= len(data) {
			return 0
		}
		// do not count escaped chars
		if i != 0 && data[i-1] == '\\' {
			i++
			continue
		}
		if data[i] == c {
			return i
		}

		if data[i] == '`' {
			// skip a code span
			tmpI := 0
			i++
			for i < len(data) && data[i] != '`' {
				if tmpI == 0 && data[i] == c {
					tmpI = i
				}
				i++
			}
			if i >= len(data) {
				return tmpI
			}
			i++
		} else if data[i] == '[' {
			// skip a link
			tmpI := 0
			i++
			for i < len(data) && data[i] != ']' {
				if tmpI == 0 && data[i] == c {
					tmpI = i
				}
				i++
			}
			i++
			for i < len(data) && (data[i] == ' ' || data[i] == '\n') {
				i++
			}
			if i >= len(data) {
				return tmpI
			}
			if data[i] != '[' && data[i] != '(' { // not a link
				if tmpI > 0 {
					return tmpI
				}
				continue
			}
			cc := data[i]
			i++
			for i < len(data) && data[i] != cc {
				if tmpI == 0 && data[i] == c {
					return i
				}
				i++
			}
			if i >= len(data) {
				return tmpI
			}
			i++
		}
	}
	return 0
}

func helperEmphasis(p *Parser, data []byte, c byte) (int, ast.Node) {
	i := 0

	// skip one symbol if coming from emph3
	if len(data) > 1 && data[0] == c && data[1] == c {
		i = 1
	}

	for i < len(data) {
		length := helperFindEmphChar(data[i:], c)
		if length == 0 {
			return 0, nil
		}
		i += length
		if i >= len(data) {
			return 0, nil
		}

		if i+1 < len(data) && data[i+1] == c {
			i++
			continue
		}

		if data[i] == c && !isSpace(data[i-1]) {

			if p.extensions&NoIntraEmphasis != 0 {
				if !(i+1 == len(data) || isSpace(data[i+1]) || isPunctuation(data[i+1])) {
					continue
				}
			}

			emph := &ast.Emph{}
			p.Inline(emph, data[:i])
			return i + 1, emph
		}
	}

	return 0, nil
}

func helperDoubleEmphasis(p *Parser, data []byte, c byte) (int, ast.Node) {
	i := 0

	for i < len(data) {
		length := helperFindEmphChar(data[i:], c)
		if length == 0 {
			return 0, nil
		}
		i += length

		if i+1 < len(data) && data[i] == c && data[i+1] == c && i > 0 && !isSpace(data[i-1]) {
			var node ast.Node = &ast.Strong{}
			if c == '~' {
				node = &ast.Del{}
			}
			p.Inline(node, data[:i])
			return i + 2, node
		}
		i++
	}
	return 0, nil
}

func helperTripleEmphasis(p *Parser, data []byte, offset int, c byte) (int, ast.Node) {
	i := 0
	origData := data
	data = data[offset:]

	for i < len(data) {
		length := helperFindEmphChar(data[i:], c)
		if length == 0 {
			return 0, nil
		}
		i += length

		// skip whitespace preceded symbols
		if data[i] != c || isSpace(data[i-1]) {
			continue
		}

		switch {
		case i+2 < len(data) && data[i+1] == c && data[i+2] == c:
			// triple symbol found
			strong := &ast.Strong{}
			em := &ast.Emph{}
			ast.AppendChild(strong, em)
			p.Inline(em, data[:i])
			return i + 3, strong
		case i+1 < len(data) && data[i+1] == c:
			// double symbol found, hand over to emph1
			length, node := helperEmphasis(p, origData[offset-2:], c)
			if length == 0 {
				return 0, nil
			}
			return length - 2, node
		default:
			// single symbol found, hand over to emph2
			length, node := helperDoubleEmphasis(p, origData[offset-1:], c)
			if length == 0 {
				return 0, nil
			}
			return length - 1, node
		}
	}
	return 0, nil
}

// math handle inline math wrapped with '$'
func math(p *Parser, data []byte, offset int) (int, ast.Node) {
	data = data[offset:]

	// too short, or block math
	if len(data) <= 2 || data[1] == '$' {
		return 0, nil
	}

	// find next '$'
	var end int
	for end = 1; end < len(data) && data[end] != '$'; end++ {
	}

	// $ not match
	if end == len(data) {
		return 0, nil
	}

	// create inline math node
	math := &ast.Math{}
	math.Literal = data[1:end]
	return end + 1, math
}

func newTextNode(d []byte) *ast.Text {
	return &ast.Text{ast.Leaf{Literal: d}}
}

func normalizeURI(s []byte) []byte {
	return s // TODO: implement
}
