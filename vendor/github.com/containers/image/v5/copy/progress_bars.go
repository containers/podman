package copy

import (
	"context"
	"fmt"
	"io"

	"github.com/containers/image/v5/internal/private"
	"github.com/containers/image/v5/types"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// newProgressPool creates a *mpb.Progress.
// The caller must eventually call pool.Wait() after the pool will no longer be updated.
// NOTE: Every progress bar created within the progress pool must either successfully
// complete or be aborted, or pool.Wait() will hang. That is typically done
// using "defer bar.Abort(false)", which must be called BEFORE pool.Wait() is called.
func (c *copier) newProgressPool() *mpb.Progress {
	return mpb.New(mpb.WithWidth(40), mpb.WithOutput(c.progressOutput))
}

// customPartialBlobDecorFunc implements mpb.DecorFunc for the partial blobs retrieval progress bar
func customPartialBlobDecorFunc(s decor.Statistics) string {
	if s.Total == 0 {
		pairFmt := "%.1f / %.1f (skipped: %.1f)"
		return fmt.Sprintf(pairFmt, decor.SizeB1024(s.Current), decor.SizeB1024(s.Total), decor.SizeB1024(s.Refill))
	}
	pairFmt := "%.1f / %.1f (skipped: %.1f = %.2f%%)"
	percentage := 100.0 * float64(s.Refill) / float64(s.Total)
	return fmt.Sprintf(pairFmt, decor.SizeB1024(s.Current), decor.SizeB1024(s.Total), decor.SizeB1024(s.Refill), percentage)
}

// progressBar wraps a *mpb.Bar, allowing us to add extra state and methods.
type progressBar struct {
	*mpb.Bar
	originalSize int64 // or -1 if unknown
}

// createProgressBar creates a progressBar in pool.  Note that if the copier's reportWriter
// is io.Discard, the progress bar's output will be discarded.  Callers may call printCopyInfo()
// to print a single line instead.
//
// NOTE: Every progress bar created within a progress pool must either successfully
// complete or be aborted, or pool.Wait() will hang. That is typically done
// using "defer bar.Abort(false)", which must happen BEFORE pool.Wait() is called.
//
// As a convention, most users of progress bars should call mark100PercentComplete on full success;
// by convention, we don't leave progress bars in partial state when fully done
// (even if we copied much less data than anticipated).
func (c *copier) createProgressBar(pool *mpb.Progress, partial bool, info types.BlobInfo, kind string, onComplete string) *progressBar {
	// shortDigestLen is the length of the digest used for blobs.
	const shortDigestLen = 12

	prefix := fmt.Sprintf("Copying %s %s", kind, info.Digest.Encoded())
	// Truncate the prefix (chopping of some part of the digest) to make all progress bars aligned in a column.
	maxPrefixLen := len("Copying blob ") + shortDigestLen
	if len(prefix) > maxPrefixLen {
		prefix = prefix[:maxPrefixLen]
	}

	// onComplete will replace prefix once the bar/spinner has completed
	onComplete = prefix + " " + onComplete

	// Use a normal progress bar when we know the size (i.e., size > 0).
	// Otherwise, use a spinner to indicate that something's happening.
	var bar *mpb.Bar
	if info.Size > 0 {
		if partial {
			bar = pool.AddBar(info.Size,
				mpb.BarFillerClearOnComplete(),
				mpb.PrependDecorators(
					decor.OnComplete(decor.Name(prefix), onComplete),
				),
				mpb.AppendDecorators(
					decor.Any(customPartialBlobDecorFunc),
				),
			)
		} else {
			bar = pool.AddBar(info.Size,
				mpb.BarFillerClearOnComplete(),
				mpb.PrependDecorators(
					decor.OnComplete(decor.Name(prefix), onComplete),
				),
				mpb.AppendDecorators(
					decor.OnComplete(decor.CountersKibiByte("%.1f / %.1f"), ""),
				),
			)
		}
	} else {
		bar = pool.New(0,
			mpb.SpinnerStyle(".", "..", "...", "....", "").PositionLeft(),
			mpb.BarFillerClearOnComplete(),
			mpb.PrependDecorators(
				decor.OnComplete(decor.Name(prefix), onComplete),
			),
		)
	}
	return &progressBar{
		Bar:          bar,
		originalSize: info.Size,
	}
}

// printCopyInfo prints a "Copying ..." message on the copier if the output is
// set to `io.Discard`.  In that case, the progress bars won't be rendered but
// we still want to indicate when blobs and configs are copied.
func (c *copier) printCopyInfo(kind string, info types.BlobInfo) {
	if c.progressOutput == io.Discard {
		c.Printf("Copying %s %s\n", kind, info.Digest)
	}
}

// mark100PercentComplete marks the progres bars as 100% complete;
// it may do so by possibly advancing the current state if it is below the known total.
func (bar *progressBar) mark100PercentComplete() {
	if bar.originalSize > 0 {
		// We can't call bar.SetTotal even if we wanted to; the total can not be changed
		// after a progress bar is created with a definite total.
		bar.SetCurrent(bar.originalSize) // This triggers the completion condition.
	} else {
		// -1 = unknown size
		// 0 is somewhat of a special case: Unlike c/image, where 0 is a definite known
		// size (possible at least in theory), in mpb, zero-sized progress bars are treated
		// as unknown size, in particular they are not configured to be marked as
		// complete on bar.Current() reaching bar.total (because that would happen already
		// when creating the progress bar).
		// That means that we are both _allowed_ to call SetTotal, and we _have to_.
		bar.SetTotal(-1, true) // total < 0 = set it to bar.Current(), report it; and mark the bar as complete.
	}
}

// blobChunkAccessorProxy wraps a BlobChunkAccessor and updates a *progressBar
// with the number of received bytes.
type blobChunkAccessorProxy struct {
	wrapped private.BlobChunkAccessor // The underlying BlobChunkAccessor
	bar     *progressBar              // A progress bar updated with the number of bytes read so far
}

// GetBlobAt returns a sequential channel of readers that contain data for the requested
// blob chunks, and a channel that might get a single error value.
// The specified chunks must be not overlapping and sorted by their offset.
// The readers must be fully consumed, in the order they are returned, before blocking
// to read the next chunk.
func (s *blobChunkAccessorProxy) GetBlobAt(ctx context.Context, info types.BlobInfo, chunks []private.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
	rc, errs, err := s.wrapped.GetBlobAt(ctx, info, chunks)
	if err == nil {
		total := int64(0)
		for _, c := range chunks {
			total += int64(c.Length)
		}
		s.bar.IncrInt64(total)
	}
	return rc, errs, err
}
