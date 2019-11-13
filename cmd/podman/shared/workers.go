package shared

import (
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// JobFunc provides the function signature for the pool'ed functions
type JobFunc func() error

// Job defines the function to run
type Job struct {
	ID string
	Fn JobFunc
}

// JobResult defines the results from the function ran
type JobResult struct {
	Job Job
	Err error
}

// Pool defines the worker pool and queues
type Pool struct {
	id       string
	wg       *sync.WaitGroup
	jobs     chan Job
	results  chan JobResult
	size     int
	capacity int
}

// NewPool creates and initializes a new Pool
func NewPool(id string, size int, capacity int) *Pool {
	var wg sync.WaitGroup

	// min for int...
	s := size
	if s > capacity {
		s = capacity
	}

	return &Pool{
		id,
		&wg,
		make(chan Job, capacity),
		make(chan JobResult, capacity),
		s,
		capacity,
	}
}

// Add Job to pool for parallel processing
func (p *Pool) Add(job Job) {
	p.wg.Add(1)
	p.jobs <- job
}

// Run the Job's in the pool, gather and return results
func (p *Pool) Run() ([]string, map[string]error, error) {
	var (
		ok       = []string{}
		failures = map[string]error{}
	)

	for w := 0; w < p.size; w++ {
		w := w
		go p.newWorker(w)
	}
	close(p.jobs)
	p.wg.Wait()

	close(p.results)
	for r := range p.results {
		if r.Err == nil {
			ok = append(ok, r.Job.ID)
		} else {
			failures[r.Job.ID] = r.Err
		}
	}

	if logrus.GetLevel() == logrus.DebugLevel {
		for i, f := range failures {
			logrus.Debugf("Pool[%s, %s: %s]", p.id, i, f.Error())
		}
	}

	return ok, failures, nil
}

// newWorker creates new parallel workers to monitor jobs channel from Pool
func (p *Pool) newWorker(slot int) {
	for job := range p.jobs {
		err := job.Fn()
		p.results <- JobResult{job, err}
		if logrus.GetLevel() == logrus.DebugLevel {
			n := strings.Split(runtime.FuncForPC(reflect.ValueOf(job.Fn).Pointer()).Name(), ".")
			logrus.Debugf("Worker#%d finished job %s/%s (%v)", slot, n[2:], job.ID, err)
		}
		p.wg.Done()
	}
}

// DefaultPoolSize provides the maximum number of parallel workers (int) as calculated by a basic
// heuristic. This can be overridden by the --max-workers primary switch to podman.
func DefaultPoolSize(name string) int {
	numCpus := runtime.NumCPU()
	switch name {
	case "init":
		fallthrough
	case "kill":
		fallthrough
	case "pause":
		fallthrough
	case "rm":
		fallthrough
	case "unpause":
		if numCpus <= 3 {
			return numCpus * 3
		}
		return numCpus * 4
	case "ps":
		return 8
	case "restart":
		return numCpus * 2
	case "stop":
		if numCpus <= 2 {
			return 4
		} else {
			return numCpus * 3
		}
	}
	return 3
}
