package mpb

import (
	"io"

	"github.com/vbauerster/mpb/v5/decor"
)

// BarFiller interface.
// Bar (without decorators) renders itself by calling BarFiller's Fill method.
//
//	`reqWidth` is requested width, which is set via:
//	func WithWidth(width int) ContainerOption
//	func BarWidth(width int) BarOption
//
// Default implementations can be obtained via:
//
//	func NewBarFiller(style string, reverse bool) BarFiller
//	func NewSpinnerFiller(style []string, alignment SpinnerAlignment) BarFiller
//
type BarFiller interface {
	Fill(w io.Writer, reqWidth int, stat decor.Statistics)
}

// BarFillerFunc is function type adapter to convert function into BarFiller.
type BarFillerFunc func(w io.Writer, reqWidth int, stat decor.Statistics)

func (f BarFillerFunc) Fill(w io.Writer, reqWidth int, stat decor.Statistics) {
	f(w, reqWidth, stat)
}
