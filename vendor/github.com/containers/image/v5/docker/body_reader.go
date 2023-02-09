package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// bodyReaderMinimumProgress is the minimum progress we consider a good reason to retry
	bodyReaderMinimumProgress = 1 * 1024 * 1024
	// bodyReaderMSSinceLastRetry is the minimum time since a last retry we consider a good reason to retry
	bodyReaderMSSinceLastRetry = 60 * 1_000
)

// bodyReader is an io.ReadCloser returned by dockerImageSource.GetBlob,
// which can transparently resume some (very limited) kinds of aborted connections.
type bodyReader struct {
	ctx                 context.Context
	c                   *dockerClient
	path                string   // path to pass to makeRequest to retry
	logURL              *url.URL // a string to use in error messages
	firstConnectionTime time.Time

	body            io.ReadCloser // The currently open connection we use to read data, or nil if there is nothing to read from / close.
	lastRetryOffset int64         // -1 if N/A
	lastRetryTime   time.Time     // time.Time{} if N/A
	offset          int64         // Current offset within the blob
	lastSuccessTime time.Time     // time.Time{} if N/A

	originalHeaders string
}

// newBodyReader creates a bodyReader for request path in c.
// firstBody is an already correctly opened body for the blob, returning the full blob from the start.
// If reading from firstBody fails, bodyReader may heuristically decide to resume.
func newBodyReader(ctx context.Context, c *dockerClient, path string, resp *http.Response, firstBody io.ReadCloser) (io.ReadCloser, error) {
	logURL, err := c.resolveRequestURL(path)
	if err != nil {
		return nil, err
	}
	var headers bytes.Buffer
	if err := resp.Header.Write(&headers); err != nil {
		headers.Reset()
		fmt.Fprintf(&headers, "Error writing headers: %v", err)
	}
	res := &bodyReader{
		ctx:                 ctx,
		c:                   c,
		path:                path,
		logURL:              logURL,
		firstConnectionTime: time.Now(),

		body:            firstBody,
		lastRetryOffset: -1,
		lastRetryTime:   time.Time{},
		offset:          0,
		lastSuccessTime: time.Time{},

		originalHeaders: headers.String(),
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

	default:
		logrus.Debugf("Error reading blob body from %s: %#v", br.logURL.Redacted(), err)
		originalErr := err
		currentTime := time.Now()
		msSinceFirstConnection := millisecondsSinceOptional(currentTime, br.firstConnectionTime)
		msSinceLastRetry := millisecondsSinceOptional(currentTime, br.lastRetryTime)
		msSinceLastSuccess := millisecondsSinceOptional(currentTime, br.lastSuccessTime)
		logrus.Debugf("Reading blob body from %s failed (%#v), decision inputs: total %d @%.3f ms, last retry %d @%.3f ms, last progress @%.3f ms",
			br.logURL.Redacted(), originalErr, br.offset, msSinceFirstConnection, br.lastRetryOffset, msSinceLastRetry, msSinceLastSuccess)
		logrus.Errorf("Original HTTP response headers: %s", br.originalHeaders)
		return n, fmt.Errorf("[URL: %#v; total %d @%.3f ms, last retry %d @%.3f ms, last progress @ %.3f ms; original response headers: %#v] %w",
			br.logURL.Redacted(), br.offset, msSinceFirstConnection, br.lastRetryOffset, msSinceLastRetry, msSinceLastSuccess,
			br.originalHeaders, err)
	}
}

// millisecondsSinceOptional is like currentTime.Sub(tm).Milliseconds, but it returns a floating-point value.
// If tm is time.Time{}, it returns math.NaN()
func millisecondsSinceOptional(currentTime time.Time, tm time.Time) float64 {
	if tm == (time.Time{}) {
		return math.NaN()
	}
	return float64(currentTime.Sub(tm).Nanoseconds()) / 1_000_000.0
}

// errorIfNotReconnecting makes a heuristic decision whether we should reconnect after err at redactedURL; if so, it returns nil,
// otherwise it returns an appropriate error to return to the caller (possibly augmented with data about the heuristic)
func (br *bodyReader) errorIfNotReconnecting(originalErr error, redactedURL string) error {
	currentTime := time.Now()
	msSinceFirstConnection := millisecondsSinceOptional(currentTime, br.firstConnectionTime)
	msSinceLastRetry := millisecondsSinceOptional(currentTime, br.lastRetryTime)
	msSinceLastSuccess := millisecondsSinceOptional(currentTime, br.lastSuccessTime)
	logrus.Debugf("Reading blob body from %s failed (%#v), decision inputs: total %d @%.3f ms, last retry %d @%.3f ms, last progress @%.3f ms",
		redactedURL, originalErr, br.offset, msSinceFirstConnection, br.lastRetryOffset, msSinceLastRetry, msSinceLastSuccess)
	progress := br.offset - br.lastRetryOffset
	if progress >= bodyReaderMinimumProgress {
		logrus.Infof("Reading blob body from %s failed (%v), reconnecting after %d bytes…", redactedURL, originalErr, progress)
		return nil
	}
	if br.lastRetryTime == (time.Time{}) || msSinceLastRetry >= bodyReaderMSSinceLastRetry {
		if br.lastRetryTime == (time.Time{}) {
			logrus.Infof("Reading blob body from %s failed (%v), reconnecting (first reconnection)…", redactedURL, originalErr)
		} else {
			logrus.Infof("Reading blob body from %s failed (%v), reconnecting after %.3f ms…", redactedURL, originalErr, msSinceLastRetry)
		}
		return nil
	}
	logrus.Debugf("Not reconnecting to %s: insufficient progress %d / time since last retry %.3f ms", redactedURL, progress, msSinceLastRetry)
	return fmt.Errorf("(heuristic tuning data: total %d @%.3f ms, last retry %d @%.3f ms, last progress @ %.3f ms): %w",
		br.offset, msSinceFirstConnection, br.lastRetryOffset, msSinceLastRetry, msSinceLastSuccess, originalErr)
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
