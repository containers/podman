package virtwriter

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/remotecommand"
)

// SocketDest is the "key" to where IO should go on the varlink
// multiplexed socket
type SocketDest int

const (
	// ToStdout indicates traffic should go stdout
	ToStdout SocketDest = iota
	// ToStdin indicates traffic came from stdin
	ToStdin SocketDest = iota
	// ToStderr indicates traffuc should go to stderr
	ToStderr SocketDest = iota
	// TerminalResize indicates a terminal resize event has occurred
	// and data should be passed to resizer
	TerminalResize SocketDest = iota
	// Quit and detach
	Quit SocketDest = iota
	// HangUpFromClient hangs up from the client
	HangUpFromClient SocketDest = iota
)

// ErrClientHangup signifies that the client wants to drop its connection from
// the server.
var ErrClientHangup = errors.New("client hangup")

// IntToSocketDest returns a socketdest based on integer input
func IntToSocketDest(i int) SocketDest {
	switch i {
	case ToStdout.Int():
		return ToStdout
	case ToStderr.Int():
		return ToStderr
	case ToStdin.Int():
		return ToStdin
	case TerminalResize.Int():
		return TerminalResize
	case Quit.Int():
		return Quit
	case HangUpFromClient.Int():
		return HangUpFromClient
	default:
		return ToStderr
	}
}

// Int returns the integer representation of the socket dest
func (sd SocketDest) Int() int {
	return int(sd)
}

// VirtWriteCloser are writers for attach which include the dest
// of the data
type VirtWriteCloser struct {
	writer *bufio.Writer
	dest   SocketDest
}

// NewVirtWriteCloser is a constructor
func NewVirtWriteCloser(w *bufio.Writer, dest SocketDest) VirtWriteCloser {
	return VirtWriteCloser{w, dest}
}

// Close is a required method for a writecloser
func (v VirtWriteCloser) Close() error {
	return v.writer.Flush()
}

// Write prepends a header to the input message.  The header is
// 8bytes.  Position one contains the destination.  Positions
// 5,6,7,8 are a big-endian encoded uint32 for len of the message.
func (v VirtWriteCloser) Write(input []byte) (int, error) {
	header := []byte{byte(v.dest), 0, 0, 0}
	// Go makes us define the byte for big endian
	mlen := make([]byte, 4)
	binary.BigEndian.PutUint32(mlen, uint32(len(input)))
	// append the message len to the header
	msg := append(header, mlen...)
	// append the message to the header
	msg = append(msg, input...)
	_, err := v.writer.Write(msg)
	if err != nil {
		return 0, err
	}
	err = v.writer.Flush()
	return len(input), err
}

// Reader decodes the content that comes over the wire and directs it to the proper destination.
func Reader(r *bufio.Reader, output, errput, input io.Writer, resize chan remotecommand.TerminalSize, execEcChan chan int) error {
	var messageSize int64
	headerBytes := make([]byte, 8)

	if r == nil {
		return errors.Errorf("Reader must not be nil")
	}
	for {
		n, err := io.ReadFull(r, headerBytes)
		if err != nil {
			return errors.Wrapf(err, "Virtual Read failed, %d", n)
		}
		if n < 8 {
			return errors.New("short read and no full header read")
		}

		messageSize = int64(binary.BigEndian.Uint32(headerBytes[4:8]))
		switch IntToSocketDest(int(headerBytes[0])) {
		case ToStdout:
			if output != nil {
				_, err := io.CopyN(output, r, messageSize)
				if err != nil {
					return err
				}
			}
		case ToStderr:
			if errput != nil {
				_, err := io.CopyN(errput, r, messageSize)
				if err != nil {
					return err
				}
			}
		case ToStdin:
			if input != nil {
				_, err := io.CopyN(input, r, messageSize)
				if err != nil {
					return err
				}
			}
		case TerminalResize:
			if resize != nil {
				out := make([]byte, messageSize)
				if messageSize > 0 {
					_, err = io.ReadFull(r, out)

					if err != nil {
						return err
					}
				}
				// Resize events come over in bytes, need to be reserialized
				resizeEvent := remotecommand.TerminalSize{}
				if err := json.Unmarshal(out, &resizeEvent); err != nil {
					return err
				}
				resize <- resizeEvent
			}
		case Quit:
			out := make([]byte, messageSize)
			if messageSize > 0 {
				_, err = io.ReadFull(r, out)

				if err != nil {
					return err
				}
			}
			if execEcChan != nil {
				ecInt := binary.BigEndian.Uint32(out)
				execEcChan <- int(ecInt)
			}
			return nil
		case HangUpFromClient:
			// This sleep allows the pipes to flush themselves before tearing everything down.
			// It makes me sick to do it but after a full day I cannot put my finger on the race
			// that occurs when closing things up.  It would require a significant rewrite of code
			// to make the pipes close down properly.  Given that we are currently discussing a
			// rewrite of all things remote, this hardly seems worth resolving.
			//
			// reproducer: echo hello | (podman-remote run -i alpine cat)
			time.Sleep(1 * time.Second)
			return ErrClientHangup
		default:
			// Something really went wrong
			return errors.New("unknown multiplex destination")
		}
	}
}

// HangUp sends message to peer to close connection
func HangUp(writer *bufio.Writer, ec uint32) (err error) {
	n := 0
	msg := make([]byte, 4)

	binary.BigEndian.PutUint32(msg, ec)

	writeQuit := NewVirtWriteCloser(writer, Quit)
	if n, err = writeQuit.Write(msg); err != nil {
		return
	}

	if n != len(msg) {
		return errors.Errorf("Failed to send complete %s message", string(msg))
	}
	return
}
