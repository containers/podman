package goterm

import (
	"bytes"
	"strings"
)

const DEFAULT_BORDER = "- │ ┌ ┐ └ ┘"

// Box allows you to create independent parts of screen, with its own buffer and borders.
// Can be used for creating modal windows
//
// Generates boxes likes this:
// ┌--------┐
// │hello   │
// │world   │
// │        │
// └--------┘
//
type Box struct {
	Buf *bytes.Buffer

	Width  int
	Height int

	// To get even padding: PaddingX ~= PaddingY*4
	PaddingX int
	PaddingY int

	// Should contain 6 border pieces separated by spaces
	//
	// Example border:
	//   "- │ ┌ ┐ └ ┘"
	Border string

	Flags int // Not used now
}

// Create new Box.
// Width and height can be relative:
//
//    // Create box with 50% with of current screen and 10 lines height
//    box := tm.NewBox(50|tm.PCT, 10, 0)
//
func NewBox(width, height int, flags int) *Box {
	width, height = GetXY(width, height)

	box := new(Box)
	box.Buf = new(bytes.Buffer)
	box.Width = width
	box.Height = height
	box.Border = DEFAULT_BORDER
	box.PaddingX = 1
	box.PaddingY = 0
	box.Flags = flags

	return box
}

func (b *Box) Write(p []byte) (int, error) {
	return b.Buf.Write(p)
}

// Render Box
func (b *Box) String() (out string) {
	borders := strings.Split(b.Border, " ")
	lines := strings.Split(b.Buf.String(), "\n")

	// Border + padding
	prefix := borders[1] + strings.Repeat(" ", b.PaddingX)
	suffix := strings.Repeat(" ", b.PaddingX) + borders[1]

	offset := b.PaddingY + 1 // 1 is border width

	// Content width without borders and padding
	contentWidth := b.Width - (b.PaddingX+1)*2

	for y := 0; y < b.Height; y++ {
		var line string

		switch {
		// Draw borders for first line
		case y == 0:
			line = borders[2] + strings.Repeat(borders[0], b.Width-2) + borders[3]

		// Draw borders for last line
		case y == (b.Height - 1):
			line = borders[4] + strings.Repeat(borders[0], b.Width-2) + borders[5]

		// Draw top and bottom padding
		case y <= b.PaddingY || y >= (b.Height-b.PaddingY):
			line = borders[1] + strings.Repeat(" ", b.Width-2) + borders[1]

		// Render content
		default:
			if len(lines) > y-offset {
				line = lines[y-offset]
			} else {
				line = ""
			}

			if len(line) > contentWidth-1 {
				// If line is too large limit it
				line = line[0:contentWidth]
			} else {
				// If line is too small enlarge it by adding spaces
				line = line + strings.Repeat(" ", contentWidth-len(line))
			}

			line = prefix + line + suffix
		}

		// Don't add newline for last element
		if y != b.Height-1 {
			line = line + "\n"
		}

		out += line
	}

	return out
}
