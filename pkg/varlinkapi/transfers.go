// +build varlink

package varlinkapi

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/containers/libpod/cmd/podman/varlink"
	"github.com/sirupsen/logrus"
)

// SendFile allows a client to send a file to the varlink server
func (i *LibpodAPI) SendFile(call iopodman.VarlinkCall, ftype string, length int64) error {
	varlink := VarlinkCall{&call}
	if err := varlink.RequiresUpgrade(); err != nil {
		return varlink.ReplyErrorOccurred(err.Error())
	}

	outputFile, err := ioutil.TempFile("", "varlink_send")
	if err != nil {
		return varlink.ReplyErrorOccurred(err.Error())
	}
	defer outputFile.Close()

	if err = varlink.ReplySendFile(outputFile.Name()); err != nil {
		return varlink.ReplyErrorOccurred(err.Error())
	}

	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	reader := varlink.Call.Reader
	if _, err := io.CopyN(writer, reader, length); err != nil {
		return err
	}

	logrus.Debugf("successfully received %s", outputFile.Name())
	// Send an ACK to the client
	varlink.Call.Writer.WriteString(fmt.Sprintf("%s:", outputFile.Name()))
	varlink.Call.Writer.Flush()
	return nil

}

// ReceiveFile allows the varlink server to send a file to a client
func (i *LibpodAPI) ReceiveFile(call iopodman.VarlinkCall, filepath string, delete bool) error {
	varlink := VarlinkCall{&call}
	if err := varlink.RequiresUpgrade(); err != nil {
		return varlink.ReplyErrorOccurred(err.Error())
	}

	fs, err := os.Open(filepath)
	if err != nil {
		return varlink.ReplyErrorOccurred(err.Error())
	}
	fileInfo, err := fs.Stat()
	if err != nil {
		return varlink.ReplyErrorOccurred(err.Error())
	}

	// Send the file length down to client
	// Varlink connection upraded
	if err = varlink.ReplyReceiveFile(fileInfo.Size()); err != nil {
		return varlink.ReplyErrorOccurred(err.Error())
	}

	reader := bufio.NewReader(fs)
	_, err = reader.WriteTo(varlink.Writer)
	if err != nil {
		return err
	}
	if delete {
		if err := os.Remove(filepath); err != nil {
			return err
		}
	}
	return varlink.Writer.Flush()
}
