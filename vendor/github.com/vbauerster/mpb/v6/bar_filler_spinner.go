package mpb

import (
	"io"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v6/decor"
	"github.com/vbauerster/mpb/v6/internal"
)

// SpinnerAlignment enum.
type SpinnerAlignment int

// SpinnerAlignment kinds.
const (
	SpinnerOnLeft SpinnerAlignment = iota
	SpinnerOnMiddle
	SpinnerOnRight
)

// SpinnerDefaultStyle is a style for rendering a spinner.
var SpinnerDefaultStyle = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinnerFiller struct {
	frames    []string
	count     uint
	alignment SpinnerAlignment
}

// NewSpinnerFiller returns a BarFiller implementation which renders
// a spinner. If style is nil or zero length, SpinnerDefaultStyle is
// applied. To be used with `*Progress.Add(...) *Bar` method.
func NewSpinnerFiller(style []string, alignment SpinnerAlignment) BarFiller {
	if len(style) == 0 {
		style = SpinnerDefaultStyle
	}
	filler := &spinnerFiller{
		frames:    style,
		alignment: alignment,
	}
	return filler
}

func (s *spinnerFiller) Fill(w io.Writer, reqWidth int, stat decor.Statistics) {
	width := internal.CheckRequestedWidth(reqWidth, stat.AvailableWidth)

	frame := s.frames[s.count%uint(len(s.frames))]
	frameWidth := runewidth.StringWidth(frame)

	if width < frameWidth {
		return
	}

	switch rest := width - frameWidth; s.alignment {
	case SpinnerOnLeft:
		io.WriteString(w, frame+strings.Repeat(" ", rest))
	case SpinnerOnMiddle:
		str := strings.Repeat(" ", rest/2) + frame + strings.Repeat(" ", rest/2+rest%2)
		io.WriteString(w, str)
	case SpinnerOnRight:
		io.WriteString(w, strings.Repeat(" ", rest)+frame)
	}
	s.count++
}
