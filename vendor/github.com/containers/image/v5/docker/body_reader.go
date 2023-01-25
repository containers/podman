package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// bodyReaderMinimumProgress is the minimum progress we want to see before we retry
const bodyReaderMinimumProgress = 1 * 1024 * 1024

// bodyReader is an io.ReadCloser returned by dockerImageSource.GetBlob,
// which can transparently resume some (very limited) kinds of aborted connections.
type bodyReader struct {
	ctx context.Context
	c   *dockerClient

	path                string        // path to pass to makeRequest to retry
	logURL              *url.URL      // a string to use in error messages
	body                io.ReadCloser // The currently open connection we use to read data, or nil if there is nothing to read from / close.
	lastRetryOffset     int64
	offset              int64 // Current offset within the blob
	firstConnectionTime time.Time
	lastSuccessTime     time.Time // time.Time{} if N/A
}

// newBodyReader creates a bodyReader for request path in c.
// firstBody is an already correctly opened body for the blob, returing the full blob from the start.
// If reading from firstBody fails, bodyReader may heuristically decide to resume.
func newBodyReader(ctx context.Context, c *dockerClient, path string, firstBody io.ReadCloser) (io.ReadCloser, error) {
	logURL, err := c.resolveRequestURL(path)
	if err != nil {
		return nil, err
	}
	res := &bodyReader{
		ctx: ctx,
		c:   c,

		path:                path,
		logURL:              logURL,
		body:                firstBody,
		lastRetryOffset:     0,
		offset:              0,
		firstConnectionTime: time.Now(),
	}
	return res, nil
}

// parseDecimalInString ensures that s[start:] starts with a non-negative decimal number, and returns that number and the offset after the number.
func parseDecimalInString(s string, start int) (int64, int, error) {
	i := start
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return -1, -1, errors.New("missing decimal number")
	}
	v, err := strconv.ParseInt(s[start:i], 10, 64)
	if err != nil {
		return -1, -1, fmt.Errorf("parsing number: %w", err)
	}
	return v, i, nil
}

// parseExpectedChar ensures that s[pos] is the expected byte, and returns the offset after it.
func parseExpectedChar(s string, pos int, expected byte) (int, error) {
	if pos == len(s) || s[pos] != expected {
		return -1, fmt.Errorf("missing expected %q", expected)
	}
	return pos + 1, nil
}

// parseContentRange ensures that res contains a Content-Range header with a byte range, and returns (first, last, completeLength) on success. Size can be -1.
func parseContentRange(res *http.Response) (int64, int64, int64, error) {
	hdrs := res.Header.Values("Content-Range")
	switch len(hdrs) {
	case 0:
		return -1, -1, -1, errors.New("missing Content-Range: header")
	case 1:
		break
	default:
		return -1, -1, -1, fmt.Errorf("ambiguous Content-Range:, %d header values", len(hdrs))
	}
	hdr := hdrs[0]
	expectedPrefix := "bytes "
	if !strings.HasPrefix(hdr, expectedPrefix) {
		return -1, -1, -1, fmt.Errorf("invalid Content-Range: %q, missing prefix %q", hdr, expectedPrefix)
	}
	first, pos, err := parseDecimalInString(hdr, len(expectedPrefix))
	if err != nil {
		return -1, -1, -1, fmt.Errorf("invalid Content-Range: %q, parsing first-pos: %w", hdr, err)
	}
	pos, err = parseExpectedChar(hdr, pos, '-')
	if err != nil {
		return -1, -1, -1, fmt.Errorf("invalid Content-Range: %q: %w", hdr, err)
	}
	last, pos, err := parseDecimalInString(hdr, pos)
	if err != nil {
		return -1, -1, -1, fmt.Errorf("invalid Content-Range: %q, parsing last-pos: %w", hdr, err)
	}
	pos, err = parseExpectedChar(hdr, pos, '/')
	if err != nil {
		return -1, -1, -1, fmt.Errorf("invalid Content-Range: %q: %w", hdr, err)
	}
	completeLength := int64(-1)
	if pos < len(hdr) && hdr[pos] == '*' {
		pos++
	} else {
		completeLength, pos, err = parseDecimalInString(hdr, pos)
		if err != nil {
			return -1, -1, -1, fmt.Errorf("invalid Content-Range: %q, parsing complete-length: %w", hdr, err)
		}
	}
	if pos < len(hdr) {
		return -1, -1, -1, fmt.Errorf("invalid Content-Range: %q, unexpected trailing content", hdr)
	}
	return first, last, completeLength, nil
}

