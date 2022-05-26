package parser

import (
	"bytes"
	"path"
	"path/filepath"
)

// isInclude parses {{...}}[...], that contains a path between the {{, the [...] syntax contains
// an address to select which lines to include. It is treated as an opaque string and just given
// to readInclude.
func (p *Parser) isInclude(data []byte) (filename string, address []byte, consumed int) {
	i := skipCharN(data, 0, ' ', 3) // start with up to 3 spaces
	if len(data[i:]) < 3 {
		return "", nil, 0
	}
	if data[i] != '{' || data[i+1] != '{' {
		return "", nil, 0
	}
	start := i + 2

	// find the end delimiter
	i = skipUntilChar(data, i, '}')
	if i+1 >= len(data) {
		return "", nil, 0
	}
	end := i
	i++
	if data[i] != '}' {
		return "", nil, 0
	}
	filename = string(data[start:end])

	if i+1 < len(data) && data[i+1] == '[' { // potential address specification
		start := i + 2

		end = skipUntilChar(data, start, ']')
		if end >= len(data) {
			return "", nil, 0
		}
		address = data[start:end]
		return filename, address, end + 1
	}

	return filename, address, i + 1
}

func (p *Parser) readInclude(from, file string, address []byte) []byte {
	if p.Opts.ReadIncludeFn != nil {
		return p.Opts.ReadIncludeFn(from, file, address)
	}

	return nil
}

// isCodeInclude parses <{{...}} which is similar to isInclude the returned bytes are, however wrapped in a code block.
func (p *Parser) isCodeInclude(data []byte) (filename string, address []byte, consumed int) {
	i := skipCharN(data, 0, ' ', 3) // start with up to 3 spaces
	if len(data[i:]) < 3 {
		return "", nil, 0
	}
	if data[i] != '<' {
		return "", nil, 0
	}
	start := i

	filename, address, consumed = p.isInclude(data[i+1:])
	if consumed == 0 {
		return "", nil, 0
	}
	return filename, address, start + consumed + 1
}

// readCodeInclude acts like include except the returned bytes are wrapped in a fenced code block.
func (p *Parser) readCodeInclude(from, file string, address []byte) []byte {
	data := p.readInclude(from, file, address)
	if data == nil {
		return nil
	}
	ext := path.Ext(file)
	buf := &bytes.Buffer{}
	buf.Write([]byte("```"))
	if ext != "" { // starts with a dot
		buf.WriteString(" " + ext[1:] + "\n")
	} else {
		buf.WriteByte('\n')
	}
	buf.Write(data)
	buf.WriteString("```\n")
	return buf.Bytes()
}

// incStack hold the current stack of chained includes. Each value is the containing
// path of the file being parsed.
type incStack struct {
	stack []string
}

func newIncStack() *incStack {
	return &incStack{stack: []string{}}
}

// Push updates i with new.
func (i *incStack) Push(new string) {
	if path.IsAbs(new) {
		i.stack = append(i.stack, path.Dir(new))
		return
	}
	last := ""
	if len(i.stack) > 0 {
		last = i.stack[len(i.stack)-1]
	}
	i.stack = append(i.stack, path.Dir(filepath.Join(last, new)))
}

// Pop pops the last value.
func (i *incStack) Pop() {
	if len(i.stack) == 0 {
		return
	}
	i.stack = i.stack[:len(i.stack)-1]
}

func (i *incStack) Last() string {
	if len(i.stack) == 0 {
		return ""
	}
	return i.stack[len(i.stack)-1]
}
