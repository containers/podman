package entities

// AutoUpdateReport contains the results from running auto-update.
type AutoUpdateReport struct {
	// Units - the restarted systemd units during auto-update.
	Units []string
}
