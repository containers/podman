// Copyright 2016 Google LLC
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
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/internal/trace"
	"google.golang.org/api/googleapi"
	storagepb "google.golang.org/genproto/googleapis/storage/v2"
)

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

// ReaderObjectAttrs are attributes about the object being read. These are populated
// during the New call. This struct only holds a subset of object attributes: to
// get the full set of attributes, use ObjectHandle.Attrs.
//
// Each field is read-only.
type ReaderObjectAttrs struct {
	// Size is the length of the object's content.
	Size int64

	// StartOffset is the byte offset within the object
	// from which reading begins.
	// This value is only non-zero for range requests.
	StartOffset int64

	// ContentType is the MIME type of the object's content.
	ContentType string

	// ContentEncoding is the encoding of the object's content.
	ContentEncoding string

	// CacheControl specifies whether and for how long browser and Internet
	// caches are allowed to cache your objects.
	CacheControl string

	// LastModified is the time that the object was last modified.
	LastModified time.Time

	// Generation is the generation number of the object's content.
	Generation int64

	// Metageneration is the version of the metadata for this object at
	// this generation. This field is used for preconditions and for
	// detecting changes in metadata. A metageneration number is only
	// meaningful in the context of a particular generation of a
	// particular object.
	Metageneration int64
}

// NewReader creates a new Reader to read the contents of the
// object.
// ErrObjectNotExist will be returned if the object is not found.
//
// The caller must call Close on the returned Reader when done reading.
func (o *ObjectHandle) NewReader(ctx context.Context) (*Reader, error) {
	return o.NewRangeReader(ctx, 0, -1)
}

