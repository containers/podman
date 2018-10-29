package shared

import (
	"runtime"
	"sync"
)

type pFunc func() error

// ParallelWorkerInput is a struct used to pass in a slice of parallel funcs to be
// performed on a container ID
type ParallelWorkerInput struct {
	ContainerID  string
	ParallelFunc pFunc
}

type containerError struct {
	ContainerID string
	Err         error
}

// ParallelWorker is a "threaded" worker that takes jobs from the channel "queue"
func ParallelWorker(wg *sync.WaitGroup, jobs <-chan ParallelWorkerInput, results chan<- containerError) {
	for j := range jobs {
		err := j.ParallelFunc()
		results <- containerError{ContainerID: j.ContainerID, Err: err}
		wg.Done()
	}
}

// ParallelExecuteWorkerPool takes container jobs and performs them in parallel.  The worker
// int determines how many workers/threads should be premade.
func ParallelExecuteWorkerPool(workers int, functions []ParallelWorkerInput) map[string]error {
	var (
		wg sync.WaitGroup
	)

	resultChan := make(chan containerError, len(functions))
	results := make(map[string]error)
	paraJobs := make(chan ParallelWorkerInput, len(functions))

	// If we have more workers than functions, match up the number of workers and functions
	if workers > len(functions) {
		workers = len(functions)
	}

	// Create the workers
	for w := 1; w <= workers; w++ {
		go ParallelWorker(&wg, paraJobs, resultChan)
	}

	// Add jobs to the workers
	for _, j := range functions {
		j := j
		wg.Add(1)
		paraJobs <- j
	}

	close(paraJobs)
	wg.Wait()

	close(resultChan)
	for ctrError := range resultChan {
		results[ctrError.ContainerID] = ctrError.Err
	}

	return results
}

// Parallelize provides the maximum number of parallel workers (int) as calculated by a basic
// heuristic. This can be overriden by the --max-workers primary switch to podman.
func Parallelize(job string) int {
	numCpus := runtime.NumCPU()
	switch job {
	case "kill":
		if numCpus <= 3 {
			return numCpus * 3
		}
		return numCpus * 4
	case "pause":
		if numCpus <= 3 {
			return numCpus * 3
		}
		return numCpus * 4
	case "ps":
		return 8
	case "restart":
		return numCpus * 2
	case "rm":
		if numCpus <= 3 {
			return numCpus * 3
		} else {
			return numCpus * 4
		}
	case "stop":
		if numCpus <= 2 {
			return 4
		} else {
			return numCpus * 3
		}
	case "unpause":
		if numCpus <= 3 {
			return numCpus * 3
		}
		return numCpus * 4
	}
	return 3
}
