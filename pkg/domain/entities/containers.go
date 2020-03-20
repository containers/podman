package entities

import (
	"time"

	"github.com/containers/libpod/libpod/define"
)

type WaitOptions struct {
	Condition define.ContainerStatus
	Interval  time.Duration
	Latest    bool
}

type WaitReport struct {
	Id       string
	Error    error
	ExitCode int32
}

type BoolReport struct {
	Value bool
}
