package mpb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vbauerster/mpb/v5/decor"
)

// BarFiller interface.
// Bar renders itself by calling BarFiller's Fill method. You can
// literally have any bar kind, by implementing this interface and
// passing it to the *Progress.Add(...) *Bar method.
type BarFiller interface {
	Fill(w io.Writer, width int, stat *decor.Statistics)
}

// BarFillerFunc is function type adapter to convert function into Filler.
type BarFillerFunc func(w io.Writer, width int, stat *decor.Statistics)

func (f BarFillerFunc) Fill(w io.Writer, width int, stat *decor.Statistics) {
	f(w, width, stat)
}

// Bar represents a progress Bar.
type Bar struct {
	priority int // used by heap
	index    int // used by heap

	extendedLines     int
	toShutdown        bool
	toDrop            bool
	noPop             bool
	hasEwmaDecorators bool
	operateState      chan func(*bState)
	frameCh           chan io.Reader
	syncTableCh       chan [][]chan int
	completed         chan bool

	// cancel is called either by user or on complete event
	cancel func()
	// done is closed after cacheState is assigned
	done chan struct{}
	// cacheState is populated, right after close(shutdown)
	cacheState *bState

	container      *Progress
	dlogger        *log.Logger
	recoveredPanic interface{}
}

type extFunc func(in io.Reader, tw int, st *decor.Statistics) (out io.Reader, lines int)

type bState struct {
	baseF             BarFiller
	filler            BarFiller
	id                int
	width             int
	total             int64
	current           int64
	lastN             int64
	iterated          bool
	trimSpace         bool
	toComplete        bool
	completeFlushed   bool
	ignoreComplete    bool
	noPop             bool
	aDecorators       []decor.Decorator
	pDecorators       []decor.Decorator
	averageDecorators []decor.AverageDecorator
	ewmaDecorators    []decor.EwmaDecorator
	shutdownListeners []decor.ShutdownListener
	bufP, bufB, bufA  *bytes.Buffer
	extender          extFunc

	// priority overrides *Bar's priority, if set
	priority int
	// dropOnComplete propagates to *Bar
	dropOnComplete bool
	// runningBar is a key for *pState.parkedBars
	runningBar *Bar

	debugOut io.Writer
}

func newBar(container *Progress, bs *bState) *Bar {
	logPrefix := fmt.Sprintf("%sbar#%02d ", container.dlogger.Prefix(), bs.id)
	ctx, cancel := context.WithCancel(container.ctx)

	bar := &Bar{
		container:    container,
		priority:     bs.priority,
		toDrop:       bs.dropOnComplete,
		noPop:        bs.noPop,
		operateState: make(chan func(*bState)),
		frameCh:      make(chan io.Reader, 1),
		syncTableCh:  make(chan [][]chan int),
		completed:    make(chan bool, 1),
		done:         make(chan struct{}),
		cancel:       cancel,
		dlogger:      log.New(bs.debugOut, logPrefix, log.Lshortfile),
	}

	go bar.serve(ctx, bs)
	return bar
}

// ProxyReader wraps r with metrics required for progress tracking.
// Panics if r is nil.
func (b *Bar) ProxyReader(r io.Reader) io.ReadCloser {
	if r == nil {
		panic("expected non nil io.Reader")
	}
	return newProxyReader(r, b)
}

// ID returs id of the bar.
func (b *Bar) ID() int {
	result := make(chan int)
	select {
	case b.operateState <- func(s *bState) { result <- s.id }:
		return <-result
	case <-b.done:
		return b.cacheState.id
	}
}

// Current returns bar's current number, in other words sum of all increments.
func (b *Bar) Current() int64 {
	result := make(chan int64)
	select {
	case b.operateState <- func(s *bState) { result <- s.current }:
		return <-result
	case <-b.done:
		return b.cacheState.current
	}
}

// SetRefill fills bar with refill rune up to amount argument.
// Given default bar style is "[=>-]<+", refill rune is '+'.
// To set bar style use mpb.BarStyle(string) BarOption.
func (b *Bar) SetRefill(amount int64) {
	type refiller interface {
		SetRefill(int64)
	}
	b.operateState <- func(s *bState) {
		if f, ok := s.baseF.(refiller); ok {
			f.SetRefill(amount)
		}
	}
}

// TraverseDecorators traverses all available decorators and calls cb func on each.
func (b *Bar) TraverseDecorators(cb func(decor.Decorator)) {
	b.operateState <- func(s *bState) {
		for _, decorators := range [...][]decor.Decorator{
			s.pDecorators,
			s.aDecorators,
		} {
			for _, d := range decorators {
				cb(extractBaseDecorator(d))
			}
		}
	}
}