// NewRangeReader reads part of an object, reading at most length bytes
// starting at the given offset. If length is negative, the object is read
// until the end. If offset is negative, the object is read abs(offset) bytes
// from the end, and length must also be negative to indicate all remaining
// bytes will be read.
//
// If the object's metadata property "Content-Encoding" is set to "gzip" or satisfies
// decompressive transcoding per https://cloud.google.com/storage/docs/transcoding
// that file will be served back whole, regardless of the requested range as
// Google Cloud Storage dictates.
func (o *ObjectHandle) NewRangeReader(ctx context.Context, offset, length int64) (r *Reader, err error) {
	ctx = trace.StartSpan(ctx, "cloud.google.com/go/storage.Object.NewRangeReader")
	defer func() { trace.EndSpan(ctx, err) }()

	if o.c.gc != nil {
		return o.newRangeReaderWithGRPC(ctx, offset, length)
	}

	if err := o.validate(); err != nil {
		return nil, err
	}
	if offset < 0 && length >= 0 {
		return nil, fmt.Errorf("storage: invalid offset %d < 0 requires negative length", offset)
	}
	if o.conds != nil {
		if err := o.conds.validate("NewRangeReader"); err != nil {
			return nil, err
		}
	}
	u := &url.URL{
		Scheme: o.c.scheme,
		Host:   o.c.readHost,
		Path:   fmt.Sprintf("/%s/%s", o.bucket, o.object),
	}
	verb := "GET"
	if length == 0 {
		verb = "HEAD"
	}
	req, err := http.NewRequest(verb, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if o.userProject != "" {
		req.Header.Set("X-Goog-User-Project", o.userProject)
	}
	if o.readCompressed {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	if err := setEncryptionHeaders(req.Header, o.encryptionKey, false); err != nil {
		return nil, err
	}

	gen := o.gen

	// Define a function that initiates a Read with offset and length, assuming we
	// have already read seen bytes.
	reopen := func(seen int64) (*http.Response, error) {
		// If the context has already expired, return immediately without making a
		// call.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		start := offset + seen
		if length < 0 && start < 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d", start))
		} else if length < 0 && start > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
		} else if length > 0 {
			// The end character isn't affected by how many bytes we've seen.
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, offset+length-1))
		}
		// We wait to assign conditions here because the generation number can change in between reopen() runs.
		if err := setConditionsHeaders(req.Header, o.conds); err != nil {
			return nil, err
		}
		// If an object generation is specified, include generation as query string parameters.
		if gen >= 0 {
			req.URL.RawQuery = fmt.Sprintf("generation=%d", gen)
		}

		var res *http.Response
		err = runWithRetry(ctx, func() error {
			res, err = o.c.hc.Do(req)
			if err != nil {
				return err
			}
			if res.StatusCode == http.StatusNotFound {
				res.Body.Close()
				return ErrObjectNotExist
			}
			if res.StatusCode < 200 || res.StatusCode > 299 {
				body, _ := ioutil.ReadAll(res.Body)
				res.Body.Close()
				return &googleapi.Error{
					Code:   res.StatusCode,
					Header: res.Header,
					Body:   string(body),
				}
			}

			partialContentNotSatisfied :=
				!decompressiveTranscoding(res) &&
					start > 0 && length != 0 &&
					res.StatusCode != http.StatusPartialContent

			if partialContentNotSatisfied {
				res.Body.Close()
				return errors.New("storage: partial request not satisfied")
			}

			// With "Content-Encoding": "gzip" aka decompressive transcoding, GCS serves
			// back the whole file regardless of the range count passed in as per:
			//      https://cloud.google.com/storage/docs/transcoding#range,
			// thus we have to manually move the body forward by seen bytes.
			if decompressiveTranscoding(res) && seen > 0 {
				_, _ = io.CopyN(ioutil.Discard, res.Body, seen)
			}

			// If a generation hasn't been specified, and this is the first response we get, let's record the
			// generation. In future requests we'll use this generation as a precondition to avoid data races.
			if gen < 0 && res.Header.Get("X-Goog-Generation") != "" {
				gen64, err := strconv.ParseInt(res.Header.Get("X-Goog-Generation"), 10, 64)
				if err != nil {
					return err
				}
				gen = gen64
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		return res, nil
	}

	res, err := reopen(0)
	if err != nil {
		return nil, err
	}
	var (
		size        int64 // total size of object, even if a range was requested.
		checkCRC    bool
		crc         uint32
		startOffset int64 // non-zero if range request.
	)
	if res.StatusCode == http.StatusPartialContent {
		cr := strings.TrimSpace(res.Header.Get("Content-Range"))
		if !strings.HasPrefix(cr, "bytes ") || !strings.Contains(cr, "/") {
			return nil, fmt.Errorf("storage: invalid Content-Range %q", cr)
		}
		// Content range is formatted <first byte>-<last byte>/<total size>. We take
		// the total size.
		size, err = strconv.ParseInt(cr[strings.LastIndex(cr, "/")+1:], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("storage: invalid Content-Range %q", cr)
		}

		dashIndex := strings.Index(cr, "-")
		if dashIndex >= 0 {
			startOffset, err = strconv.ParseInt(cr[len("bytes="):dashIndex], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("storage: invalid Content-Range %q: %v", cr, err)
			}
		}
	} else {
		size = res.ContentLength
		// Check the CRC iff all of the following hold:
		// - We asked for content (length != 0).
		// - We got all the content (status != PartialContent).
		// - The server sent a CRC header.
		// - The Go http stack did not uncompress the file.
		// - We were not served compressed data that was uncompressed on download.
		// The problem with the last two cases is that the CRC will not match -- GCS
		// computes it on the compressed contents, but we compute it on the
		// uncompressed contents.
		if length != 0 && !res.Uncompressed && !uncompressedByServer(res) {
			crc, checkCRC = parseCRC32c(res)
		}
	}

	remain := res.ContentLength
	body := res.Body
	if length == 0 {
		remain = 0
		body.Close()
		body = emptyBody
	}
	var metaGen int64
	if res.Header.Get("X-Goog-Metageneration") != "" {
		metaGen, err = strconv.ParseInt(res.Header.Get("X-Goog-Metageneration"), 10, 64)
		if err != nil {
			return nil, err
		}
	}

	var lm time.Time
	if res.Header.Get("Last-Modified") != "" {
		lm, err = http.ParseTime(res.Header.Get("Last-Modified"))
		if err != nil {
			return nil, err
		}
	}

	attrs := ReaderObjectAttrs{
		Size:            size,
		ContentType:     res.Header.Get("Content-Type"),
		ContentEncoding: res.Header.Get("Content-Encoding"),
		CacheControl:    res.Header.Get("Cache-Control"),
		LastModified:    lm,
		StartOffset:     startOffset,
		Generation:      gen,
		Metageneration:  metaGen,
	}
	return &Reader{
		Attrs:    attrs,
		body:     body,
		size:     size,
		remain:   remain,
		wantCRC:  crc,
		checkCRC: checkCRC,
		reopen:   reopen,
	}, nil
}

// decompressiveTranscoding returns true if the request was served decompressed
// and different than its original storage form. This happens when the "Content-Encoding"
// header is "gzip".
// See:
//  * https://cloud.google.com/storage/docs/transcoding#transcoding_and_gzip
//  * https://github.com/googleapis/google-cloud-go/issues/1800
func decompressiveTranscoding(res *http.Response) bool {
	// Decompressive Transcoding.
	return res.Header.Get("Content-Encoding") == "gzip" ||
		res.Header.Get("X-Goog-Stored-Content-Encoding") == "gzip"
}

func uncompressedByServer(res *http.Response) bool {
	// If the data is stored as gzip but is not encoded as gzip, then it
	// was uncompressed by the server.
	return res.Header.Get("X-Goog-Stored-Content-Encoding") == "gzip" &&
		res.Header.Get("Content-Encoding") != "gzip"
}

func parseCRC32c(res *http.Response) (uint32, bool) {
	const prefix = "crc32c="
	for _, spec := range res.Header["X-Goog-Hash"] {
		if strings.HasPrefix(spec, prefix) {
			c, err := decodeUint32(spec[len(prefix):])
			if err == nil {
				return c, true
			}
		}
	}
	return 0, false
}

// setConditionsHeaders sets precondition request headers for downloads
// using the XML API. It assumes that the conditions have been validated.
func setConditionsHeaders(headers http.Header, conds *Conditions) error {
	if conds == nil {
		return nil
	}
	if conds.MetagenerationMatch != 0 {
		headers.Set("x-goog-if-metageneration-match", fmt.Sprint(conds.MetagenerationMatch))
	}
	switch {
	case conds.GenerationMatch != 0:
		headers.Set("x-goog-if-generation-match", fmt.Sprint(conds.GenerationMatch))
	case conds.DoesNotExist:
		headers.Set("x-goog-if-generation-match", "0")
	}
	return nil
}

var emptyBody = ioutil.NopCloser(strings.NewReader(""))

// Reader reads a Cloud Storage object.
// It implements io.Reader.
//
// Typically, a Reader computes the CRC of the downloaded content and compares it to
// the stored CRC, returning an error from Read if there is a mismatch. This integrity check
// is skipped if transcoding occurs. See https://cloud.google.com/storage/docs/transcoding.
type Reader struct {
	Attrs              ReaderObjectAttrs
	body               io.ReadCloser
	seen, remain, size int64
	checkCRC           bool   // should we check the CRC?
	wantCRC            uint32 // the CRC32c value the server sent in the header
	gotCRC             uint32 // running crc
	reopen             func(seen int64) (*http.Response, error)

	// The following fields are only for use in the gRPC hybrid client.
	stream         storagepb.Storage_ReadObjectClient
	reopenWithGRPC func(seen int64) (*readStreamResponse, context.CancelFunc, error)
	leftovers      []byte
	cancelStream   context.CancelFunc
}

type readStreamResponse struct {
	stream   storagepb.Storage_ReadObjectClient
	response *storagepb.ReadObjectResponse
}

// Close closes the Reader. It must be called when done reading.
func (r *Reader) Close() error {
	if r.body != nil {
		return r.body.Close()
	}

	r.closeStream()
	return nil
}

func (r *Reader) Read(p []byte) (int, error) {
	read := r.readWithRetry
	if r.reopenWithGRPC != nil {
		read = r.readWithGRPC
	}

	n, err := read(p)
	if r.remain != -1 {
		r.remain -= int64(n)
	}
	if r.checkCRC {
		r.gotCRC = crc32.Update(r.gotCRC, crc32cTable, p[:n])
		// Check CRC here. It would be natural to check it in Close, but
		// everybody defers Close on the assumption that it doesn't return
		// anything worth looking at.
		if err == io.EOF {
			if r.gotCRC != r.wantCRC {
				return n, fmt.Errorf("storage: bad CRC on read: got %d, want %d",
					r.gotCRC, r.wantCRC)
			}
		}
	}
	return n, err
}

// newRangeReaderWithGRPC creates a new Reader with the given range that uses
// gRPC to read Object content.
//
// This is an experimental API and not intended for public use.
func (o *ObjectHandle) newRangeReaderWithGRPC(ctx context.Context, offset, length int64) (r *Reader, err error) {
	ctx = trace.StartSpan(ctx, "cloud.google.com/go/storage.Object.newRangeReaderWithGRPC")
	defer func() { trace.EndSpan(ctx, err) }()

	if o.c.gc == nil {
		err = fmt.Errorf("handle doesn't have a gRPC client initialized")
		return
	}
	if err = o.validate(); err != nil {
		return
	}

	// A negative length means "read to the end of the object", but the
	// read_limit field it corresponds to uses zero to mean the same thing. Thus
	// we coerce the length to 0 to read to the end of the object.
	if length < 0 {
		length = 0
	}

	// For now, there are only globally unique buckets, and "_" is the alias
	// project ID for such buckets.
	b := bucketResourceName("_", o.bucket)
	req := &storagepb.ReadObjectRequest{
		Bucket: b,
		Object: o.object,
	}
	// The default is a negative value, which means latest.
	if o.gen >= 0 {
		req.Generation = o.gen
	}

	// Define a function that initiates a Read with offset and length, assuming
	// we have already read seen bytes.
	reopen := func(seen int64) (*readStreamResponse, context.CancelFunc, error) {
		// If the context has already expired, return immediately without making
		// we call.
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}

		cc, cancel := context.WithCancel(ctx)

		start := offset + seen
		// Only set a ReadLimit if length is greater than zero, because zero
		// means read it all.
		if length > 0 {
			req.ReadLimit = length - seen
		}
		req.ReadOffset = start

		if err := applyCondsProto("reopenWithGRPC", o.gen, o.conds, req); err != nil {
			cancel()
			return nil, nil, err
		}

		var stream storagepb.Storage_ReadObjectClient
		var msg *storagepb.ReadObjectResponse
		var err error

		err = runWithRetry(cc, func() error {
			stream, err = o.c.gc.ReadObject(cc, req)
			if err != nil {
				return err
			}

			msg, err = stream.Recv()

			return err
		})
		if err != nil {
			// Close the stream context we just created to ensure we don't leak
			// resources.
			cancel()
			return nil, nil, err
		}

		return &readStreamResponse{stream, msg}, cancel, nil
	}

	res, cancel, err := reopen(0)
	if err != nil {
		return nil, err
	}

	r = &Reader{
		stream:         res.stream,
		reopenWithGRPC: reopen,
		cancelStream:   cancel,
	}

	// The first message was Recv'd on stream open, use it to populate the
	// object metadata.
	msg := res.response
	obj := msg.GetMetadata()
	// This is the size of the entire object, even if only a range was requested.
	size := obj.GetSize()

	r.Attrs = ReaderObjectAttrs{
		Size:            size,
		ContentType:     obj.GetContentType(),
		ContentEncoding: obj.GetContentEncoding(),
		CacheControl:    obj.GetCacheControl(),
		LastModified:    obj.GetUpdateTime().AsTime(),
		Metageneration:  obj.GetMetageneration(),
		Generation:      obj.GetGeneration(),
	}

	r.size = size
	cr := msg.GetContentRange()
	if cr != nil {
		r.Attrs.StartOffset = cr.GetStart()
		r.remain = cr.GetEnd() - cr.GetStart() + 1
	} else {
		r.remain = size
	}

	// Only support checksums when reading an entire object, not a range.
	if msg.GetObjectChecksums().Crc32C != nil && offset == 0 && length == 0 {
		r.wantCRC = msg.GetObjectChecksums().GetCrc32C()
		r.checkCRC = true
	}

	// Store the content from the first Recv in the client buffer for reading
	// later.
	r.leftovers = msg.GetChecksummedData().GetContent()

	return r, nil
}

