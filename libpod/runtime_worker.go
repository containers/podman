package libpod

func (r *Runtime) startWorker() {
	r.workerChannel = make(chan func(), 10)
	go func() {
		for w := range r.workerChannel {
			w()
			r.workerGroup.Done()
		}
	}()
}

func (r *Runtime) queueWork(f func()) {
	r.workerGroup.Add(1)
	go func() {
		r.workerChannel <- f
	}()
}
