// Copyright 2014 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"unicode/utf8"

	"github.com/golang/protobuf/proto"
	"golang.org/x/xerrors"
	"google.golang.org/api/googleapi"
	raw "google.golang.org/api/storage/v1"
	storagepb "google.golang.org/genproto/googleapis/storage/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// Maximum amount of content that can be sent per WriteObjectRequest message.
	// A buffer reaching this amount will precipitate a flush of the buffer.
	//
	// This is only used for the gRPC-based Writer.
	maxPerMessageWriteSize int = int(storagepb.ServiceConstants_MAX_WRITE_CHUNK_BYTES)
)

// A Writer writes a Cloud Storage object.
type Writer struct {
	// ObjectAttrs are optional attributes to set on the object. Any attributes
	// must be initialized before the first Write call. Nil or zero-valued
	// attributes are ignored.
	ObjectAttrs

	// SendCRC specifies whether to transmit a CRC32C field. It should be set
	// to true in addition to setting the Writer's CRC32C field, because zero
	// is a valid CRC and normally a zero would not be transmitted.
	// If a CRC32C is sent, and the data written does not match the checksum,
	// the write will be rejected.
	SendCRC32C bool

	// ChunkSize controls the maximum number of bytes of the object that the
	// Writer will attempt to send to the server in a single request. Objects
	// smaller than the size will be sent in a single request, while larger
	// objects will be split over multiple requests. The size will be rounded up
	// to the nearest multiple of 256K.
	//
	// ChunkSize will default to a reasonable value. If you perform many
	// concurrent writes of small objects (under ~8MB), you may wish set ChunkSize
	// to a value that matches your objects' sizes to avoid consuming large
	// amounts of memory. See
	// https://cloud.google.com/storage/docs/json_api/v1/how-tos/upload#size
	// for more information about performance trade-offs related to ChunkSize.
	//
	// If ChunkSize is set to zero, chunking will be disabled and the object will
	// be uploaded in a single request without the use of a buffer. This will
	// further reduce memory used during uploads, but will also prevent the writer
	// from retrying in case of a transient error from the server, since a buffer
	// is required in order to retry the failed request.
	//
	// ChunkSize must be set before the first Write call.
	ChunkSize int

	// ProgressFunc can be used to monitor the progress of a large write.
	// operation. If ProgressFunc is not nil and writing requires multiple
	// calls to the underlying service (see
	// https://cloud.google.com/storage/docs/json_api/v1/how-tos/resumable-upload),
	// then ProgressFunc will be invoked after each call with the number of bytes of
	// content copied so far.
	//
	// ProgressFunc should return quickly without blocking.
	ProgressFunc func(int64)

	ctx context.Context
	o   *ObjectHandle

	opened bool
	pw     *io.PipeWriter

	donec chan struct{} // closed after err and obj are set.
	obj   *ObjectAttrs

	mu  sync.Mutex
	err error

	// The gRPC client-stream used for sending buffers.
	//
	// This is an experimental API and not intended for public use.
	stream storagepb.Storage_WriteObjectClient

	// The Resumable Upload ID started by a gRPC-based Writer.
	//
	// This is an experimental API and not intended for public use.
	upid string
}

