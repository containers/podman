package mpb

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/vbauerster/mpb/decor"
	"github.com/vbauerster/mpb/internal"
)

const (
	rLeft = iota
	rFill
	rTip
	rEmpty
	rRight
)

const formatLen = 5

type barRunes [formatLen]rune

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

type (
	bState struct {
		id                 int
		width              int
		total              int64
		current            int64
		runes              barRunes
		trimLeftSpace      bool
		trimRightSpace     bool
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
		char rune
		till int64
	}
	frameReader struct {
		io.Reader
		extendedLines    int
		toShutdown       bool
		removeOnComplete bool
	}
)

func newBar(wg *sync.WaitGroup, id int, total int64, cancel <-chan struct{}, options ...BarOption) *Bar {
	if total <= 0 {
		total = time.Now().Unix()
	}

	s := &bState{
		id:       id,
		priority: id,
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

	if s.newLineExtendFn != nil {
		s.bufNL = bytes.NewBuffer(make([]byte, 0, s.width))
	}

	go b.serve(wg, s, cancel)
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
// Set final to true, when total is known, it will trigger bar complete event.
func (b *Bar) SetTotal(total int64, final bool) bool {
	select {
	case b.operateState <- func(s *bState) {
		if total > 0 {
			s.total = total
		}
		if final {
			s.current = s.total
			s.toComplete = true
		}
	}:
		return true
	case <-b.done:
		return false
	}
}

// SetRefill sets fill rune to r, up until n.
func (b *Bar) SetRefill(n int, r rune) {
	if n <= 0 {
		return
	}
	b.operateState <- func(s *bState) {
		s.refill = &refill{r, int64(n)}
	}
}

// RefillBy is deprecated, use SetRefill
func (b *Bar) RefillBy(n int, r rune) {
	b.SetRefill(n, r)
}

// Increment is a shorthand for b.IncrBy(1).
func (b *Bar) Increment() {
	b.IncrBy(1)
}

// IncrBy increments progress bar by amount of n.
// wdd is optional work duration i.e. time.Since(start),
// which expected to be provided, if any ewma based decorator is used.
func (b *Bar) IncrBy(n int, wdd ...time.Duration) {
	select {
	case b.operateState <- func(s *bState) {
		s.current += int64(n)
		if s.current >= s.total {
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
	// condition, like 	for !bar.Completed() {...}
	// so when toComplete=true it is called once (at which time, the bar is still alive),
	// then quits the loop and never suppose to be called afterwards.
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

func (b *Bar) serve(wg *sync.WaitGroup, s *bState, cancel <-chan struct{}) {
	defer wg.Done()
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
	defer s.bufA.WriteByte('\n')

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

	prependCount := utf8.RuneCount(s.bufP.Bytes())
	appendCount := utf8.RuneCount(s.bufA.Bytes())

	if s.barClearOnComplete && s.completeFlushed {
		return io.MultiReader(s.bufP, s.bufA)
	}

	s.fillBar(s.width)
	barCount := utf8.RuneCount(s.bufB.Bytes())
	totalCount := prependCount + barCount + appendCount
	if spaceCount := 0; totalCount > termWidth {
		if !s.trimLeftSpace {
			spaceCount++
		}
		if !s.trimRightSpace {
			spaceCount++
		}
		s.fillBar(termWidth - prependCount - appendCount - spaceCount)
	}

	return io.MultiReader(s.bufP, s.bufB, s.bufA)
}

func (s *bState) fillBar(width int) {
	defer func() {
		s.bufB.WriteRune(s.runes[rRight])
		if !s.trimRightSpace {
			s.bufB.WriteByte(' ')
		}
	}()

	s.bufB.Reset()
	if !s.trimLeftSpace {
		s.bufB.WriteByte(' ')
	}
	s.bufB.WriteRune(s.runes[rLeft])
	if width <= 2 {
		return
	}

	// bar s.width without leftEnd and rightEnd runes
	barWidth := width - 2

	completedWidth := internal.Percentage(s.total, s.current, int64(barWidth))

	if s.refill != nil {
		till := internal.Percentage(s.total, s.refill.till, int64(barWidth))
		// append refill rune
		var i int64
		for i = 0; i < till; i++ {
			s.bufB.WriteRune(s.refill.char)
		}
		for i = till; i < completedWidth; i++ {
			s.bufB.WriteRune(s.runes[rFill])
		}
	} else {
		var i int64
		for i = 0; i < completedWidth; i++ {
			s.bufB.WriteRune(s.runes[rFill])
		}
	}

	if completedWidth < int64(barWidth) && completedWidth > 0 {
		_, size := utf8.DecodeLastRune(s.bufB.Bytes())
		s.bufB.Truncate(s.bufB.Len() - size)
		s.bufB.WriteRune(s.runes[rTip])
	}

	for i := completedWidth; i < int64(barWidth); i++ {
		s.bufB.WriteRune(s.runes[rEmpty])
	}
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

func strToBarRunes(format string) (array barRunes) {
	for i, n := 0, 0; len(format) > 0; i++ {
		array[i], n = utf8.DecodeRuneInString(format)
		format = format[n:]
	}
	return
}

func countLines(b []byte) int {
	return bytes.Count(b, []byte("\n"))
}
