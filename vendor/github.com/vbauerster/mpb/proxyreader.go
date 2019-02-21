package mpb

import (
	"io"
	"time"
)

// proxyReader is io.Reader wrapper, for proxy read bytes
type proxyReader struct {
	io.ReadCloser
	bar *Bar
	iT  time.Time
}

func (pr *proxyReader) Read(p []byte) (n int, err error) {
	n, err = pr.ReadCloser.Read(p)
	if n > 0 {
		pr.bar.IncrBy(n, time.Since(pr.iT))
		pr.iT = time.Now()
	}
	return
}
