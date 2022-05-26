// Package throttler fills the gap between sync.WaitGroup and manually monitoring your goroutines
// with channels. The API is almost identical to Wait Groups, but it allows you to set
// a max number of workers that can be running simultaneously. It uses channels internally
// to block until a job completes by calling Done(err) or until all jobs have been completed.
//
// After exiting the loop where you are using Throttler, you can call the `Err` or `Errs` method to check
// for errors. `Err` will return a single error representative of all the errors Throttler caught. The
// `Errs` method will return all the errors as a slice of errors (`[]error`).
//
// Compare the Throttler example to the sync.WaitGroup example http://golang.org/pkg/sync/#example_WaitGroup
//
// See a fully functional example on the playground at http://bit.ly/throttler-v3
package throttler

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
)

// Throttler stores all the information about the number of workers, the active workers and error information
type Throttler struct {
	maxWorkers    int32
	workerCount   int32
	batchingTotal int32
	batchSize     int32
	totalJobs     int32
	jobsStarted   int32
	jobsCompleted int32
	doneChan      chan struct{}
	errsMutex     *sync.Mutex
	errs          []error
	errorCount    int32
}

// New returns a Throttler that will govern the max number of workers and will
// work with the total number of jobs. It panics if maxWorkers < 1.
func New(maxWorkers, totalJobs int) *Throttler {
	if maxWorkers < 1 {
		panic("maxWorkers has to be at least 1")
	}
	return &Throttler{
		maxWorkers: int32(maxWorkers),
		batchSize:  1,
		totalJobs:  int32(totalJobs),
		doneChan:   make(chan struct{}, totalJobs),
		errsMutex:  &sync.Mutex{},
	}
}

// NewBatchedThrottler returns a Throttler (just like New), but also enables batching.
func NewBatchedThrottler(maxWorkers, batchingTotal, batchSize int) *Throttler {
	totalJobs := int(math.Ceil(float64(batchingTotal) / float64(batchSize)))
	t := New(maxWorkers, totalJobs)
	t.batchSize = int32(batchSize)
	t.batchingTotal = int32(batchingTotal)
	return t
}

// SetMaxWorkers lets you change the total number of workers that can run concurrently. NOTE: If
// all workers are currently running, this setting is not guaranteed to take effect until one of them
// completes and Throttle() is called again
func (t *Throttler) SetMaxWorkers(maxWorkers int) {
	if maxWorkers < 1 {
		panic("maxWorkers has to be at least 1")
	}
	atomic.StoreInt32(&t.maxWorkers, int32(maxWorkers))
}

// Throttle works similarly to sync.WaitGroup, except inside your goroutine dispatch
// loop rather than after. It will not block until the number of active workers
// matches the max number of workers designated in the call to NewThrottler or
// all of the jobs have been dispatched. It stops blocking when Done has been called
// as many times as totalJobs.
func (t *Throttler) Throttle() int {
	if atomic.LoadInt32(&t.totalJobs) < 1 {
		return int(atomic.LoadInt32(&t.errorCount))
	}
	atomic.AddInt32(&t.jobsStarted, 1)
	atomic.AddInt32(&t.workerCount, 1)

	// check to see if the current number of workers equals the max number of workers
	// if they are equal, wait for one to finish before continuing
	if atomic.LoadInt32(&t.workerCount) == atomic.LoadInt32(&t.maxWorkers) {
		atomic.AddInt32(&t.jobsCompleted, 1)
		atomic.AddInt32(&t.workerCount, -1)
		<-t.doneChan
	}

	// check to see if all of the jobs have been started, and if so, wait until all
	// jobs have been completed before continuing
	if atomic.LoadInt32(&t.jobsStarted) == atomic.LoadInt32(&t.totalJobs) {
		for atomic.LoadInt32(&t.jobsCompleted) < atomic.LoadInt32(&t.totalJobs) {
			atomic.AddInt32(&t.jobsCompleted, 1)
			<-t.doneChan
		}
	}

	return int(atomic.LoadInt32(&t.errorCount))
}

// Done lets Throttler know that a job has been completed so that another worker
// can be activated. If Done is called less times than totalJobs,
// Throttle will block forever
func (t *Throttler) Done(err error) {
	if err != nil {
		t.errsMutex.Lock()
		t.errs = append(t.errs, err)
		atomic.AddInt32(&t.errorCount, 1)
		t.errsMutex.Unlock()
	}
	t.doneChan <- struct{}{}
}

// Err returns an error representative of all errors caught by throttler
func (t *Throttler) Err() error {
	t.errsMutex.Lock()
	defer t.errsMutex.Unlock()
	if atomic.LoadInt32(&t.errorCount) == 0 {
		return nil
	}
	return multiError(t.errs)
}

// Errs returns a slice of any errors that were received from calling Done()
func (t *Throttler) Errs() []error {
	t.errsMutex.Lock()
	defer t.errsMutex.Unlock()
	return t.errs
}

type multiError []error

func (te multiError) Error() string {
	errString := te[0].Error()
	if len(te) > 1 {
		errString += fmt.Sprintf(" (and %d more errors)", len(te)-1)
	}
	return errString
}

// BatchStartIndex returns the starting index for the next batch. The job count isn't modified
// until th.Throttle() is called, so if you don't call Throttle before executing this
// again, it will return the same index as before
func (t *Throttler) BatchStartIndex() int {
	return int(atomic.LoadInt32(&t.jobsStarted) * atomic.LoadInt32(&t.batchSize))
}

// BatchEndIndex returns the ending index for the next batch. It either returns the full batch size
// or the remaining amount of jobs. The job count isn't modified
// until th.Throttle() is called, so if you don't call Throttle before executing this
// again, it will return the same index as before.
func (t *Throttler) BatchEndIndex() int {
	end := (atomic.LoadInt32(&t.jobsStarted) + 1) * atomic.LoadInt32(&t.batchSize)
	if end > atomic.LoadInt32(&t.batchingTotal) {
		end = atomic.LoadInt32(&t.batchingTotal)
	}
	return int(end)
}

// TotalJobs returns the total number of jobs throttler is performing
func (t *Throttler) TotalJobs() int {
	return int(atomic.LoadInt32(&t.totalJobs))
}
