package mpb

import (
	"io"

	"github.com/vbauerster/mpb/v8/decor"
)

// NopStyle provides BarFillerBuilder which builds NOP BarFiller.
func NopStyle() BarFillerBuilder {
	return BarFillerBuilderFunc(func() BarFiller {
		return BarFillerFunc(func(io.Writer, decor.Statistics) error {
			return nil
		})
	})
}
