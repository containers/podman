package parallel

import (
	"context"
	"sync"

	"github.com/containers/podman/v2/libpod"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ContainerOp performs the given function on the given set of
// containers, using a number of parallel threads.
// If no error is returned, each container specified in ctrs will have an entry
// in the resulting map; containers with no error will be set to nil.
func ContainerOp(ctx context.Context, ctrs []*libpod.Container, applyFunc func(*libpod.Container) error) (map[*libpod.Container]error, error) {
	jobControlLock.RLock()
	defer jobControlLock.RUnlock()

	// We could use a sync.Map but given Go's lack of generic I'd rather
	// just use a lock on a normal map...
	// The expectation is that most of the time is spent in applyFunc
	// anyways.
	var (
		errMap  = make(map[*libpod.Container]error)
		errLock sync.Mutex
		allDone sync.WaitGroup
	)

	for _, ctr := range ctrs {
		// Block until a thread is available
		if err := jobControl.Acquire(ctx, 1); err != nil {
			return nil, errors.Wrapf(err, "error acquiring job control semaphore")
		}

		allDone.Add(1)

		c := ctr
		go func() {
			logrus.Debugf("Launching job on container %s", c.ID())

			err := applyFunc(c)
			errLock.Lock()
			errMap[c] = err
			errLock.Unlock()

			allDone.Done()
			jobControl.Release(1)
		}()
	}

	allDone.Wait()

	return errMap, nil
}

// TODO: Add an Enqueue() function that returns a promise
