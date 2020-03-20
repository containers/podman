package entities

import "time"

type WaitOptions struct {
	Condition string
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
