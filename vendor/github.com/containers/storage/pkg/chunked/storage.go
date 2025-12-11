package chunked

import (
	"io"
)

// ImageSourceChunk is a portion of a blob.
type ImageSourceChunk struct {
	Offset uint64
	Length uint64
}

// ImageSourceSeekable is an image source that permits to fetch chunks of the entire blob.
type ImageSourceSeekable interface {
	// GetBlobAt returns a stream for the specified blob.
	GetBlobAt([]ImageSourceChunk) (chan io.ReadCloser, chan error, error)
}

// ErrBadRequest is returned when the request is not valid
type ErrBadRequest struct { //nolint: errname
}

func (e ErrBadRequest) Error() string {
	return "bad request"
}
