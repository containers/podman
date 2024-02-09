package compression

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/utils"
	"github.com/containers/storage/pkg/archive"
	"github.com/sirupsen/logrus"
)

const (
	zipExt            = ".zip"
	progressBarPrefix = "Extracting compressed file"
	macOs             = "darwin"
)

type decompressor interface {
	srcFilePath() string
	reader() (io.Reader, error)
	copy(w *os.File, r io.Reader) error
	close()
}

func newDecompressor(compressedFilePath string, compressedFileContent []byte) decompressor {
	compressionType := archive.DetectCompression(compressedFileContent)
	os := runtime.GOOS
	hasZipSuffix := strings.HasSuffix(compressedFilePath, zipExt)

	switch {
	case compressionType == archive.Xz:
		return newXzDecompressor(compressedFilePath)
	case compressionType == archive.Uncompressed && hasZipSuffix:
		return newZipDecompressor(compressedFilePath)
	case compressionType == archive.Uncompressed:
		return newUncompressedDecompressor(compressedFilePath)
	case compressionType == archive.Gzip && os == macOs:
		return newGzipDecompressor(compressedFilePath)
	default:
		return newGenericDecompressor(compressedFilePath)
	}
}

func Decompress(srcVMFile *define.VMFile, dstFilePath string) error {
	srcFilePath := srcVMFile.GetPath()
	// Are we reading full image file?
	// Only few bytes are read to detect
	// the compression type
	srcFileContent, err := srcVMFile.Read()
	if err != nil {
		return err
	}

	d := newDecompressor(srcFilePath, srcFileContent)
	return runDecompression(d, dstFilePath)
}

func runDecompression(d decompressor, dstFilePath string) error {
	decompressorReader, err := d.reader()
	if err != nil {
		return err
	}
	defer d.close()

	stat, err := os.Stat(d.srcFilePath())
	if err != nil {
		return err
	}

	initMsg := progressBarPrefix + ": " + filepath.Base(dstFilePath)
	finalMsg := initMsg + ": done"

	// We are getting the compressed file size but
	// the progress bar needs the full size of the
	// decompressed file.
	// As a result the progress bar shows 100%
	// before the decompression completes.
	// A workaround is to set the size to -1 but the
	// side effect is that we won't see any advancment in
	// the bar.
	// An update in utils.ProgressBar to handle is needed
	// to improve the case of size=-1 (i.e. unkwonw size).
	p, bar := utils.ProgressBar(initMsg, stat.Size(), finalMsg)
	// Wait for bars to complete and then shut down the bars container
	defer p.Wait()

	readProxy := bar.ProxyReader(decompressorReader)
	// Interrupts the bar goroutine. It's important that
	// bar.Abort(false) is called before p.Wait(), otherwise
	// can hang.
	defer bar.Abort(false)

	dstFileWriter, err := os.OpenFile(dstFilePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, stat.Mode())
	if err != nil {
		logrus.Errorf("Unable to open destination file %s for writing: %q", dstFilePath, err)
		return err
	}
	defer func() {
		if err := dstFileWriter.Close(); err != nil {
			logrus.Errorf("Unable to to close destination file %s: %q", dstFilePath, err)
		}
	}()

	err = d.copy(dstFileWriter, readProxy)
	if err != nil {
		logrus.Errorf("Error extracting compressed file: %q", err)
		return err
	}
	return nil
}
