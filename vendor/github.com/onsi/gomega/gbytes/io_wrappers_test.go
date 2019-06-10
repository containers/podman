package gbytes_test

import (
	"fmt"
	"io"
	"time"

	. "github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type FakeCloser struct {
	err      error
	duration time.Duration
}

func (f FakeCloser) Close() error {
	time.Sleep(f.duration)
	return f.err
}

type FakeReader struct {
	err      error
	duration time.Duration
}

func (f FakeReader) Read(p []byte) (int, error) {
	time.Sleep(f.duration)
	if f.err != nil {
		return 0, f.err
	}

	for i := 0; i < len(p); i++ {
		p[i] = 'a'
	}

	return len(p), nil
}

type FakeWriter struct {
	err      error
	duration time.Duration
}

func (f FakeWriter) Write(p []byte) (int, error) {
	time.Sleep(f.duration)
	if f.err != nil {
		return 0, f.err
	}

	return len(p), nil
}

var _ = Describe("Io Wrappers", func() {
	Describe("TimeoutCloser", func() {
		var innerCloser io.Closer
		var timeoutCloser io.Closer

		JustBeforeEach(func() {
			timeoutCloser = TimeoutCloser(innerCloser, 20*time.Millisecond)
		})

		Context("when the underlying Closer closes with no error", func() {
			BeforeEach(func() {
				innerCloser = FakeCloser{}
			})

			It("returns with no error", func() {
				Expect(timeoutCloser.Close()).Should(Succeed())
			})
		})

		Context("when the underlying Closer closes with an error", func() {
			BeforeEach(func() {
				innerCloser = FakeCloser{err: fmt.Errorf("boom")}
			})

			It("returns the error", func() {
				Expect(timeoutCloser.Close()).Should(MatchError("boom"))
			})
		})

		Context("when the underlying Closer hangs", func() {
			BeforeEach(func() {
				innerCloser = FakeCloser{
					err:      fmt.Errorf("boom"),
					duration: time.Hour,
				}
			})

			It("returns ErrTimeout", func() {
				Expect(timeoutCloser.Close()).Should(MatchError(ErrTimeout))
			})
		})
	})

	Describe("TimeoutReader", func() {
		var innerReader io.Reader
		var timeoutReader io.Reader

		JustBeforeEach(func() {
			timeoutReader = TimeoutReader(innerReader, 20*time.Millisecond)
		})

		Context("when the underlying Reader returns no error", func() {
			BeforeEach(func() {
				innerReader = FakeReader{}
			})

			It("returns with no error", func() {
				p := make([]byte, 5)
				n, err := timeoutReader.Read(p)
				Expect(n).Should(Equal(5))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(p).Should(Equal([]byte("aaaaa")))
			})
		})

		Context("when the underlying Reader returns an error", func() {
			BeforeEach(func() {
				innerReader = FakeReader{err: fmt.Errorf("boom")}
			})

			It("returns the error", func() {
				p := make([]byte, 5)
				_, err := timeoutReader.Read(p)
				Expect(err).Should(MatchError("boom"))
			})
		})

		Context("when the underlying Reader hangs", func() {
			BeforeEach(func() {
				innerReader = FakeReader{err: fmt.Errorf("boom"), duration: time.Hour}
			})

			It("returns ErrTimeout", func() {
				p := make([]byte, 5)
				_, err := timeoutReader.Read(p)
				Expect(err).Should(MatchError(ErrTimeout))
			})
		})
	})

	Describe("TimeoutWriter", func() {
		var innerWriter io.Writer
		var timeoutWriter io.Writer

		JustBeforeEach(func() {
			timeoutWriter = TimeoutWriter(innerWriter, 20*time.Millisecond)
		})

		Context("when the underlying Writer returns no error", func() {
			BeforeEach(func() {
				innerWriter = FakeWriter{}
			})

			It("returns with no error", func() {
				n, err := timeoutWriter.Write([]byte("aaaaa"))
				Expect(n).Should(Equal(5))
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the underlying Writer returns an error", func() {
			BeforeEach(func() {
				innerWriter = FakeWriter{err: fmt.Errorf("boom")}
			})

			It("returns the error", func() {
				_, err := timeoutWriter.Write([]byte("aaaaa"))
				Expect(err).Should(MatchError("boom"))
			})
		})

		Context("when the underlying Writer hangs", func() {
			BeforeEach(func() {
				innerWriter = FakeWriter{err: fmt.Errorf("boom"), duration: time.Hour}
			})

			It("returns ErrTimeout", func() {
				_, err := timeoutWriter.Write([]byte("aaaaa"))
				Expect(err).Should(MatchError(ErrTimeout))
			})
		})
	})
})
