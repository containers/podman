package machine

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v6"
	"github.com/vbauerster/mpb/v6/decor"
)

// DownloadVMImage downloads a VM image from url to given path
// with download status
func DownloadVMImage(downloadURL fmt.Stringer, localImagePath string) error {
	out, err := os.Create(localImagePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	resp, err := http.Get(downloadURL.String())
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error downloading VM image: %s", resp.Status)
	}
	size := resp.ContentLength
	urlSplit := strings.Split(downloadURL.String(), "/")
	prefix := "Downloading VM image: " + urlSplit[len(urlSplit)-1]
	onComplete := prefix + ": done"

	p := mpb.New(
		mpb.WithWidth(60),
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

// Will error out if file without .xz already exists
// Maybe extracting then renameing is a good idea here..
// depends on xz: not pre-installed on mac, so it becomes a brew dependecy
func decompressXZ(src string, output io.Writer) error {
	fmt.Println("Extracting compressed file")
	cmd := exec.Command("xzcat", "-k", src)
	//cmd := exec.Command("xz", "-d", "-k", "-v", src)
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	//cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		if _, err := io.Copy(output, stdOut); err != nil {
			logrus.Error(err)
		}
	}()
	return cmd.Run()
}