func (r *Reader) readWithRetry(p []byte) (int, error) {
	n := 0
	for len(p[n:]) > 0 {
		m, err := r.body.Read(p[n:])
		n += m
		r.seen += int64(m)
		if err == nil || err == io.EOF {
			return n, err
		}
		// Read failed (likely due to connection issues), but we will try to reopen
		// the pipe and continue. Send a ranged read request that takes into account
		// the number of bytes we've already seen.
		res, err := r.reopen(r.seen)
		if err != nil {
			// reopen already retries
			return n, err
		}
		r.body.Close()
		r.body = res.Body
	}
	return n, nil
}

// closeStream cancels a stream's context in order for it to be closed and
// collected.
//
// This is an experimental API and not intended for public use.
func (r *Reader) closeStream() {
	if r.cancelStream != nil {
		r.cancelStream()
	}
	r.stream = nil
}

// readWithGRPC reads bytes into the user's buffer from an open gRPC stream.
//
// This is an experimental API and not intended for public use.
func (r *Reader) readWithGRPC(p []byte) (int, error) {
	// No stream to read from, either never initiliazed or Close was called.
	// Note: There is a potential concurrency issue if multiple routines are
	// using the same reader. One encounters an error and the stream is closed
	// and then reopened while the other routine attempts to read from it.
	if r.stream == nil {
		return 0, fmt.Errorf("reader has been closed")
	}

	// The entire object has been read by this reader, return EOF.
	if r.size != 0 && r.size == r.seen {
		return 0, io.EOF
	}

	var n int
	// Read leftovers and return what was available to conform to the Reader
	// interface: https://pkg.go.dev/io#Reader.
	if len(r.leftovers) > 0 {
		n = copy(p, r.leftovers)
		r.seen += int64(n)
		r.leftovers = r.leftovers[n:]
		return n, nil
	}

	// Attempt to Recv the next message on the stream.
	msg, err := r.recv()
	if err != nil {
		return 0, err
	}

	// TODO: Determine if we need to capture incremental CRC32C for this
	// chunk. The Object CRC32C checksum is captured when directed to read
	// the entire Object. If directed to read a range, we may need to
	// calculate the range's checksum for verification if the checksum is
	// present in the response here.
	// TODO: Figure out if we need to support decompressive transcoding
	// https://cloud.google.com/storage/docs/transcoding.
	content := msg.GetChecksummedData().GetContent()
	n = copy(p[n:], content)
	leftover := len(content) - n
	if leftover > 0 {
		// Wasn't able to copy all of the data in the message, store for
		// future Read calls.
		r.leftovers = content[n:]
	}
	r.seen += int64(n)

	return n, nil
}

