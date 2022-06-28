package mpb

import (
	"io"
	"time"
)

type proxyReader struct {
	io.ReadCloser
	bar *Bar
	iT  time.Time
}

func (prox *proxyReader) Read(p []byte) (n int, err error) {
	n, err = prox.ReadCloser.Read(p)
	if n > 0 {
		prox.bar.IncrBy(n, time.Since(prox.iT))
		prox.iT = time.Now()
	}
	if err == io.EOF {
		go prox.bar.SetTotal(0, true)
	}
	return
}

type proxyWriterTo struct {
	*proxyReader
	wt io.WriterTo
}

func (prox *proxyWriterTo) WriteTo(w io.Writer) (n int64, err error) {
	n, err = prox.wt.WriteTo(w)
	if n > 0 {
		prox.bar.IncrInt64(n, time.Since(prox.iT))
		prox.iT = time.Now()
	}
	if err == io.EOF {
		go prox.bar.SetTotal(0, true)
	}
	return
}
