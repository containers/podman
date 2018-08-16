package types

// PsContext controls some internals of the psgo library.
type PsContext struct {
	// JoinUserNS will force /proc and /dev parsing from within each PIDs
	// user namespace.
	JoinUserNS bool
}
