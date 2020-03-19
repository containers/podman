package containers

// LogOptions describe finer control of log content or
// how the content is formatted.
type LogOptions struct {
	Follow     *bool
	Since      *string
	Stderr     *bool
	Stdout     *bool
	Tail       *string
	Timestamps *bool
	Until      *string
}
