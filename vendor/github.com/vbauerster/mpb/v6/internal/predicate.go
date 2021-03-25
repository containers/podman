package internal

// Predicate helper for internal use.
func Predicate(pick bool) func() bool {
	return func() bool { return pick }
}
