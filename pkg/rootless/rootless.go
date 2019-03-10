package rootless

// Opts allows to customize how re-execing to a rootless process is done
type Opts struct {
	// Argument overrides the arguments on the command line
	// for the re-execed process.  The process in the namespace
	// must use rootless.Argument() to read its value.
	Argument string
}
