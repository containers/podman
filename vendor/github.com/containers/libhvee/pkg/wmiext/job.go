//go:build windows
// +build windows

package wmiext

import (
	"fmt"
	"time"
)

type JobError struct {
	ErrorCode int
}

func (err *JobError) Error() string {
	return fmt.Sprintf("Job failed with error code: %d", err.ErrorCode)
}

// WaitJob waits on the specified job instance until it has completed and
// returns a JobError containing the result code in the event of
// a failure.
func WaitJob(service *Service, job *Instance) error {
	var jobs []*Instance
	defer func() {
		for _, job := range jobs {
			job.Close()
		}
	}()
	for {
		state, _, _, err := job.GetAsAny("JobState")
		if err != nil {
			return err
		}
		time.Sleep(100 * time.Millisecond)
		job, _ = service.RefetchObject(job)
		jobs = append(jobs, job)
		// 7+ = completed
		if state.(int32) >= 7 {
			break
		}
	}

	result, _, _, err := job.GetAsAny("ErrorCode")
	if err != nil {
		return err
	}

	if result.(int32) != 0 {
		return &JobError{ErrorCode: int(result.(int32))}
	}

	return nil
}
