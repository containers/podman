package connection

import (
	"bytes"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// ExecRemoteCommand takes a ssh client connection and a command to run and executes the
// command on the specified client. The function returns the Stdout from the client or the Stderr
func ExecRemoteCommand(dial *ssh.Client, run string) ([]byte, error) {
	sess, err := dial.NewSession() // new ssh client session
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	var buffer bytes.Buffer
	var bufferErr bytes.Buffer
	sess.Stdout = &buffer                 // output from client funneled into buffer
	sess.Stderr = &bufferErr              // err form client funneled into buffer
	if err := sess.Run(run); err != nil { // run the command on the ssh client
		return nil, errors.Wrapf(err, bufferErr.String())
	}
	return buffer.Bytes(), nil
}
