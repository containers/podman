package decor

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/acarl005/stripansi"
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

// Statistics consists of progress related statistics, that Decorator
// may need.
type Statistics struct {
	ID        int
	Completed bool
	Total     int64
	Current   int64
}

// Decorator interface.
// Implementors should embed WC type, that way only single method
// Decor(*Statistics) needs to be implemented, the rest will be handled
// by WC type.
type Decorator interface {
	Configurator
	Synchronizer
	Decor(*Statistics) string
}

// Synchronizer interface.
// All decorators implement this interface implicitly. Its Sync
// method exposes width sync channel, if DSyncWidth bit is set.
type Synchronizer interface {
	Sync() (chan int, bool)
}

// Configurator interface.
type Configurator interface {
	GetConf() WC
	SetConf(WC)
}

// Wrapper interface.
// If you're implementing custom Decorator by wrapping a built-in one,
// it is necessary to implement this interface to retain functionality
// of built-in Decorator.
type Wrapper interface {
	Base() Decorator
}

// AmountReceiver interface.
// EWMA based decorators need to implement this one.
type AmountReceiver interface {
	NextAmount(int64, ...time.Duration)
}

// ShutdownListener interface.
// If decorator needs to be notified once upon bar shutdown event, so
// this is the right interface to implement.
type ShutdownListener interface {
	Shutdown()
}

// AverageAdjuster interface.
// Average decorators should implement this interface to provide start
// time adjustment facility, for resume-able tasks.
type AverageAdjuster interface {
	AverageAdjust(time.Time)
}

// CBFunc convenience call back func type.
type CBFunc func(Decorator)

// Global convenience instances of WC with sync width bit set.
var (
	WCSyncWidth  = WC{C: DSyncWidth}
	WCSyncWidthR = WC{C: DSyncWidthR}
	WCSyncSpace  = WC{C: DSyncSpace}
	WCSyncSpaceR = WC{C: DSyncSpaceR}
)

// WC is a struct with two public fields W and C, both of int type.
// W represents width and C represents bit set of width related config.
// A decorator should embed WC, to enable width synchronization.
type WC struct {
	W         int
	C         int
	dynFormat string
	wsync     chan int
}

// FormatMsg formats final message according to WC.W and WC.C.
// Should be called by any Decorator implementation.
func (wc *WC) FormatMsg(msg string) string {
	var format string
	runeCount := utf8.RuneCountInString(stripansi.Strip(msg))
	ansiCount := utf8.RuneCountInString(msg) - runeCount
	if (wc.C & DSyncWidth) != 0 {
		if (wc.C & DextraSpace) != 0 {
			runeCount++
		}
		wc.wsync <- runeCount
		max := <-wc.wsync
		format = fmt.Sprintf(wc.dynFormat, ansiCount+max)
	} else {
		format = fmt.Sprintf(wc.dynFormat, ansiCount+wc.W)
	}
	return fmt.Sprintf(format, msg)
}

// Init initializes width related config.
func (wc *WC) Init() WC {
	wc.dynFormat = "%%"
	if (wc.C & DidentRight) != 0 {
		wc.dynFormat += "-"
	}
	wc.dynFormat += "%ds"
	if (wc.C & DSyncWidth) != 0 {
		// it's deliberate choice to override wsync on each Init() call,
		// this way globals like WCSyncSpace can be reused
		wc.wsync = make(chan int)
	}
	return *wc
}

// Sync is implementation of Synchronizer interface.
func (wc *WC) Sync() (chan int, bool) {
	if (wc.C&DSyncWidth) != 0 && wc.wsync == nil {
		panic(fmt.Sprintf("%T is not initialized", wc))
	}
	return wc.wsync, (wc.C & DSyncWidth) != 0
}

// GetConf is implementation of Configurator interface.
func (wc *WC) GetConf() WC {
	return *wc
}

// SetConf is implementation of Configurator interface.
func (wc *WC) SetConf(conf WC) {
	*wc = conf.Init()
}

func initWC(wcc ...WC) WC {
	var wc WC
	for _, nwc := range wcc {
		wc = nwc
	}
	return wc.Init()
}
