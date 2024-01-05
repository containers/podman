package stdpull

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	url2 "net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/containers/podman/v4/pkg/machine/compression"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/utils"
	"github.com/sirupsen/logrus"
)

type DiskFromURL struct {
	u            *url2.URL
	finalPath    *define.VMFile
	tempLocation *define.VMFile
}

func NewDiskFromURL(inputPath string, finalPath *define.VMFile, tempDir *define.VMFile) (*DiskFromURL, error) {
	var (
		err error
	)
	u, err := url2.Parse(inputPath)
	if err != nil {
		return nil, err
	}

	// Make sure the temporary location exists before we get too deep
	if _, err := os.Stat(tempDir.GetPath()); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("temporary download directory %s does not exist", tempDir.GetPath())
		}
	}

	remoteImageName := path.Base(inputPath)
	if remoteImageName == "" {
		return nil, fmt.Errorf("invalid url: unable to determine image name in %q", inputPath)
	}

	tempLocation, err := tempDir.AppendToNewVMFile(remoteImageName, nil)
	if err != nil {
		return nil, err
	}

	return &DiskFromURL{
		u:            u,
		finalPath:    finalPath,
		tempLocation: tempLocation,
	}, nil
}

func (d *DiskFromURL) Get() error {
	// this fetches the image and writes it to the temporary location
	if err := d.pull(); err != nil {
		return err
	}
	logrus.Debugf("decompressing %s to %s", d.tempLocation.GetPath(), d.finalPath.GetPath())
	return compression.Decompress(d.tempLocation, d.finalPath.GetPath())
}

func (d *DiskFromURL) pull() error {
	out, err := os.Create(d.tempLocation.GetPath())
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	resp, err := http.Get(d.u.String())
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading VM image %s: %s", d.u.String(), resp.Status)
	}
	size := resp.ContentLength
	prefix := "Downloading VM image: " + filepath.Base(d.tempLocation.GetPath())
	onComplete := prefix + ": done"

	p, bar := utils.ProgressBar(prefix, size, onComplete)

	proxyReader := bar.ProxyReader(resp.Body)
	defer func() {
		if err := proxyReader.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if _, err := io.Copy(out, proxyReader); err != nil {
		return err
	}

	p.Wait()
	return nil
}
