package decor

// OnPredicate returns decorator if predicate evaluates to true.
//
//	`decorator` Decorator
//
//	`predicate` func() bool
//
func OnPredicate(decorator Decorator, predicate func() bool) Decorator {
	if predicate() {
		return decorator
	}
	return nil
}

// OnCondition returns decorator if condition is true.
//
//	`decorator` Decorator
//
//	`cond` bool
//
func OnCondition(decorator Decorator, cond bool) Decorator {
	if cond {
		return decorator
	}
	return nil
}