// Read implements io.ReadCloser
func (br *bodyReader) Read(p []byte) (int, error) {
	if br.body == nil {
		return 0, fmt.Errorf("internal error: bodyReader.Read called on a closed object for %s", br.logURL.Redacted())
	}
	n, err := br.body.Read(p)
	br.offset += int64(n)
	switch {
	case err == nil || err == io.EOF:
		br.lastSuccessTime = time.Now()
		return n, err // Unlike the default: case, don’t log anything.

	case errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET):
		originalErr := err
		redactedURL := br.logURL.Redacted()
		if err := br.errorIfNotReconnecting(originalErr, redactedURL); err != nil {
			return n, err
		}

		if err := br.body.Close(); err != nil {
			logrus.Debugf("Error closing blob body: %v", err) // … and ignore err otherwise
		}
		br.body = nil
		time.Sleep(1*time.Second + time.Duration(rand.Intn(100_000))*time.Microsecond) // Some jitter so that a failure blip doesn’t cause a deterministic stampede

		headers := map[string][]string{
			"Range": {fmt.Sprintf("bytes=%d-", br.offset)},
		}
		res, err := br.c.makeRequest(br.ctx, http.MethodGet, br.path, headers, nil, v2Auth, nil)
		if err != nil {
			return n, fmt.Errorf("%w (while reconnecting: %v)", originalErr, err)
		}
		consumedBody := false
		defer func() {
			if !consumedBody {
				res.Body.Close()
			}
		}()
		switch res.StatusCode {
		case http.StatusPartialContent: // OK
			// A client MUST inspect a 206 response's Content-Type and Content-Range field(s) to determine what parts are enclosed and whether additional requests are needed.
			// The recipient of an invalid Content-Range MUST NOT attempt to recombine the received content with a stored representation.
			first, last, completeLength, err := parseContentRange(res)
			if err != nil {
				return n, fmt.Errorf("%w (after reconnecting, invalid Content-Range header: %v)", originalErr, err)
			}
			// We don’t handle responses that start at an unrequested offset, nor responses that terminate before the end of the full blob.
			if first != br.offset || (completeLength != -1 && last+1 != completeLength) {
				return n, fmt.Errorf("%w (after reconnecting at offset %d, got unexpected Content-Range %d-%d/%d)", originalErr, br.offset, first, last, completeLength)
			}
			// Continue below
		case http.StatusOK:
			return n, fmt.Errorf("%w (after reconnecting, server did not process a Range: header, status %d)", originalErr, http.StatusOK)
		default:
			err := registryHTTPResponseToError(res)
			return n, fmt.Errorf("%w (after reconnecting, fetching blob: %v)", originalErr, err)
		}

		logrus.Debugf("Succesfully reconnected to %s", redactedURL)
		consumedBody = true
		br.body = res.Body
		br.lastRetryOffset = br.offset
		return n, nil

	default:
		logrus.Debugf("Error reading blob body from %s: %#v", br.logURL.Redacted(), err)
		return n, err
	}
}

// millisecondsSince is like time.Since(tm).Milliseconds, but it returns a floating-point value
func millisecondsSince(tm time.Time) float64 {
	return float64(time.Since(tm).Nanoseconds()) / 1_000_000.0
}

// errorIfNotReconnecting makes a heuristic decision whether we should reconnect after err at redactedURL; if so, it returns nil,
// otherwise it returns an appropriate error to return to the caller (possibly augmented with data about the heuristic)
func (br *bodyReader) errorIfNotReconnecting(originalErr error, redactedURL string) error {
	totalTime := millisecondsSince(br.firstConnectionTime)
	failureTime := math.NaN()
	if (br.lastSuccessTime != time.Time{}) {
		failureTime = millisecondsSince(br.lastSuccessTime)
	}
	logrus.Debugf("Reading blob body from %s failed (%#v), decision inputs: lastRetryOffset %d, offset %d, %.3f ms since first connection, %.3f ms since last progress",
		redactedURL, originalErr, br.lastRetryOffset, br.offset, totalTime, failureTime)
	progress := br.offset - br.lastRetryOffset
	if progress < bodyReaderMinimumProgress {
		logrus.Debugf("Not reconnecting to %s because only %d bytes progress made", redactedURL, progress)
		return fmt.Errorf("(heuristic tuning data: last retry %d, current offset %d; %.3f ms total, %.3f ms since progress): %w",
			br.lastRetryOffset, br.offset, totalTime, failureTime, originalErr)
	}
	logrus.Infof("Reading blob body from %s failed (%v), reconnecting…", redactedURL, originalErr)
	return nil
}

// Close implements io.ReadCloser
func (br *bodyReader) Close() error {
	if br.body == nil {
		return nil
	}
	err := br.body.Close()
	br.body = nil
	return err
}
