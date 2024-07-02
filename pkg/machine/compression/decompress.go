package compression

import (
	"io"
	"os"
	"strings"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/storage/pkg/archive"
	"github.com/sirupsen/logrus"
)

const (
	decompressedFileFlag = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	macOs                = "darwin"
	progressBarPrefix    = "Extracting compressed file"
	zipExt               = ".zip"
	magicNumberMaxBytes  = 10
)

type decompressor interface {
	compressedFileSize() int64
	compressedFileMode() os.FileMode
	compressedFileReader() (io.ReadCloser, error)
	decompress(w io.WriteSeeker, r io.Reader) error
	close()
}

func Decompress(compressedVMFile *define.VMFile, decompressedFilePath string) error {
	compressedFilePath := compressedVMFile.GetPath()
	compressedFileMagicNum, err := compressedVMFile.ReadMagicNumber(magicNumberMaxBytes)
	if err != nil {
		return err
	}

	var d decompressor
	if d, err = newDecompressor(compressedFilePath, compressedFileMagicNum); err != nil {
		return err
	}

	return runDecompression(d, decompressedFilePath)
}

func newDecompressor(compressedFilePath string, compressedFileMagicNum []byte) (decompressor, error) {
	compressionType := archive.DetectCompression(compressedFileMagicNum)
	hasZipSuffix := strings.HasSuffix(compressedFilePath, zipExt)

	switch {
	// Zip files are not guaranteed to have a magic number at the beginning
	// of the file, so we need to use the file name to detect them.
	case compressionType == archive.Uncompressed && hasZipSuffix:
		return newZipDecompressor(compressedFilePath)
	case compressionType == archive.Uncompressed:
		return newUncompressedDecompressor(compressedFilePath)
	default:
		return newGenericDecompressor(compressedFilePath)
	}
}

func runDecompression(d decompressor, decompressedFilePath string) (retErr error) {
	compressedFileReader, err := d.compressedFileReader()
	if err != nil {
		return err
	}
	defer d.close()

	var decompressedFileWriter *os.File

	if decompressedFileWriter, err = os.OpenFile(decompressedFilePath, decompressedFileFlag, d.compressedFileMode()); err != nil {
		logrus.Errorf("Unable to open destination file %s for writing: %q", decompressedFilePath, err)
		return err
	}
	defer func() {
		if err := decompressedFileWriter.Close(); err != nil {
			logrus.Warnf("Unable to to close destination file %s: %q", decompressedFilePath, err)
			if retErr == nil {
				retErr = err
			}
		}
	}()

	if err = d.decompress(decompressedFileWriter, compressedFileReader); err != nil {
		logrus.Errorf("Error extracting compressed file: %q", err)
		return err
	}

	return nil
}
