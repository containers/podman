package mpb

import (
	"io"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v7/decor"
	"github.com/vbauerster/mpb/v7/internal"
)

const (
	positionLeft = 1 + iota
	positionRight
)

// SpinnerStyleComposer interface.
type SpinnerStyleComposer interface {
	BarFillerBuilder
	PositionLeft() SpinnerStyleComposer
	PositionRight() SpinnerStyleComposer
}

type sFiller struct {
	count    uint
	position uint
	frames   []string
}

type spinnerStyle struct {
	position uint
	frames   []string
}

// SpinnerStyle constructs default spinner style which can be altered via
// SpinnerStyleComposer interface.
func SpinnerStyle(frames ...string) SpinnerStyleComposer {
	ss := new(spinnerStyle)
	if len(frames) != 0 {
		ss.frames = append(ss.frames, frames...)
	} else {
		ss.frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	}
	return ss
}

func (s *spinnerStyle) PositionLeft() SpinnerStyleComposer {
	s.position = positionLeft
	return s
}

func (s *spinnerStyle) PositionRight() SpinnerStyleComposer {
	s.position = positionRight
	return s
}

func (s *spinnerStyle) Build() BarFiller {
	sf := &sFiller{
		position: s.position,
		frames:   s.frames,
	}
	return sf
}

func (s *sFiller) Fill(w io.Writer, width int, stat decor.Statistics) {
	width = internal.CheckRequestedWidth(width, stat.AvailableWidth)

	frame := s.frames[s.count%uint(len(s.frames))]
	frameWidth := runewidth.StringWidth(stripansi.Strip(frame))

	if width < frameWidth {
		return
	}

	rest := width - frameWidth
	switch s.position {
	case positionLeft:
		io.WriteString(w, frame+strings.Repeat(" ", rest))
	case positionRight:
		io.WriteString(w, strings.Repeat(" ", rest)+frame)
	default:
		str := strings.Repeat(" ", rest/2) + frame + strings.Repeat(" ", rest/2+rest%2)
		io.WriteString(w, str)
	}
	s.count++
}
