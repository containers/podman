package mpb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v6/decor"
)

// Bar represents a progress bar.
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

type extenderFunc func(in io.Reader, reqWidth int, st decor.Statistics) (out io.Reader, lines int)

// bState is actual bar state. It gets passed to *Bar.serve(...) monitor
// goroutine.
type bState struct {
	id                int
	priority          int
	reqWidth          int
	total             int64
	current           int64
	refill            int64
	lastN             int64
	iterated          bool
	trimSpace         bool
	completed         bool
	completeFlushed   bool
	triggerComplete   bool
	dropOnComplete    bool
	noPop             bool
	aDecorators       []decor.Decorator
	pDecorators       []decor.Decorator
	averageDecorators []decor.AverageDecorator
	ewmaDecorators    []decor.EwmaDecorator
	shutdownListeners []decor.ShutdownListener
	bufP, bufB, bufA  *bytes.Buffer
	filler            BarFiller
	middleware        func(BarFiller) BarFiller
	extender          extenderFunc

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
		syncTableCh:  make(chan [][]chan int, 1),
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

// SetRefill sets refill flag with specified amount.
// The underlying BarFiller will change its visual representation, to
// indicate refill event. Refill event may be referred to some retry
// operation for example.
func (b *Bar) SetRefill(amount int64) {
	select {
	case b.operateState <- func(s *bState) {
		s.refill = amount
	}:
	case <-b.done:
	}
}

// TraverseDecorators traverses all available decorators and calls cb func on each.
func (b *Bar) TraverseDecorators(cb func(decor.Decorator)) {
	select {
	case b.operateState <- func(s *bState) {
		for _, decorators := range [...][]decor.Decorator{
			s.pDecorators,
			s.aDecorators,
		} {
			for _, d := range decorators {
				cb(extractBaseDecorator(d))
			}
		}
	}:
	case <-b.done:
	}
}

// SetTotal sets total dynamically.
// If total is less than or equal to zero it takes progress' current value.
func (b *Bar) SetTotal(total int64, triggerComplete bool) {
	select {
	case b.operateState <- func(s *bState) {
		s.triggerComplete = triggerComplete
		if total <= 0 {
			s.total = s.current
		} else {
			s.total = total
		}
		if s.triggerComplete && !s.completed {
			s.current = s.total
			s.completed = true
			go b.refreshTillShutdown()
		}
	}:
	case <-b.done:
	}
}

// SetCurrent sets progress' current to an arbitrary value.
// Setting a negative value will cause a panic.
func (b *Bar) SetCurrent(current int64) {
	select {
	case b.operateState <- func(s *bState) {
		s.iterated = true
		s.lastN = current - s.current
		s.current = current
		if s.triggerComplete && s.current >= s.total {
			s.current = s.total
			s.completed = true
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
		if s.triggerComplete && s.current >= s.total {
			s.current = s.total
			s.completed = true
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
	case b.operateState <- func(s *bState) { b.completed <- s.completed }:
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
	select {
	case b.operateState <- func(s *bState) {
		stat := newStatistics(tw, s)
		defer func() {
			// recovering if user defined decorator panics for example
			if p := recover(); p != nil {
				if b.recoveredPanic == nil {
					s.extender = makePanicExtender(p)
					b.toShutdown = !b.toShutdown
					b.recoveredPanic = p
				}
				frame, lines := s.extender(nil, s.reqWidth, stat)
				b.extendedLines = lines
				b.frameCh <- frame
				b.dlogger.Println(p)
			}
			s.completeFlushed = s.completed
		}()
		frame, lines := s.extender(s.draw(stat), s.reqWidth, stat)
		b.extendedLines = lines
		b.toShutdown = s.completed && !s.completeFlushed
		b.frameCh <- frame
	}:
	case <-b.done:
		s := b.cacheState
		stat := newStatistics(tw, s)
		var r io.Reader
		if b.recoveredPanic == nil {
			r = s.draw(stat)
		}
		frame, lines := s.extender(r, s.reqWidth, stat)
		b.extendedLines = lines
		b.frameCh <- frame
	}
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
	select {
	case b.operateState <- func(s *bState) {
		s.averageDecorators = averageDecorators
		s.ewmaDecorators = ewmaDecorators
		s.shutdownListeners = shutdownListeners
	}:
		b.hasEwmaDecorators = len(ewmaDecorators) != 0
	case <-b.done:
	}
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

func (s *bState) draw(stat decor.Statistics) io.Reader {
	if !s.trimSpace {
		stat.AvailableWidth -= 2
		s.bufB.WriteByte(' ')
		defer s.bufB.WriteByte(' ')
	}

	nlr := strings.NewReader("\n")
	tw := stat.AvailableWidth
	for _, d := range s.pDecorators {
		str := d.Decor(stat)
		stat.AvailableWidth -= runewidth.StringWidth(stripansi.Strip(str))
		s.bufP.WriteString(str)
	}
	if stat.AvailableWidth <= 0 {
		trunc := strings.NewReader(runewidth.Truncate(stripansi.Strip(s.bufP.String()), tw, "…"))
		s.bufP.Reset()
		return io.MultiReader(trunc, s.bufB, nlr)
	}

	tw = stat.AvailableWidth
	for _, d := range s.aDecorators {
		str := d.Decor(stat)
		stat.AvailableWidth -= runewidth.StringWidth(stripansi.Strip(str))
		s.bufA.WriteString(str)
	}
	if stat.AvailableWidth <= 0 {
		trunc := strings.NewReader(runewidth.Truncate(stripansi.Strip(s.bufA.String()), tw, "…"))
		s.bufA.Reset()
		return io.MultiReader(s.bufP, s.bufB, trunc, nlr)
	}

	s.filler.Fill(s.bufB, s.reqWidth, stat)

	return io.MultiReader(s.bufP, s.bufB, s.bufA, nlr)
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

func newStatistics(tw int, s *bState) decor.Statistics {
	return decor.Statistics{
		ID:             s.id,
		AvailableWidth: tw,
		Total:          s.total,
		Current:        s.current,
		Refill:         s.refill,
		Completed:      s.completeFlushed,
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

func makePanicExtender(p interface{}) extenderFunc {
	pstr := fmt.Sprint(p)
	stack := debug.Stack()
	stackLines := bytes.Count(stack, []byte("\n"))
	return func(_ io.Reader, _ int, st decor.Statistics) (io.Reader, int) {
		mr := io.MultiReader(
			strings.NewReader(runewidth.Truncate(pstr, st.AvailableWidth, "…")),
			strings.NewReader(fmt.Sprintf("\n%#v\n", st)),
			bytes.NewReader(stack),
		)
		return mr, stackLines + 1
	}
}