func (w *Writer) open() error {
	if err := w.validateWriteAttrs(); err != nil {
		return err
	}

	pr, pw := io.Pipe()
	w.pw = pw
	w.opened = true

	go w.monitorCancel()

	attrs := w.ObjectAttrs
	mediaOpts := []googleapi.MediaOption{
		googleapi.ChunkSize(w.ChunkSize),
	}
	if c := attrs.ContentType; c != "" {
		mediaOpts = append(mediaOpts, googleapi.ContentType(c))
	}

	go func() {
		defer close(w.donec)

		rawObj := attrs.toRawObject(w.o.bucket)
		if w.SendCRC32C {
			rawObj.Crc32c = encodeUint32(attrs.CRC32C)
		}
		if w.MD5 != nil {
			rawObj.Md5Hash = base64.StdEncoding.EncodeToString(w.MD5)
		}
		call := w.o.c.raw.Objects.Insert(w.o.bucket, rawObj).
			Media(pr, mediaOpts...).
			Projection("full").
			Context(w.ctx).
			Name(w.o.object)

		if w.ProgressFunc != nil {
			call.ProgressUpdater(func(n, _ int64) { w.ProgressFunc(n) })
		}
		if attrs.KMSKeyName != "" {
			call.KmsKeyName(attrs.KMSKeyName)
		}
		if attrs.PredefinedACL != "" {
			call.PredefinedAcl(attrs.PredefinedACL)
		}
		if err := setEncryptionHeaders(call.Header(), w.o.encryptionKey, false); err != nil {
			w.mu.Lock()
			w.err = err
			w.mu.Unlock()
			pr.CloseWithError(err)
			return
		}
		var resp *raw.Object
		err := applyConds("NewWriter", w.o.gen, w.o.conds, call)
		if err == nil {
			if w.o.userProject != "" {
				call.UserProject(w.o.userProject)
			}
			setClientHeader(call.Header())

			// The internals that perform call.Do automatically retry both the initial
			// call to set up the upload as well as calls to upload individual chunks
			// for a resumable upload (as long as the chunk size is non-zero). Hence
			// there is no need to add retries here.
			resp, err = call.Do()
		}
		if err != nil {
			w.mu.Lock()
			w.err = err
			w.mu.Unlock()
			pr.CloseWithError(err)
			return
		}
		w.obj = newObject(resp)
	}()
	return nil
}

// Write appends to w. It implements the io.Writer interface.
//
// Since writes happen asynchronously, Write may return a nil
// error even though the write failed (or will fail). Always
// use the error returned from Writer.Close to determine if
// the upload was successful.
//
// Writes will be retried on transient errors from the server, unless
// Writer.ChunkSize has been set to zero.
func (w *Writer) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	werr := w.err
	w.mu.Unlock()
	if werr != nil {
		return 0, werr
	}
	if !w.opened {
		// gRPC client has been initialized - use gRPC to upload.
		if w.o.c.gc != nil {
			if err := w.openGRPC(); err != nil {
				return 0, err
			}
		} else if err := w.open(); err != nil {
			return 0, err
		}
	}
	n, err = w.pw.Write(p)
	if err != nil {
		w.mu.Lock()
		werr := w.err
		w.mu.Unlock()
		// Preserve existing functionality that when context is canceled, Write will return
		// context.Canceled instead of "io: read/write on closed pipe". This hides the
		// pipe implementation detail from users and makes Write seem as though it's an RPC.
		if xerrors.Is(werr, context.Canceled) || xerrors.Is(werr, context.DeadlineExceeded) {
			return n, werr
		}
	}
	return n, err
}

