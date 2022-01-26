package mpb

import (
	"io"
	"io/ioutil"
	"time"
)

type proxyReader struct {
	io.ReadCloser
	bar *Bar
}

func (x proxyReader) Read(p []byte) (int, error) {
	n, err := x.ReadCloser.Read(p)
	x.bar.IncrBy(n)
	if err == io.EOF {
		go x.bar.SetTotal(-1, true)
	}
	return n, err
}

type proxyWriterTo struct {
	proxyReader
	wt io.WriterTo
}

func (x proxyWriterTo) WriteTo(w io.Writer) (int64, error) {
	n, err := x.wt.WriteTo(w)
	x.bar.IncrInt64(n)
	if err == io.EOF {
		go x.bar.SetTotal(-1, true)
	}
	return n, err
}

type ewmaProxyReader struct {
	proxyReader
}

func (x ewmaProxyReader) Read(p []byte) (int, error) {
	start := time.Now()
	n, err := x.proxyReader.Read(p)
	if n > 0 {
		x.bar.DecoratorEwmaUpdate(time.Since(start))
	}
	return n, err
}

type ewmaProxyWriterTo struct {
	ewmaProxyReader
	wt proxyWriterTo
}

func (x ewmaProxyWriterTo) WriteTo(w io.Writer) (int64, error) {
	start := time.Now()
	n, err := x.wt.WriteTo(w)
	if n > 0 {
		x.bar.DecoratorEwmaUpdate(time.Since(start))
	}
	return n, err
}

func (b *Bar) newProxyReader(r io.Reader) (rc io.ReadCloser) {
	pr := proxyReader{toReadCloser(r), b}
	if wt, ok := r.(io.WriterTo); ok {
		pw := proxyWriterTo{pr, wt}
		if b.hasEwmaDecorators {
			rc = ewmaProxyWriterTo{ewmaProxyReader{pr}, pw}
		} else {
			rc = pw
		}
	} else if b.hasEwmaDecorators {
		rc = ewmaProxyReader{pr}
	} else {
		rc = pr
	}
	return rc
}

func toReadCloser(r io.Reader) io.ReadCloser {
	if rc, ok := r.(io.ReadCloser); ok {
		return rc
	}
	return ioutil.NopCloser(r)
}
