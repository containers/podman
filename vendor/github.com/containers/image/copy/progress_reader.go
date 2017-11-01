package copy

import (
	"io"
	"time"

	"github.com/containers/image/types"
)

// progressReader is a reader that reports its progress on an interval.
type progressReader struct {
	source   io.Reader
	channel  chan types.ProgressProperties
	interval time.Duration
	artifact types.BlobInfo
	lastTime time.Time
	offset   uint64
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.offset += uint64(n)
	if time.Since(r.lastTime) > r.interval {
		r.channel <- types.ProgressProperties{Artifact: r.artifact, Offset: r.offset}
		r.lastTime = time.Now()
	}
	return n, err
}
