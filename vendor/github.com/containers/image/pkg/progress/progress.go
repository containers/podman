// Package progress exposes a convenience API and abstractions for creating and
// managing pools of multi-progress bars.
package progress

import (
	"context"
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"
	"github.com/vbauerster/mpb"
	"github.com/vbauerster/mpb/decor"
)

// Pool allows managing progress bars.
type Pool struct {
	pool   *mpb.Progress
	cancel context.CancelFunc
	writer io.Writer
}

// Bar represents a progress bar.
type Bar struct {
	bar  *mpb.Bar
	pool *Pool
}

// BarOptions includes various options to control AddBar.
type BarOptions struct {
	// RemoveOnCompletion the bar on completion. This must be true if the bar
	// will be replaced by another one.
	RemoveOnCompletion bool
	// OnCompletionMessage will be shown on completion and replace the progress
	// bar. Note that setting OnCompletionMessage will cause the progress bar
	// (or the static message) to be cleared on completion.
	OnCompletionMessage string
	// ReplaceBar is the bar to replace.
	ReplaceBar *Bar
	// StaticMessage, if set, will replace displaying the progress bar. Use this
	// field for static progress bars that do not show progress.
	StaticMessage string
}

// NewPool returns a Pool. The caller must eventually call ProgressPool.CleanUp()
// when the pool will no longer be updated.
func NewPool(ctx context.Context, writer io.Writer) *Pool {
	ctx, cancelFunc := context.WithCancel(ctx)
	return &Pool{
		pool:   mpb.New(mpb.WithWidth(40), mpb.WithOutput(writer), mpb.WithContext(ctx)),
		cancel: cancelFunc,
		writer: writer,
	}
}

// CleanUp cleans up resources, such as remaining progress bars, and stops them
// if necessary.
func (p *Pool) CleanUp() {
	p.cancel()
	p.pool.Wait()
}

// DigestToCopyAction returns a string based on the blobinfo and kind.
// It's a convenience function for the c/image library when copying images.
func DigestToCopyAction(digest digest.Digest, kind string) string {
	// shortDigestLen is the length of the digest used for blobs.
	const shortDigestLen = 12
	const maxLen = len("Copying blob ") + shortDigestLen
	// Truncate the string (chopping of some part of the digest) to make all
	// progress bars aligned in a column.
	copyAction := fmt.Sprintf("Copying %s %s", kind, digest.Encoded())
	if len(copyAction) > maxLen {
		copyAction = copyAction[:maxLen]
	}
	return copyAction
}

// AddBar adds a new Bar to the Pool. Use options to control the behavior and
// appearance of the bar.
func (p *Pool) AddBar(action string, size int64, options BarOptions) *Bar {
	var bar *mpb.Bar

	// First decorator showing action (e.g., "Copying blob 123456abcd")
	mpbOptions := []mpb.BarOption{
		mpb.PrependDecorators(
			decor.Name(action),
		),
	}

	if options.RemoveOnCompletion {
		mpbOptions = append(mpbOptions, mpb.BarRemoveOnComplete())
	}

	if options.ReplaceBar != nil {
		mpbOptions = append(mpbOptions, mpb.BarReplaceOnComplete(options.ReplaceBar.bar))
		// bar.SetTotal(0, true) will make sure that the bar is stopped
		defer options.ReplaceBar.bar.SetTotal(0, true)
	}

	// If no static message is set, we display the progress bar. Otherwise,
	// we'll display the message only.
	if options.StaticMessage == "" {
		mpbOptions = append(mpbOptions,
			mpb.AppendDecorators(
				decor.OnComplete(decor.CountersKibiByte("%.1f / %.1f"), " "+options.OnCompletionMessage),
			),
		)
		mpbOptions = append(mpbOptions, mpb.BarClearOnComplete())
		bar = p.pool.AddBar(size, mpbOptions...)
		return &Bar{
			bar:  bar,
			pool: p,
		}
	}

	barFiller := mpb.FillerFunc(
		func(w io.Writer, width int, st *decor.Statistics) {
			fmt.Fprint(w, options.StaticMessage)
		})

	// If OnCompletionMessage is set, we need to add the decorator and clear
	// the bar on completion.
	if options.OnCompletionMessage != "" {
		mpbOptions = append(mpbOptions,
			mpb.AppendDecorators(
				decor.OnComplete(decor.Name(""), " "+options.OnCompletionMessage),
			),
		)
		mpbOptions = append(mpbOptions, mpb.BarClearOnComplete())
	}

	bar = p.pool.Add(size, barFiller, mpbOptions...)
	return &Bar{
		bar:  bar,
		pool: p,
	}
}

// ReplaceBar is like Pool.AddBar but replace the bar in it's pool. Note that
// the bar be terminated and should have been created with
// options.RemoveOnCompletion.
func (b *Bar) ReplaceBar(action string, size int64, options BarOptions) *Bar {
	options.ReplaceBar = b
	return b.pool.AddBar(action, size, options)
}

// ProxyReader wraps the reader with metrics for progress tracking.
func (b *Bar) ProxyReader(reader io.Reader) io.ReadCloser {
	return b.bar.ProxyReader(reader)
}
