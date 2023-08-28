package decor

// OnCompleteOrOnAbort wrap decorator.
// Displays provided message on complete or on abort event.
//
//	`decorator` Decorator to wrap
//	`message` message to display
func OnCompleteOrOnAbort(decorator Decorator, message string) Decorator {
	return OnComplete(OnAbort(decorator, message), message)
}