// SetTotal sets total dynamically.
// If total is less than or equal to zero it takes progress' current value.
// A complete flag enables or disables complete event on `current >= total`.
func (b *Bar) SetTotal(total int64, complete bool) {
	select {
	case b.operateState <- func(s *bState) {
		s.ignoreComplete = !complete
		if total <= 0 {
			s.total = s.current
		} else {
			s.total = total
		}
		if !s.ignoreComplete && !s.toComplete {
			s.current = s.total
			s.toComplete = true
			go b.refreshTillShutdown()
		}
	}:
	case <-b.done:
	}
}

// SetCurrent sets progress' current to an arbitrary value.
func (b *Bar) SetCurrent(current int64) {
	select {
	case b.operateState <- func(s *bState) {
		s.iterated = true
		s.lastN = current - s.current
		s.current = current
		if !s.ignoreComplete && s.current >= s.total {
			s.current = s.total
			s.toComplete = true
			go b.refreshTillShutdown()
		}
	}:
	case <-b.done:
	}
}

// Increment is a shorthand for b.IncrInt64(1).
func (b *Bar) Increment() {
	b.IncrInt64(1)
}

// IncrBy is a shorthand for b.IncrInt64(int64(n)).
func (b *Bar) IncrBy(n int) {
	b.IncrInt64(int64(n))
}

// IncrInt64 increments progress by amount of n.
func (b *Bar) IncrInt64(n int64) {
	select {
	case b.operateState <- func(s *bState) {
		s.iterated = true
		s.lastN = n
		s.current += n
		if !s.ignoreComplete && s.current >= s.total {
			s.current = s.total
			s.toComplete = true
			go b.refreshTillShutdown()
		}
	}:
	case <-b.done:
	}
}

// DecoratorEwmaUpdate updates all EWMA based decorators. Should be
// called on each iteration, because EWMA's unit of measure is an
// iteration's duration. Panics if called before *Bar.Incr... family
// methods.
func (b *Bar) DecoratorEwmaUpdate(dur time.Duration) {
	select {
	case b.operateState <- func(s *bState) {
		ewmaIterationUpdate(false, s, dur)
	}:
	case <-b.done:
		ewmaIterationUpdate(true, b.cacheState, dur)
	}
}

// DecoratorAverageAdjust adjusts all average based decorators. Call
// if you need to adjust start time of all average based decorators
// or after progress resume.
func (b *Bar) DecoratorAverageAdjust(start time.Time) {
	select {
	case b.operateState <- func(s *bState) {
		for _, d := range s.averageDecorators {
			d.AverageAdjust(start)
		}
	}:
	case <-b.done:
	}
}

// SetPriority changes bar's order among multiple bars. Zero is highest
// priority, i.e. bar will be on top. If you don't need to set priority
// dynamically, better use BarPriority option.
func (b *Bar) SetPriority(priority int) {
	select {
	case <-b.done:
	default:
		b.container.setBarPriority(b, priority)
	}
}

// Abort interrupts bar's running goroutine. Call this, if you'd like
// to stop/remove bar before completion event. It has no effect after
// completion event. If drop is true bar will be removed as well.
func (b *Bar) Abort(drop bool) {
	select {
	case <-b.done:
	default:
		if drop {
			b.container.dropBar(b)
		}
		b.cancel()
	}
}

// Completed reports whether the bar is in completed state.
func (b *Bar) Completed() bool {
	select {
	case b.operateState <- func(s *bState) { b.completed <- s.toComplete }:
		return <-b.completed
	case <-b.done:
		return true
	}
}

func (b *Bar) serve(ctx context.Context, s *bState) {
	defer b.container.bwg.Done()
	for {
		select {
		case op := <-b.operateState:
			op(s)
		case <-ctx.Done():
			b.cacheState = s
			close(b.done)
			// Notifying decorators about shutdown event
			for _, sl := range s.shutdownListeners {
				sl.Shutdown()
			}
			return
		}
	}
}

