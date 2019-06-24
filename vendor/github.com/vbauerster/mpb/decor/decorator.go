package decor

import (
	"fmt"
	"time"
	"unicode/utf8"
)

const (
	// DidentRight bit specifies identation direction.
	// |foo   |b     | With DidentRight
	// |   foo|     b| Without DidentRight
	DidentRight = 1 << iota

	// DextraSpace bit adds extra space, makes sense with DSyncWidth only.
	// When DidentRight bit set, the space will be added to the right,
	// otherwise to the left.
	DextraSpace

	// DSyncWidth bit enables same column width synchronization.
	// Effective with multiple bars only.
	DSyncWidth

	// DSyncWidthR is shortcut for DSyncWidth|DidentRight
	DSyncWidthR = DSyncWidth | DidentRight

	// DSyncSpace is shortcut for DSyncWidth|DextraSpace
	DSyncSpace = DSyncWidth | DextraSpace

	// DSyncSpaceR is shortcut for DSyncWidth|DextraSpace|DidentRight
	DSyncSpaceR = DSyncWidth | DextraSpace | DidentRight
)

// TimeStyle enum.
type TimeStyle int

// TimeStyle kinds.
const (
	ET_STYLE_GO TimeStyle = iota
	ET_STYLE_HHMMSS
	ET_STYLE_HHMM
	ET_STYLE_MMSS
)

// Statistics is a struct, which gets passed to a Decorator.
type Statistics struct {
	ID        int
	Completed bool
	Total     int64
	Current   int64
}

// Decorator interface.
// A decorator must implement this interface, in order to be used with
// mpb library.
type Decorator interface {
	Decor(*Statistics) string
	Syncable
}

// Syncable interface.
// All decorators implement this interface implicitly. Its Syncable
// method exposes width sync channel, if sync is enabled.
type Syncable interface {
	Syncable() (bool, chan int)
}

// OnCompleteMessenger interface.
// Decorators implementing this interface suppose to return provided
// string on complete event.
type OnCompleteMessenger interface {
	OnCompleteMessage(string)
}

// AmountReceiver interface.
// If decorator needs to receive increment amount, so this is the right
// interface to implement.
type AmountReceiver interface {
	NextAmount(int, ...time.Duration)
}

// ShutdownListener interface.
// If decorator needs to be notified once upon bar shutdown event, so
// this is the right interface to implement.
type ShutdownListener interface {
	Shutdown()
}

// Global convenience shortcuts
var (
	WCSyncWidth  = WC{C: DSyncWidth}
	WCSyncWidthR = WC{C: DSyncWidthR}
	WCSyncSpace  = WC{C: DSyncSpace}
	WCSyncSpaceR = WC{C: DSyncSpaceR}
)

// WC is a struct with two public fields W and C, both of int type.
// W represents width and C represents bit set of width related config.
// A decorator should embed WC, in order to become Syncable.
type WC struct {
	W      int
	C      int
	format string
	wsync  chan int
}

// FormatMsg formats final message according to WC.W and WC.C.
// Should be called by any Decorator implementation.
func (wc WC) FormatMsg(msg string) string {
	if (wc.C & DSyncWidth) != 0 {
		wc.wsync <- utf8.RuneCountInString(msg)
		max := <-wc.wsync
		if max == 0 {
			max = wc.W
		}
		if (wc.C & DextraSpace) != 0 {
			max++
		}
		return fmt.Sprintf(fmt.Sprintf(wc.format, max), msg)
	}
	return fmt.Sprintf(fmt.Sprintf(wc.format, wc.W), msg)
}

// Init initializes width related config.
func (wc *WC) Init() {
	wc.format = "%%"
	if (wc.C & DidentRight) != 0 {
		wc.format += "-"
	}
	wc.format += "%ds"
	if (wc.C & DSyncWidth) != 0 {
		wc.wsync = make(chan int)
	}
}

// Syncable is implementation of Syncable interface.
func (wc *WC) Syncable() (bool, chan int) {
	return (wc.C & DSyncWidth) != 0, wc.wsync
}

// OnComplete returns decorator, which wraps provided decorator, with
// sole purpose to display provided message on complete event.
//
//	`decorator` Decorator to wrap
//
//	`message` message to display on complete event
func OnComplete(decorator Decorator, message string) Decorator {
	if d, ok := decorator.(OnCompleteMessenger); ok {
		d.OnCompleteMessage(message)
	}
	return decorator
}
