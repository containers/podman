package copy

import (
	"context"
	"io"
	"time"

	internalTypes "github.com/containers/image/v5/internal/types"
	"github.com/containers/image/v5/types"
)

// progressReader is a reader that reports its progress on an interval.
type progressReader struct {
	source       io.Reader
	channel      chan<- types.ProgressProperties
	interval     time.Duration
	artifact     types.BlobInfo
	lastUpdate   time.Time
	offset       uint64
	offsetUpdate uint64
}

// newProgressReader creates a new progress reader for:
// `source`:   The source when internally reading bytes
// `channel`:  The reporter channel to which the progress will be sent
// `interval`: The update interval to indicate how often the progress should update
// `artifact`: The blob metadata which is currently being progressed
func newProgressReader(
	source io.Reader,
	channel chan<- types.ProgressProperties,
	interval time.Duration,
	artifact types.BlobInfo,
) *progressReader {
	// The progress reader constructor informs the progress channel
	// that a new artifact will be read
	channel <- types.ProgressProperties{
		Event:    types.ProgressEventNewArtifact,
		Artifact: artifact,
	}
	return &progressReader{
		source:       source,
		channel:      channel,
		interval:     interval,
		artifact:     artifact,
		lastUpdate:   time.Now(),
		offset:       0,
		offsetUpdate: 0,
	}
}

// reportDone indicates to the internal channel that the progress has been
// finished
func (r *progressReader) reportDone() {
	r.channel <- types.ProgressProperties{
		Event:        types.ProgressEventDone,
		Artifact:     r.artifact,
		Offset:       r.offset,
		OffsetUpdate: r.offsetUpdate,
	}
}

// Read continuously reads bytes into the progress reader and reports the
// status via the internal channel
func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.offset += uint64(n)
	r.offsetUpdate += uint64(n)

	// Fire the progress reader in the provided interval
	if time.Since(r.lastUpdate) > r.interval {
		r.channel <- types.ProgressProperties{
			Event:        types.ProgressEventRead,
			Artifact:     r.artifact,
			Offset:       r.offset,
			OffsetUpdate: r.offsetUpdate,
		}
		r.lastUpdate = time.Now()
		r.offsetUpdate = 0
	}
	return n, err
}

// imageSourceSeekableProxy wraps ImageSourceSeekable and keeps track of how many bytes
// are received.
type imageSourceSeekableProxy struct {
	// source is the seekable input to read from.
	source internalTypes.ImageSourceSeekable
	// progress is the chan where the total number of bytes read so far are reported.
	progress chan int64
}

// GetBlobAt reads from the ImageSourceSeekable and report how many bytes were received
// to the progress chan.
func (s imageSourceSeekableProxy) GetBlobAt(ctx context.Context, bInfo types.BlobInfo, chunks []internalTypes.ImageSourceChunk) (chan io.ReadCloser, chan error, error) {
	rc, errs, err := s.source.GetBlobAt(ctx, bInfo, chunks)
	if err == nil {
		total := int64(0)
		for _, c := range chunks {
			total += int64(c.Length)
		}
		s.progress <- total
	}
	return rc, errs, err
}
