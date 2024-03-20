package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/containers/storage/pkg/archive"
	"github.com/containers/storage/pkg/chrootarchive"
	"github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// ExecCmd executes a command with args and returns its output as a string along
// with an error, if any.
func ExecCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("`%v %v` failed: %v %v (%v)", name, strings.Join(args, " "), stderr.String(), stdout.String(), err)
	}

	return stdout.String(), nil
}

// ExecCmdWithStdStreams execute a command with the specified standard streams.
func ExecCmdWithStdStreams(stdin io.Reader, stdout, stderr io.Writer, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = env

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("`%v %v` failed: %v", name, strings.Join(args, " "), err)
	}

	return nil
}

// UntarToFileSystem untars an os.file of a tarball to a destination in the filesystem
func UntarToFileSystem(dest string, tarball *os.File, options *archive.TarOptions) error {
	logrus.Debugf("untarring %s", tarball.Name())
	return archive.Untar(tarball, dest, options)
}

// Creates a new tar file and writes bytes from io.ReadCloser
func CreateTarFromSrc(source string, dest string) error {
	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("could not create tarball file '%s': %w", dest, err)
	}
	defer file.Close()
	return TarChrootToFilesystem(source, file)
}

// TarToFilesystem creates a tarball from source and writes to an os.file
// provided
func TarToFilesystem(source string, tarball *os.File) error {
	tb, err := Tar(source)
	if err != nil {
		return err
	}
	defer tb.Close()
	_, err = io.Copy(tarball, tb)
	if err != nil {
		return err
	}
	logrus.Debugf("wrote tarball file %s", tarball.Name())
	return nil
}

// Tar creates a tarball from source and returns a readcloser of it
func Tar(source string) (io.ReadCloser, error) {
	logrus.Debugf("creating tarball of %s", source)
	return archive.Tar(source, archive.Uncompressed)
}

// TarChrootToFilesystem creates a tarball from source and writes to an os.file
// provided while chrooted to the source.
func TarChrootToFilesystem(source string, tarball *os.File) error {
	tb, err := TarWithChroot(source)
	if err != nil {
		return err
	}
	defer tb.Close()
	_, err = io.Copy(tarball, tb)
	if err != nil {
		return err
	}
	logrus.Debugf("wrote tarball file %s", tarball.Name())
	return nil
}

// TarWithChroot creates a tarball from source and returns a readcloser of it
// while chrooted to the source.
func TarWithChroot(source string) (io.ReadCloser, error) {
	logrus.Debugf("creating tarball of %s", source)
	return chrootarchive.Tar(source, nil, source)
}

// GuardedRemoveAll functions much like os.RemoveAll but
// will not delete certain catastrophic paths.
func GuardedRemoveAll(path string) error {
	if path == "" || path == "/" {
		return fmt.Errorf("refusing to recursively delete `%s`", path)
	}
	return os.RemoveAll(path)
}

func ProgressBar(prefix string, size int64, onComplete string) (*mpb.Progress, *mpb.Bar) {
	p := mpb.New(
		mpb.WithWidth(80), // Do not go below 80, see bug #17718
		mpb.WithRefreshRate(180*time.Millisecond),
	)

	bar := p.AddBar(size,
		mpb.BarFillerClearOnComplete(),
		mpb.PrependDecorators(
			decor.OnComplete(decor.Name(prefix), onComplete),
		),
		mpb.AppendDecorators(
			decor.OnComplete(decor.CountersKibiByte("%.1f / %.1f"), ""),
		),
	)
	if size == 0 {
		bar.SetTotal(0, true)
	}

	return p, bar
}
