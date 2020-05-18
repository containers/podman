// +build varlink

package varlinkapi

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"

	iopodman "github.com/containers/libpod/pkg/varlink"
	"github.com/sirupsen/logrus"
)

// SendFile allows a client to send a file to the varlink server
func (i *VarlinkAPI) SendFile(call iopodman.VarlinkCall, ftype string, length int64) error {
	if !call.WantsUpgrade() {
		return call.ReplyErrorOccurred("client must use upgraded connection to send files")
	}

	outputFile, err := ioutil.TempFile("", "varlink_send")
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	defer outputFile.Close()

	if err = call.ReplySendFile(outputFile.Name()); err != nil {
		// If an error occurs while sending the reply, return the error
		return err
	}

	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	reader := call.Call.Reader
	if _, err := io.CopyN(writer, reader, length); err != nil {
		return err
	}

	logrus.Debugf("successfully received %s", outputFile.Name())
	// Send an ACK to the client
	call.Call.Writer.WriteString(outputFile.Name() + ":")
	call.Call.Writer.Flush()
	return nil

}

// ReceiveFile allows the varlink server to send a file to a client
func (i *VarlinkAPI) ReceiveFile(call iopodman.VarlinkCall, filepath string, delete bool) error {
	if !call.WantsUpgrade() {
		return call.ReplyErrorOccurred("client must use upgraded connection to send files")
	}
	fs, err := os.Open(filepath)
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}
	fileInfo, err := fs.Stat()
	if err != nil {
		return call.ReplyErrorOccurred(err.Error())
	}

	// Send the file length down to client
	// Varlink connection upgraded
	if err = call.ReplyReceiveFile(fileInfo.Size()); err != nil {
		// If an error occurs while sending the reply, return the error
		return err
	}

	reader := bufio.NewReader(fs)
	_, err = reader.WriteTo(call.Writer)
	if err != nil {
		return err
	}
	if delete {
		if err := os.Remove(filepath); err != nil {
			return err
		}
	}
	return call.Writer.Flush()
}
