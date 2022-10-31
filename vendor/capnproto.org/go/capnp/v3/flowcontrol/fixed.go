package flowcontrol

import (
	"context"
	"fmt"
	"math"

	"golang.org/x/sync/semaphore"
)

// Returns a FlowLimiter that enforces a fixed limit on the total size of
// outstanding messages.
func NewFixedLimiter(size int64) FlowLimiter {
	return (*fixedLimiter)(semaphore.NewWeighted(size))
}

type fixedLimiter semaphore.Weighted

func (fl *fixedLimiter) StartMessage(ctx context.Context, size uint64) (gotResponse func(), err error) {
	if size > math.MaxInt64 {
		// semaphore.Weighted expects an int64, so we need to check the bounds.
		return nil, fmt.Errorf(
			"StartMessage(): limit %v is too large (max %v)",
			size, int64(math.MaxInt64),
		)
	}
	w := (*semaphore.Weighted)(fl)
	err = w.Acquire(ctx, int64(size))
	if err != nil {
		return nil, err
	}
	return func() {
		w.Release(int64(size))
	}, nil
}

func (fixedLimiter) Release() {}
