package mpb

import (
	"io"

	"github.com/vbauerster/mpb/decor"
)

// BarOption is a function option which changes the default behavior of a bar,
// if passed to p.AddBar(int64, ...BarOption)
type BarOption func(*bState)

// AppendDecorators let you inject decorators to the bar's right side
func AppendDecorators(appenders ...decor.Decorator) BarOption {
	return func(s *bState) {
		for _, decorator := range appenders {
			if ar, ok := decorator.(decor.AmountReceiver); ok {
				s.amountReceivers = append(s.amountReceivers, ar)
			}
			if sl, ok := decorator.(decor.ShutdownListener); ok {
				s.shutdownListeners = append(s.shutdownListeners, sl)
			}
			s.aDecorators = append(s.aDecorators, decorator)
		}
	}
}

// PrependDecorators let you inject decorators to the bar's left side
func PrependDecorators(prependers ...decor.Decorator) BarOption {
	return func(s *bState) {
		for _, decorator := range prependers {
			if ar, ok := decorator.(decor.AmountReceiver); ok {
				s.amountReceivers = append(s.amountReceivers, ar)
			}
			if sl, ok := decorator.(decor.ShutdownListener); ok {
				s.shutdownListeners = append(s.shutdownListeners, sl)
			}
			s.pDecorators = append(s.pDecorators, decorator)
		}
	}
}

// BarTrimLeft trims left side space of the bar
func BarTrimLeft() BarOption {
	return func(s *bState) {
		s.trimLeftSpace = true
	}
}

// BarTrimRight trims right space of the bar
func BarTrimRight() BarOption {
	return func(s *bState) {
		s.trimRightSpace = true
	}
}

// BarTrim trims both left and right spaces of the bar
func BarTrim() BarOption {
	return func(s *bState) {
		s.trimLeftSpace = true
		s.trimRightSpace = true
	}
}

// BarID overwrites internal bar id
func BarID(id int) BarOption {
	return func(s *bState) {
		s.id = id
	}
}

// BarRemoveOnComplete is a flag, if set whole bar line will be removed on complete event.
// If both BarRemoveOnComplete and BarClearOnComplete are set, first bar section gets cleared
// and then whole bar line gets removed completely.
func BarRemoveOnComplete() BarOption {
	return func(s *bState) {
		s.removeOnComplete = true
	}
}

// BarReplaceOnComplete is indicator for delayed bar start, after the `runningBar` is complete.
// To achieve bar replacement effect, `runningBar` should has its `BarRemoveOnComplete` option set.
func BarReplaceOnComplete(runningBar *Bar) BarOption {
	return func(s *bState) {
		s.runningBar = runningBar
	}
}

// BarClearOnComplete is a flag, if set will clear bar section on complete event.
// If you need to remove a whole bar line, refer to BarRemoveOnComplete.
func BarClearOnComplete() BarOption {
	return func(s *bState) {
		s.barClearOnComplete = true
	}
}

// BarPriority sets bar's priority.
// Zero is highest priority, i.e. bar will be on top.
// If `BarReplaceOnComplete` option is supplied, this option is ignored.
func BarPriority(priority int) BarOption {
	return func(s *bState) {
		s.priority = priority
	}
}

// BarNewLineExtend takes user defined efn, which gets called each render cycle.
// Any write to provided writer of efn, will appear on new line of respective bar.
func BarNewLineExtend(efn func(io.Writer, *decor.Statistics)) BarOption {
	return func(s *bState) {
		s.newLineExtendFn = efn
	}
}

func barWidth(w int) BarOption {
	return func(s *bState) {
		s.width = w
	}
}

func barFormat(format string) BarOption {
	return func(s *bState) {
		s.runes = strToBarRunes(format)
	}
}
