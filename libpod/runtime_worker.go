package libpod

import (
	"time"
)

func (r *Runtime) startWorker() {
	if r.workerChannel == nil {
		r.workerChannel = make(chan func(), 1)
		r.workerShutdown = make(chan bool)
	}
	go func() {
		for {
			// Make sure to read all workers before
			// checking if we're about to shutdown.
			for len(r.workerChannel) > 0 {
				w := <-r.workerChannel
				w()
			}

			select {
			// We'll read from the shutdown channel only when all
			// items above have been processed.
			//
			// (*Runtime).Shutdown() will block until until the
			// item is read.
			case <-r.workerShutdown:
				return

			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (r *Runtime) queueWork(f func()) {
	go func() {
		r.workerChannel <- f
	}()
}
