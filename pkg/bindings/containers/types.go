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

// CommitOptions describe details about the resulting committed
// image as defined by repo and tag. None of these options
// are required.
type CommitOptions struct {
	Author  *string
	Changes []string
	Comment *string
	Format  *string
	Pause   *bool
	Repo    *string
	Tag     *string
}
