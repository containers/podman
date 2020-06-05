package parallel

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

var (
	// Maximum number of jobs that will be used.
	// Set a low, but non-zero, default. We'll be overriding it by default
	// anyways.
	numThreads uint = 8
	// Semaphore to control thread creation and ensure numThreads is
	// respected.
	jobControl *semaphore.Weighted
	// Lock to control changing the semaphore - we don't want to do it
	// while anyone is using it.
	jobControlLock sync.RWMutex
)

// SetMaxThreads sets the number of threads that will be used for parallel jobs.
func SetMaxThreads(threads uint) error {
	if threads == 0 {
		return errors.New("must give a non-zero number of threads to execute with")
	}

	jobControlLock.Lock()
	defer jobControlLock.Unlock()

	numThreads = threads
	jobControl = semaphore.NewWeighted(int64(threads))
	logrus.Infof("Setting parallel job count to %d", threads)

	return nil
}

// GetMaxThreads returns the current number of threads that will be used for
// parallel jobs.
func GetMaxThreads() uint {
	return numThreads
}