func (b *Bar) render(tw int) {
	if b.recoveredPanic != nil {
		b.toShutdown = false
		b.frameCh <- b.panicToFrame(tw)
		return
	}
	select {
	case b.operateState <- func(s *bState) {
		defer func() {
			// recovering if user defined decorator panics for example
			if p := recover(); p != nil {
				b.dlogger.Println(p)
				b.recoveredPanic = p
				b.toShutdown = !s.completeFlushed
				b.frameCh <- b.panicToFrame(tw)
			}
		}()

		st := newStatistics(s)
		frame := s.draw(tw, st)
		frame, b.extendedLines = s.extender(frame, tw, st)

		b.toShutdown = s.toComplete && !s.completeFlushed
		s.completeFlushed = s.toComplete
		b.frameCh <- frame
	}:
	case <-b.done:
		s := b.cacheState
		st := newStatistics(s)
		frame := s.draw(tw, st)
		frame, b.extendedLines = s.extender(frame, tw, st)
		b.frameCh <- frame
	}
}

func (b *Bar) panicToFrame(termWidth int) io.Reader {
	return strings.NewReader(fmt.Sprintf(fmt.Sprintf("%%.%dv\n", termWidth), b.recoveredPanic))
}

func (b *Bar) subscribeDecorators() {
	var averageDecorators []decor.AverageDecorator
	var ewmaDecorators []decor.EwmaDecorator
	var shutdownListeners []decor.ShutdownListener
	b.TraverseDecorators(func(d decor.Decorator) {
		if d, ok := d.(decor.AverageDecorator); ok {
			averageDecorators = append(averageDecorators, d)
		}
		if d, ok := d.(decor.EwmaDecorator); ok {
			ewmaDecorators = append(ewmaDecorators, d)
		}
		if d, ok := d.(decor.ShutdownListener); ok {
			shutdownListeners = append(shutdownListeners, d)
		}
	})
	b.operateState <- func(s *bState) {
		s.averageDecorators = averageDecorators
		s.ewmaDecorators = ewmaDecorators
		s.shutdownListeners = shutdownListeners
	}
	b.hasEwmaDecorators = len(ewmaDecorators) != 0
}

func (b *Bar) refreshTillShutdown() {
	for {
		select {
		case b.container.refreshCh <- time.Now():
		case <-b.done:
			return
		}
	}
}

func (b *Bar) wSyncTable() [][]chan int {
	select {
	case b.operateState <- func(s *bState) { b.syncTableCh <- s.wSyncTable() }:
		return <-b.syncTableCh
	case <-b.done:
		return b.cacheState.wSyncTable()
	}
}

func (s *bState) draw(termWidth int, stat *decor.Statistics) io.Reader {
	for _, d := range s.pDecorators {
		s.bufP.WriteString(d.Decor(stat))
	}

	for _, d := range s.aDecorators {
		s.bufA.WriteString(d.Decor(stat))
	}

	s.bufA.WriteByte('\n')

	prependCount := utf8.RuneCount(s.bufP.Bytes())
	appendCount := utf8.RuneCount(s.bufA.Bytes()) - 1

	if fitWidth := s.width; termWidth > 1 {
		if !s.trimSpace {
			// reserve space for edge spaces
			termWidth -= 2
			s.bufB.WriteByte(' ')
			defer s.bufB.WriteByte(' ')
		}
		if prependCount+s.width+appendCount > termWidth {
			fitWidth = termWidth - prependCount - appendCount
		}
		s.filler.Fill(s.bufB, fitWidth, stat)
	}

	return io.MultiReader(s.bufP, s.bufB, s.bufA)
}

func (s *bState) wSyncTable() [][]chan int {
	columns := make([]chan int, 0, len(s.pDecorators)+len(s.aDecorators))
	var pCount int
	for _, d := range s.pDecorators {
		if ch, ok := d.Sync(); ok {
			columns = append(columns, ch)
			pCount++
		}
	}
	var aCount int
	for _, d := range s.aDecorators {
		if ch, ok := d.Sync(); ok {
			columns = append(columns, ch)
			aCount++
		}
	}
	table := make([][]chan int, 2)
	table[0] = columns[0:pCount]
	table[1] = columns[pCount : pCount+aCount : pCount+aCount]
	return table
}

func newStatistics(s *bState) *decor.Statistics {
	return &decor.Statistics{
		ID:        s.id,
		Completed: s.completeFlushed,
		Total:     s.total,
		Current:   s.current,
	}
}

func extractBaseDecorator(d decor.Decorator) decor.Decorator {
	if d, ok := d.(decor.Wrapper); ok {
		return extractBaseDecorator(d.Base())
	}
	return d
}

func ewmaIterationUpdate(done bool, s *bState, dur time.Duration) {
	if !done && !s.iterated {
		panic("increment required before ewma iteration update")
	} else {
		s.iterated = false
	}
	for _, d := range s.ewmaDecorators {
		d.EwmaUpdate(s.lastN, dur)
	}
}
