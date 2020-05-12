package entities

// AutoUpdateOptions are the options for running auto-update.
type AutoUpdateOptions struct {
	// Authfile to use when contacting registries.
	Authfile string
}

// AutoUpdateReport contains the results from running auto-update.
type AutoUpdateReport struct {
	// Units - the restarted systemd units during auto-update.
	Units []string
}
