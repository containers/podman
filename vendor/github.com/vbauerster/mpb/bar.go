package mpb

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/vbauerster/mpb/decor"
)

// Bar represents a progress Bar
type Bar struct {
	priority int
	index    int

	runningBar    *Bar
	cacheState    *bState
	operateState  chan func(*bState)
	int64Ch       chan int64
	boolCh        chan bool
	frameReaderCh chan *frameReader
	syncTableCh   chan [][]chan int

	// done is closed by Bar's goroutine, after cacheState is written
	done chan struct{}
	// shutdown is closed from master Progress goroutine only
	shutdown chan struct{}
}

// Filler interface.
// Bar renders by calling Filler's Fill method. You can literally have
// any bar kind, by implementing this interface and passing it to the
// Add method.
type Filler interface {
	Fill(w io.Writer, width int, s *decor.Statistics)
}

// FillerFunc is function type adapter to convert function into Filler.
type FillerFunc func(w io.Writer, width int, stat *decor.Statistics)

func (f FillerFunc) Fill(w io.Writer, width int, stat *decor.Statistics) {
	f(w, width, stat)
}

type (
	bState struct {
		filler             Filler
		id                 int
		width              int
		alignment          int
		total              int64
		current            int64
		trimSpace          bool
		toComplete         bool
		removeOnComplete   bool
		barClearOnComplete bool
		completeFlushed    bool
		aDecorators        []decor.Decorator
		pDecorators        []decor.Decorator
		amountReceivers    []decor.AmountReceiver
		shutdownListeners  []decor.ShutdownListener
		refill             *refill
		bufP, bufB, bufA   *bytes.Buffer
		bufNL              *bytes.Buffer
		panicMsg           string
		newLineExtendFn    func(io.Writer, *decor.Statistics)

		// following options are assigned to the *Bar
		priority   int
		runningBar *Bar
	}
	refill struct {
		r     rune
		limit int64
	}
	frameReader struct {
		io.Reader
		extendedLines    int
		toShutdown       bool
		removeOnComplete bool
	}
)

func newBar(
	ctx context.Context,
	wg *sync.WaitGroup,
	filler Filler,
	id, width int,
	total int64,
	options ...BarOption,
) *Bar {

	s := &bState{
		filler:   filler,
		id:       id,
		priority: id,
		width:    width,
		total:    total,
	}

	for _, opt := range options {
		if opt != nil {
			opt(s)
		}
	}

	s.bufP = bytes.NewBuffer(make([]byte, 0, s.width))
	s.bufB = bytes.NewBuffer(make([]byte, 0, s.width))
	s.bufA = bytes.NewBuffer(make([]byte, 0, s.width))
	if s.newLineExtendFn != nil {
		s.bufNL = bytes.NewBuffer(make([]byte, 0, s.width))
	}

	b := &Bar{
		priority:      s.priority,
		runningBar:    s.runningBar,
		operateState:  make(chan func(*bState)),
		int64Ch:       make(chan int64),
		boolCh:        make(chan bool),
		frameReaderCh: make(chan *frameReader, 1),
		syncTableCh:   make(chan [][]chan int),
		done:          make(chan struct{}),
		shutdown:      make(chan struct{}),
	}

	if b.runningBar != nil {
		b.priority = b.runningBar.priority
	}

	go b.serve(ctx, wg, s)
	return b
}

// RemoveAllPrependers removes all prepend functions.
func (b *Bar) RemoveAllPrependers() {
	select {
	case b.operateState <- func(s *bState) { s.pDecorators = nil }:
	case <-b.done:
	}
}

// RemoveAllAppenders removes all append functions.
func (b *Bar) RemoveAllAppenders() {
	select {
	case b.operateState <- func(s *bState) { s.aDecorators = nil }:
	case <-b.done:
	}
}

// ProxyReader wraps r with metrics required for progress tracking.
func (b *Bar) ProxyReader(r io.Reader) io.ReadCloser {
	if r == nil {
		panic("expect io.Reader, got nil")
	}
	rc, ok := r.(io.ReadCloser)
	if !ok {
		rc = ioutil.NopCloser(r)
	}
	return &proxyReader{rc, b, time.Now()}
}

// ID returs id of the bar.
func (b *Bar) ID() int {
	select {
	case b.operateState <- func(s *bState) { b.int64Ch <- int64(s.id) }:
		return int(<-b.int64Ch)
	case <-b.done:
		return b.cacheState.id
	}
}

// Current returns bar's current number, in other words sum of all increments.
func (b *Bar) Current() int64 {
	select {
	case b.operateState <- func(s *bState) { b.int64Ch <- s.current }:
		return <-b.int64Ch
	case <-b.done:
		return b.cacheState.current
	}
}

// SetTotal sets total dynamically.
// Set complete to true, to trigger bar complete event now.
func (b *Bar) SetTotal(total int64, complete bool) {
	select {
	case b.operateState <- func(s *bState) {
		s.total = total
		if complete && !s.toComplete {
			s.current = s.total
			s.toComplete = true
		}
	}:
	case <-b.done:
	}
}

// SetRefill sets refill, if supported by underlying Filler.
func (b *Bar) SetRefill(amount int64) {
	b.operateState <- func(s *bState) {
		if f, ok := s.filler.(interface{ SetRefill(int64) }); ok {
			f.SetRefill(amount)
		}
	}
}