// Close completes the write operation and flushes any buffered data.
// If Close doesn't return an error, metadata about the written object
// can be retrieved by calling Attrs.
func (w *Writer) Close() error {
	if !w.opened {
		if err := w.open(); err != nil {
			return err
		}
	}

	// Closing either the read or write causes the entire pipe to close.
	if err := w.pw.Close(); err != nil {
		return err
	}

	<-w.donec
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

// monitorCancel is intended to be used as a background goroutine. It monitors the
// context, and when it observes that the context has been canceled, it manually
// closes things that do not take a context.
func (w *Writer) monitorCancel() {
	select {
	case <-w.ctx.Done():
		w.mu.Lock()
		werr := w.ctx.Err()
		w.err = werr
		w.mu.Unlock()

		// Closing either the read or write causes the entire pipe to close.
		w.CloseWithError(werr)
	case <-w.donec:
	}
}

// CloseWithError aborts the write operation with the provided error.
// CloseWithError always returns nil.
//
// Deprecated: cancel the context passed to NewWriter instead.
func (w *Writer) CloseWithError(err error) error {
	if !w.opened {
		return nil
	}
	return w.pw.CloseWithError(err)
}

// Attrs returns metadata about a successfully-written object.
// It's only valid to call it after Close returns nil.
func (w *Writer) Attrs() *ObjectAttrs {
	return w.obj
}

func (w *Writer) validateWriteAttrs() error {
	attrs := w.ObjectAttrs
	// Check the developer didn't change the object Name (this is unfortunate, but
	// we don't want to store an object under the wrong name).
	if attrs.Name != w.o.object {
		return fmt.Errorf("storage: Writer.Name %q does not match object name %q", attrs.Name, w.o.object)
	}
	if !utf8.ValidString(attrs.Name) {
		return fmt.Errorf("storage: object name %q is not valid UTF-8", attrs.Name)
	}
	if attrs.KMSKeyName != "" && w.o.encryptionKey != nil {
		return errors.New("storage: cannot use KMSKeyName with a customer-supplied encryption key")
	}
	if w.ChunkSize < 0 {
		return errors.New("storage: Writer.ChunkSize must be non-negative")
	}
	return nil
}

// progress is a convenience wrapper that reports write progress to the Writer
// ProgressFunc if it is set and progress is non-zero.
func (w *Writer) progress(p int64) {
	if w.ProgressFunc != nil && p != 0 {
		w.ProgressFunc(p)
	}
}

// error acquires the Writer's lock, sets the Writer's err to the given error,
// then relinquishes the lock.
func (w *Writer) error(err error) {
	w.mu.Lock()
	w.err = err
	w.mu.Unlock()
}

// openGRPC initializes a pipe for the user to write data to, and a routine to
// read from that pipe and upload the data to GCS via gRPC.
//
// This is an experimental API and not intended for public use.
func (w *Writer) openGRPC() error {
	if err := w.validateWriteAttrs(); err != nil {
		return err
	}

	pr, pw := io.Pipe()
	w.pw = pw
	w.opened = true

	go w.monitorCancel()

	bufSize := w.ChunkSize
	if w.ChunkSize == 0 {
		// TODO: Should we actually use the minimum of 256 KB here when the user
		// indicates they want minimal memory usage? We cannot do a zero-copy,
		// bufferless upload like HTTP/JSON can.
		// TODO: We need to determine if we can avoid starting a
		// resumable upload when the user *plans* to send more than bufSize but
		// with a bufferless upload.
		bufSize = maxPerMessageWriteSize
	}
	buf := make([]byte, bufSize)

	var offset int64

	// This function reads the data sent to the pipe and sends sets of messages
	// on the gRPC client-stream as the buffer is filled.
	go func() {
		defer close(w.donec)

		// Loop until there is an error or the Object has been finalized.
		for {
			// Note: This blocks until either the buffer is full or EOF is read.
			recvd, doneReading, err := read(pr, buf)
			if err != nil {
				err = checkCanceled(err)
				w.error(err)
				pr.CloseWithError(err)
				return
			}
			toWrite := buf[:recvd]

			// TODO: Figure out how to set up encryption via CommonObjectRequestParams.

			// The chunk buffer is full, but there is no end in sight. This
			// means that a resumable upload will need to be used to send
			// multiple chunks, until we are done reading data. Start a
			// resumable upload if it has not already been started.
			// Otherwise, all data will be sent over a single gRPC stream.
			if !doneReading && w.upid == "" {
				err = w.startResumableUpload()
				if err != nil {
					err = checkCanceled(err)
					w.error(err)
					pr.CloseWithError(err)
					return
				}
			}

			o, off, finalized, err := w.uploadBuffer(toWrite, recvd, offset, doneReading)
			if err != nil {
				err = checkCanceled(err)
				w.error(err)
				pr.CloseWithError(err)
				return
			}
			// At this point, the current buffer has been uploaded. Capture the
			// committed offset here in case the upload was not finalized and
			// another chunk is to be uploaded.
			offset = off
			w.progress(offset)

			// When we are done reading data and the chunk has been finalized,
			// we are done.
			if doneReading && finalized {
				// Build Object from server's response.
				w.obj = newObjectFromProto(o)
				return
			}
		}
	}()

	return nil
}

// startResumableUpload initializes a Resumable Upload with gRPC and sets the
// upload ID on the Writer.
//
// This is an experimental API and not intended for public use.
func (w *Writer) startResumableUpload() error {
	var common *storagepb.CommonRequestParams
	if w.o.userProject != "" {
		common = &storagepb.CommonRequestParams{UserProject: w.o.userProject}
	}
	spec, err := w.writeObjectSpec()
	if err != nil {
		return err
	}
	upres, err := w.o.c.gc.StartResumableWrite(w.ctx, &storagepb.StartResumableWriteRequest{
		WriteObjectSpec:     spec,
		CommonRequestParams: common,
	})

	w.upid = upres.GetUploadId()
	return err
}

// queryProgress is a helper that queries the status of the resumable upload
// associated with the given upload ID.
//
// This is an experimental API and not intended for public use.
func (w *Writer) queryProgress() (int64, error) {
	q, err := w.o.c.gc.QueryWriteStatus(w.ctx, &storagepb.QueryWriteStatusRequest{UploadId: w.upid})

	// q.GetCommittedSize() will return 0 if q is nil.
	return q.GetPersistedSize(), err
}

// uploadBuffer opens a Write stream and uploads the buffer at the given offset (if
// uploading a chunk for a resumable uploadBuffer), and will mark the write as
// finished if we are done receiving data from the user. The resulting write
// offset after uploading the buffer is returned, as well as a boolean
// indicating if the Object has been finalized. If it has been finalized, the
// final Object will be returned as well. Finalizing the upload is primarily
// important for Resumable Uploads. A simple or multi-part upload will always
// be finalized once the entire buffer has been written.
//
// This is an experimental API and not intended for public use.
func (w *Writer) uploadBuffer(buf []byte, recvd int, start int64, doneReading bool) (*storagepb.Object, int64, bool, error) {
	var err error
	var finishWrite bool
	var sent, limit int = 0, maxPerMessageWriteSize
	offset := start
	for {
		first := sent == 0
		// This indicates that this is the last message and the remaining
		// data fits in one message.
		belowLimit := recvd-sent <= limit
		if belowLimit {
			limit = recvd - sent
		}
		if belowLimit && doneReading {
			finishWrite = true
		}

		// Prepare chunk section for upload.
		data := buf[sent : sent+limit]
		req := &storagepb.WriteObjectRequest{
			Data: &storagepb.WriteObjectRequest_ChecksummedData{
				ChecksummedData: &storagepb.ChecksummedData{
					Content: data,
				},
			},
			WriteOffset: offset,
			FinishWrite: finishWrite,
		}

		// Open a new stream and set the first_message field on the request.
		// The first message on the WriteObject stream must either be the
		// Object or the Resumable Upload ID.
		if first {
			w.stream, err = w.o.c.gc.WriteObject(w.ctx)
			if err != nil {
				return nil, 0, false, err
			}

			if w.upid != "" {
				req.FirstMessage = &storagepb.WriteObjectRequest_UploadId{UploadId: w.upid}
			} else {
				spec, err := w.writeObjectSpec()
				if err != nil {
					return nil, 0, false, err
				}
				req.FirstMessage = &storagepb.WriteObjectRequest_WriteObjectSpec{
					WriteObjectSpec: spec,
				}
			}

			// TODO: Currently the checksums are only sent on the first message
			// of the stream, but in the future, we must also support sending it
			// on the *last* message of the stream (instead of the first).
			if w.SendCRC32C {
				req.ObjectChecksums = &storagepb.ObjectChecksums{
					Crc32C:  proto.Uint32(w.CRC32C),
					Md5Hash: w.MD5,
				}
			}
		}

		err = w.stream.Send(req)
		if err == io.EOF {
			// err was io.EOF. The client-side of a stream only gets an EOF on Send
			// when the backend closes the stream and wants to return an error
			// status. Closing the stream receives the status as an error.
			_, err = w.stream.CloseAndRecv()

			// Retriable errors mean we should start over and attempt to
			// resend the entire buffer via a new stream.
			// If not retriable, falling through will return the error received
			// from closing the stream.
			if shouldRetry(err) {
				sent = 0
				finishWrite = false
				// TODO: Add test case for failure modes of querying progress.
				offset, err = w.determineOffset(start)
				if err == nil {
					continue
				}
			}
		}
		if err != nil {
			return nil, 0, false, err
		}

		// Update the immediate stream's sent total and the upload offset with
		// the data sent.
		sent += len(data)
		offset += int64(len(data))

		// Not done sending data, do not attempt to commit it yet, loop around
		// and send more data.
		if recvd-sent > 0 {
			continue
		}

		// Done sending data. Close the stream to "commit" the data sent.
		resp, finalized, err := w.commit()
		// Retriable errors mean we should start over and attempt to
		// resend the entire buffer via a new stream.
		// If not retriable, falling through will return the error received
		// from closing the stream.
		if shouldRetry(err) {
			sent = 0
			finishWrite = false
			offset, err = w.determineOffset(start)
			if err == nil {
				continue
			}
		}
		if err != nil {
			return nil, 0, false, err
		}

		return resp.GetResource(), offset, finalized, nil
	}
}

// determineOffset either returns the offset given to it in the case of a simple
// upload, or queries the write status in the case a resumable upload is being
// used.
//
// This is an experimental API and not intended for public use.
func (w *Writer) determineOffset(offset int64) (int64, error) {
	// For a Resumable Upload, we must start from however much data
	// was committed.
	if w.upid != "" {
		committed, err := w.queryProgress()
		if err != nil {
			return 0, err
		}
		offset = committed
	}
	return offset, nil
}

// commit closes the stream to commit the data sent and potentially receive
// the finalized object if finished uploading. If the last request sent
// indicated that writing was finished, the Object will be finalized and
// returned. If not, then the Object will be nil, and the boolean returned will
// be false.
//
// This is an experimental API and not intended for public use.
func (w *Writer) commit() (*storagepb.WriteObjectResponse, bool, error) {
	finalized := true
	resp, err := w.stream.CloseAndRecv()
	if err == io.EOF {
		// Closing a stream for a resumable upload finish_write = false results
		// in an EOF which can be ignored, as we aren't done uploading yet.
		finalized = false
		err = nil
	}
	// Drop the stream reference as it has been closed.
	w.stream = nil

	return resp, finalized, err
}

// writeObjectSpec constructs a WriteObjectSpec proto using the Writer's
// ObjectAttrs and applies its Conditions. This is only used for gRPC.
//
// This is an experimental API and not intended for public use.
func (w *Writer) writeObjectSpec() (*storagepb.WriteObjectSpec, error) {
	spec := &storagepb.WriteObjectSpec{
		Resource: w.ObjectAttrs.toProtoObject(w.o.bucket),
	}
	// WriteObject doesn't support the generation condition, so use -1.
	if err := applyCondsProto("WriteObject", -1, w.o.conds, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

// read copies the data in the reader to the given buffer and reports how much
// data was read into the buffer and if there is no more data to read (EOF).
//
// This is an experimental API and not intended for public use.
func read(r io.Reader, buf []byte) (int, bool, error) {
	// Set n to -1 to start the Read loop.
	var n, recvd int = -1, 0
	var err error
	for err == nil && n != 0 {
		// The routine blocks here until data is received.
		n, err = r.Read(buf[recvd:])
		recvd += n
	}
	var done bool
	if err == io.EOF {
		done = true
		err = nil
	}
	return recvd, done, err
}

func checkCanceled(err error) error {
	if status.Code(err) == codes.Canceled {
		return context.Canceled
	}

	return err
}
