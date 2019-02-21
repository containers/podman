//+build go1.7

package mpb

import "context"

// WithContext provided context will be used for cancellation purposes
func WithContext(ctx context.Context) ProgressOption {
	return func(s *pState) {
		if ctx == nil {
			panic("ctx must not be nil")
		}
		s.cancel = ctx.Done()
	}
}