// Increment is a shorthand for b.IncrBy(1).
func (b *Bar) Increment() {
	b.IncrBy(1)
}

// IncrBy increments progress bar by amount of n.
// wdd is optional work duration i.e. time.Since(start), which expected
// to be provided, if any ewma based decorator is used.
func (b *Bar) IncrBy(n int, wdd ...time.Duration) {
	select {
	case b.operateState <- func(s *bState) {
		s.current += int64(n)
		if s.total > 0 && s.current >= s.total {
			s.current = s.total
			s.toComplete = true
		}
		for _, ar := range s.amountReceivers {
			ar.NextAmount(n, wdd...)
		}
	}:
	case <-b.done:
	}
}

// Completed reports whether the bar is in completed state.
func (b *Bar) Completed() bool {
	// omit select here, because primary usage of the method is for loop
	// condition, like for !bar.Completed() {...} so when toComplete=true
	// it is called once (at which time, the bar is still alive), then
	// quits the loop and never suppose to be called afterwards.
	return <-b.boolCh
}

func (b *Bar) wSyncTable() [][]chan int {
	select {
	case b.operateState <- func(s *bState) { b.syncTableCh <- s.wSyncTable() }:
		return <-b.syncTableCh
	case <-b.done:
		return b.cacheState.wSyncTable()
	}
}

func (b *Bar) serve(ctx context.Context, wg *sync.WaitGroup, s *bState) {
	defer wg.Done()
	cancel := ctx.Done()
	for {
		select {
		case op := <-b.operateState:
			op(s)
		case b.boolCh <- s.toComplete:
		case <-cancel:
			s.toComplete = true
			cancel = nil
		case <-b.shutdown:
			b.cacheState = s
			close(b.done)
			for _, sl := range s.shutdownListeners {
				sl.Shutdown()
			}
			return
		}
	}
}

func (b *Bar) render(debugOut io.Writer, tw int) {
	select {
	case b.operateState <- func(s *bState) {
		defer func() {
			// recovering if user defined decorator panics for example
			if p := recover(); p != nil {
				s.panicMsg = fmt.Sprintf("panic: %v", p)
				fmt.Fprintf(debugOut, "%s %s bar id %02d %v\n", "[mpb]", time.Now(), s.id, s.panicMsg)
				b.frameReaderCh <- &frameReader{
					Reader:     strings.NewReader(fmt.Sprintf(fmt.Sprintf("%%.%ds\n", tw), s.panicMsg)),
					toShutdown: true,
				}
			}
		}()
		r := s.draw(tw)
		var extendedLines int
		if s.newLineExtendFn != nil {
			s.bufNL.Reset()
			s.newLineExtendFn(s.bufNL, newStatistics(s))
			extendedLines = countLines(s.bufNL.Bytes())
			r = io.MultiReader(r, s.bufNL)
		}
		b.frameReaderCh <- &frameReader{
			Reader:           r,
			extendedLines:    extendedLines,
			toShutdown:       s.toComplete && !s.completeFlushed,
			removeOnComplete: s.removeOnComplete,
		}
		s.completeFlushed = s.toComplete
	}:
	case <-b.done:
		s := b.cacheState
		r := s.draw(tw)
		var extendedLines int
		if s.newLineExtendFn != nil {
			s.bufNL.Reset()
			s.newLineExtendFn(s.bufNL, newStatistics(s))
			extendedLines = countLines(s.bufNL.Bytes())
			r = io.MultiReader(r, s.bufNL)
		}
		b.frameReaderCh <- &frameReader{
			Reader:        r,
			extendedLines: extendedLines,
		}
	}
}

func (s *bState) draw(termWidth int) io.Reader {
	if s.panicMsg != "" {
		return strings.NewReader(fmt.Sprintf(fmt.Sprintf("%%.%ds\n", termWidth), s.panicMsg))
	}

	stat := newStatistics(s)

	for _, d := range s.pDecorators {
		s.bufP.WriteString(d.Decor(stat))
	}

	for _, d := range s.aDecorators {
		s.bufA.WriteString(d.Decor(stat))
	}

	if s.barClearOnComplete && s.completeFlushed {
		s.bufA.WriteByte('\n')
		return io.MultiReader(s.bufP, s.bufA)
	}

	prependCount := utf8.RuneCount(s.bufP.Bytes())
	appendCount := utf8.RuneCount(s.bufA.Bytes())

	if !s.trimSpace {
		// reserve space for edge spaces
		termWidth -= 2
		s.bufB.WriteByte(' ')
	}

	if prependCount+s.width+appendCount > termWidth {
		s.filler.Fill(s.bufB, termWidth-prependCount-appendCount, stat)
	} else {
		s.filler.Fill(s.bufB, s.width, stat)
	}

	if !s.trimSpace {
		s.bufB.WriteByte(' ')
	}

	s.bufA.WriteByte('\n')
	return io.MultiReader(s.bufP, s.bufB, s.bufA)
}

func (s *bState) wSyncTable() [][]chan int {
	columns := make([]chan int, 0, len(s.pDecorators)+len(s.aDecorators))
	var pCount int
	for _, d := range s.pDecorators {
		if ok, ch := d.Syncable(); ok {
			columns = append(columns, ch)
			pCount++
		}
	}
	var aCount int
	for _, d := range s.aDecorators {
		if ok, ch := d.Syncable(); ok {
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

func countLines(b []byte) int {
	return bytes.Count(b, []byte("\n"))
}
