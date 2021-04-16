package mpb

import (
	"io"

	"github.com/vbauerster/mpb/v6/decor"
)

// BarFiller interface.
// Bar (without decorators) renders itself by calling BarFiller's Fill method.
//
//	reqWidth is requested width, set by `func WithWidth(int) ContainerOption`.
//	If not set, it defaults to terminal width.
//
// Default implementations can be obtained via:
//
//	func NewBarFiller(style string) BarFiller
//	func NewBarFillerRev(style string) BarFiller
//	func NewBarFillerPick(style string, rev bool) BarFiller
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