// recv attempts to Recv the next message on the stream. In the event
// that a retryable error is encountered, the stream will be closed, reopened,
// and Recv again. This will attempt to Recv until one of the following is true:
//
// * Recv is successful
// * A non-retryable error is encountered
// * The Reader's context is canceled
//
// The last error received is the one that is returned, which could be from
// an attempt to reopen the stream.
//
// This is an experimental API and not intended for public use.
func (r *Reader) recv() (*storagepb.ReadObjectResponse, error) {
	msg, err := r.stream.Recv()
	if err != nil && shouldRetry(err) {
		// This will "close" the existing stream and immediately attempt to
		// reopen the stream, but will backoff if further attempts are necessary.
		// Reopening the stream Recvs the first message, so if retrying is
		// successful, the next logical chunk will be returned.
		msg, err = r.reopenStream(r.seen)
	}

	return msg, err
}

// reopenStream "closes" the existing stream and attempts to reopen a stream and
// sets the Reader's stream and cancelStream properties in the process.
//
// This is an experimental API and not intended for public use.
func (r *Reader) reopenStream(seen int64) (*storagepb.ReadObjectResponse, error) {
	// Close existing stream and initialize new stream with updated offset.
	r.closeStream()

	res, cancel, err := r.reopenWithGRPC(r.seen)
	if err != nil {
		return nil, err
	}
	r.stream = res.stream
	r.cancelStream = cancel
	return res.response, nil
}

// Size returns the size of the object in bytes.
// The returned value is always the same and is not affected by
// calls to Read or Close.
//
// Deprecated: use Reader.Attrs.Size.
func (r *Reader) Size() int64 {
	return r.Attrs.Size
}

// Remain returns the number of bytes left to read, or -1 if unknown.
func (r *Reader) Remain() int64 {
	return r.remain
}

// ContentType returns the content type of the object.
//
// Deprecated: use Reader.Attrs.ContentType.
func (r *Reader) ContentType() string {
	return r.Attrs.ContentType
}

// ContentEncoding returns the content encoding of the object.
//
// Deprecated: use Reader.Attrs.ContentEncoding.
func (r *Reader) ContentEncoding() string {
	return r.Attrs.ContentEncoding
}

// CacheControl returns the cache control of the object.
//
// Deprecated: use Reader.Attrs.CacheControl.
func (r *Reader) CacheControl() string {
	return r.Attrs.CacheControl
}

// LastModified returns the value of the Last-Modified header.
//
// Deprecated: use Reader.Attrs.LastModified.
func (r *Reader) LastModified() (time.Time, error) {
	return r.Attrs.LastModified, nil
}
