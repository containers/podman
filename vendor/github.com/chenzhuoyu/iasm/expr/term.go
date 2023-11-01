package expr

// Term represents a value that can Evaluate() into an integer.
type Term interface {
    Free()
    Evaluate() (int64, error)
}
