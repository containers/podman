package channelwriter

import "github.com/pkg/errors"

// Writer is an io.writer-like object that "writes" to a channel
// instead of a buffer or file, etc. It is handy for varlink endpoints when
// needing to handle endpoints that do logging "real-time"
type Writer struct {
	ByteChannel chan []byte
}

// NewChannelWriter creates a new channel writer and adds a
// byte slice channel into it.
func NewChannelWriter() *Writer {
	byteChannel := make(chan []byte)
	return &Writer{
		ByteChannel: byteChannel,
	}
}

// Write method for Writer
func (c *Writer) Write(w []byte) (int, error) {
	if c.ByteChannel == nil {
		return 0, errors.New("channel writer channel cannot be nil")
	}
	c.ByteChannel <- w
	return len(w), nil
}

// Close method for Writer
func (c *Writer) Close() error {
	close(c.ByteChannel)
	return nil
}
