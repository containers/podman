package mpb

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/vbauerster/mpb/v5/decor"
	"github.com/vbauerster/mpb/v5/internal"
)

// SpinnerAlignment enum.
type SpinnerAlignment int

// SpinnerAlignment kinds.
const (
	SpinnerOnLeft SpinnerAlignment = iota
	SpinnerOnMiddle
	SpinnerOnRight
)

// DefaultSpinnerStyle is a slice of strings, which makes a spinner.
var DefaultSpinnerStyle = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinnerFiller struct {
	frames    []string
	count     uint
	alignment SpinnerAlignment
}

// NewSpinnerFiller constucts mpb.BarFiller, to be used with *Progress.Add(...) *Bar method.
func NewSpinnerFiller(style []string, alignment SpinnerAlignment) BarFiller {
	if len(style) == 0 {
		style = DefaultSpinnerStyle
	}
	filler := &spinnerFiller{
		frames:    style,
		alignment: alignment,
	}
	return filler
}

func (s *spinnerFiller) Fill(w io.Writer, reqWidth int, stat decor.Statistics) {
	width := internal.WidthForBarFiller(reqWidth, stat.AvailableWidth)

	frame := s.frames[s.count%uint(len(s.frames))]
	frameWidth := utf8.RuneCountInString(frame)

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
