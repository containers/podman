package daemon

import (
	"context"
	"io"

	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/docker/tarfile"
	"github.com/containers/image/v5/types"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type daemonImageDestination struct {
	ref                  daemonReference
	mustMatchRuntimeOS   bool
	*tarfile.Destination // Implements most of types.ImageDestination
	// For talking to imageLoadGoroutine
	goroutineCancel context.CancelFunc
	statusChannel   <-chan error
	writer          *io.PipeWriter
	// Other state
	committed bool // writer has been closed
}

// newImageDestination returns a types.ImageDestination for the specified image reference.
func newImageDestination(ctx context.Context, sys *types.SystemContext, ref daemonReference) (types.ImageDestination, error) {
	if ref.ref == nil {
		return nil, errors.Errorf("Invalid destination docker-daemon:%s: a destination must be a name:tag", ref.StringWithinTransport())
	}
	namedTaggedRef, ok := ref.ref.(reference.NamedTagged)
	if !ok {
		return nil, errors.Errorf("Invalid destination docker-daemon:%s: a destination must be a name:tag", ref.StringWithinTransport())
	}

	var mustMatchRuntimeOS = true
	if sys != nil && sys.DockerDaemonHost != client.DefaultDockerHost {
		mustMatchRuntimeOS = false
	}

	c, err := newDockerClient(sys)
	if err != nil {
		return nil, errors.Wrap(err, "Error initializing docker engine client")
	}

	reader, writer := io.Pipe()
	// Commit() may never be called, so we may never read from this channel; so, make this buffered to allow imageLoadGoroutine to write status and terminate even if we never read it.
	statusChannel := make(chan error, 1)

	goroutineContext, goroutineCancel := context.WithCancel(ctx)
	go imageLoadGoroutine(goroutineContext, c, reader, statusChannel)

	return &daemonImageDestination{
		ref:                ref,
		mustMatchRuntimeOS: mustMatchRuntimeOS,
		Destination:        tarfile.NewDestinationWithContext(sys, writer, namedTaggedRef),
		goroutineCancel:    goroutineCancel,
		statusChannel:      statusChannel,
		writer:             writer,
		committed:          false,
	}, nil
}

// imageLoadGoroutine accepts tar stream on reader, sends it to c, and reports error or success by writing to statusChannel
func imageLoadGoroutine(ctx context.Context, c *client.Client, reader *io.PipeReader, statusChannel chan<- error) {
	err := errors.New("Internal error: unexpected panic in imageLoadGoroutine")
	defer func() {
		logrus.Debugf("docker-daemon: sending done, status %v", err)
		statusChannel <- err
	}()
	defer func() {
		if err == nil {
			reader.Close()
		} else {
			if err := reader.CloseWithError(err); err != nil {
				logrus.Debugf("imageLoadGoroutine: Error during reader.CloseWithError: %v", err)
			}
		}
	}()

	resp, err := c.ImageLoad(ctx, reader, true)
	if err != nil {
		err = errors.Wrap(err, "Error saving image to docker engine")
		return
	}
	defer resp.Body.Close()
}

// DesiredLayerCompression indicates if layers must be compressed, decompressed or preserved
func (d *daemonImageDestination) DesiredLayerCompression() types.LayerCompression {
	return types.PreserveOriginal
}

// MustMatchRuntimeOS returns true iff the destination can store only images targeted for the current runtime architecture and OS. False otherwise.
func (d *daemonImageDestination) MustMatchRuntimeOS() bool {
	return d.mustMatchRuntimeOS
}

// Close removes resources associated with an initialized ImageDestination, if any.
func (d *daemonImageDestination) Close() error {
	if !d.committed {
		logrus.Debugf("docker-daemon: Closing tar stream to abort loading")
		// In principle, goroutineCancel() should abort the HTTP request and stop the process from continuing.
		// In practice, though, various HTTP implementations used by client.Client.ImageLoad() (including
		// https://github.com/golang/net/blob/master/context/ctxhttp/ctxhttp_pre17.go and the
		// net/http version with native Context support in Go 1.7) do not always actually immediately cancel
		// the operation: they may process the HTTP request, or a part of it, to completion in a goroutine, and
		// return early if the context is canceled without terminating the goroutine at all.
		// So we need this CloseWithError to terminate sending the HTTP request Body
		// immediately, and hopefully, through terminating the sending which uses "Transfer-Encoding: chunked"" without sending
		// the terminating zero-length chunk, prevent the docker daemon from processing the tar stream at all.
		// Whether that works or not, closing the PipeWriter seems desirable in any case.
		if err := d.writer.CloseWithError(errors.New("Aborting upload, daemonImageDestination closed without a previous .Commit()")); err != nil {
			return err
		}
	}
	d.goroutineCancel()

	return nil
}

func (d *daemonImageDestination) Reference() types.ImageReference {
	return d.ref
}

// Commit marks the process of storing the image as successful and asks for the image to be persisted.
// WARNING: This does not have any transactional semantics:
// - Uploaded data MAY be visible to others before Commit() is called
// - Uploaded data MAY be removed or MAY remain around if Close() is called without Commit() (i.e. rollback is allowed but not guaranteed)
func (d *daemonImageDestination) Commit(ctx context.Context, unparsedToplevel types.UnparsedImage) error {
	logrus.Debugf("docker-daemon: Closing tar stream")
	if err := d.Destination.Commit(ctx); err != nil {
		return err
	}
	if err := d.writer.Close(); err != nil {
		return err
	}
	d.committed = true // We may still fail, but we are done sending to imageLoadGoroutine.

	logrus.Debugf("docker-daemon: Waiting for status")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-d.statusChannel:
		return err
	}
}
