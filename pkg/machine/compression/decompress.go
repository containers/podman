package compression

import (
	"archive/zip"
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/containers/image/v5/pkg/compression"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/archive"
	"github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
)

func Decompress(localPath *define.VMFile, uncompressedPath string) error {
	var isZip bool
	uncompressedFileWriter, err := os.OpenFile(uncompressedPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	sourceFile, err := localPath.Read()
	if err != nil {
		return err
	}
	if strings.HasSuffix(localPath.GetPath(), ".zip") {
		isZip = true
	}
	prefix := "Copying uncompressed file"
	compressionType := archive.DetectCompression(sourceFile)
	if compressionType != archive.Uncompressed || isZip {
		prefix = "Extracting compressed file"
	}
	prefix += ": " + filepath.Base(uncompressedPath)
	if compressionType == archive.Xz {
		return decompressXZ(prefix, localPath.GetPath(), uncompressedFileWriter)
	}
	if isZip && runtime.GOOS == "windows" {
		return decompressZip(prefix, localPath.GetPath(), uncompressedFileWriter)
	}
	return decompressEverythingElse(prefix, localPath.GetPath(), uncompressedFileWriter)
}

// Will error out if file without .Xz already exists
// Maybe extracting then renaming is a good idea here..
// depends on Xz: not pre-installed on mac, so it becomes a brew dependency
func decompressXZ(prefix string, src string, output io.WriteCloser) error {
	var read io.Reader
	var cmd *exec.Cmd

	stat, err := os.Stat(src)
	if err != nil {
		return err
	}
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	p, bar := utils.ProgressBar(prefix, stat.Size(), prefix+": done")
	proxyReader := bar.ProxyReader(file)
	defer func() {
		if err := proxyReader.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	// Prefer Xz utils for fastest performance, fallback to go xi2 impl
	if _, err := exec.LookPath("xz"); err == nil {
		cmd = exec.Command("xz", "-d", "-c")
		cmd.Stdin = proxyReader
		read, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}
		cmd.Stderr = os.Stderr
	} else {
		// This XZ implementation is reliant on buffering. It is also 3x+ slower than XZ utils.
		// Consider replacing with a faster implementation (e.g. xi2) if podman machine is
		// updated with a larger image for the distribution base.
		buf := bufio.NewReader(proxyReader)
		read, err = xz.NewReader(buf)
		if err != nil {
			return err
		}
	}

	done := make(chan bool)
	go func() {
		if _, err := io.Copy(output, read); err != nil {
			logrus.Error(err)
		}
		output.Close()
		done <- true
	}()

	if cmd != nil {
		err := cmd.Start()
		if err != nil {
			return err
		}
		p.Wait()
		return cmd.Wait()
	}
	<-done
	p.Wait()
	return nil
}

func decompressEverythingElse(prefix string, src string, output io.WriteCloser) error {
	stat, err := os.Stat(src)
	if err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	p, bar := utils.ProgressBar(prefix, stat.Size(), prefix+": done")
	proxyReader := bar.ProxyReader(f)
	defer func() {
		if err := proxyReader.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	uncompressStream, _, err := compression.AutoDecompress(proxyReader)
	if err != nil {
		return err
	}
	defer func() {
		if err := uncompressStream.Close(); err != nil {
			logrus.Error(err)
		}
		if err := output.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	_, err = io.Copy(output, uncompressStream)
	p.Wait()
	return err
}

func decompressZip(prefix string, src string, output io.WriteCloser) error {
	zipReader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	if len(zipReader.File) != 1 {
		return errors.New("machine image files should consist of a single compressed file")
	}
	f, err := zipReader.File[0].Open()
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	defer func() {
		if err := output.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	size := int64(zipReader.File[0].CompressedSize64)
	p, bar := utils.ProgressBar(prefix, size, prefix+": done")
	proxyReader := bar.ProxyReader(f)
	defer func() {
		if err := proxyReader.Close(); err != nil {
			logrus.Error(err)
		}
	}()
	_, err = io.Copy(output, proxyReader)
	p.Wait()
	return err
}
