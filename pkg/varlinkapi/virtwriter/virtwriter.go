package virtwriter

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"

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
)

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
	return nil
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
func Reader(r *bufio.Reader, output, errput *os.File, input *io.PipeWriter, resize chan remotecommand.TerminalSize) error {
	var saveb []byte
	var eom int
	for {
		readb := make([]byte, 32*1024)
		n, err := r.Read(readb)
		fmt.Println("Bytes read: ", n, err)
		// TODO, later may be worth checking in len of the read is 0
		if err != nil {
			return errors.Wrapf(err, "Virtual Read failed, %d", n)
		}
		b := append(saveb, readb[0:n]...)
		// no sense in reading less than the header len
		for len(b) > 7 {
			eom = int(binary.BigEndian.Uint32(b[4:8])) + 8
			// The message and header are togther
			if len(b) >= eom {
				out := append([]byte{}, b[8:eom]...)

				switch IntToSocketDest(int(b[0])) {
				case ToStdout:
					n, err := output.Write(out)
					if err != nil {
						return err
					}
					if n < len(out) {
						return errors.New("short write error occurred on stdout")
					}
				case ToStderr:
					n, err := errput.Write(out)
					if err != nil {
						return err
					}
					if n < len(out) {
						return errors.New("short write error occurred on stderr")
					}
				case ToStdin:
					n, err := input.Write(out)
					if err != nil {
						return err
					}
					if n < len(out) {
						return errors.New("short write error occurred on stdin")
					}
				case TerminalResize:
					// Resize events come over in bytes, need to be reserialized
					resizeEvent := remotecommand.TerminalSize{}
					if err := json.Unmarshal(out, &resizeEvent); err != nil {
						return errors.Wrapf(err, "TerminalResize failed")
					}
					resize <- resizeEvent
				case Quit:
					return nil
				}
				b = b[eom:]
			} else {
				// 	We do not have the header and full message, need to slurp again
				saveb = b
				break
			}
		}
	}
	return nil
}

// HangUp sends message to peer to close connection
func HangUp(writer *bufio.Writer) (err error) {
	n := 0
	msg := []byte("HANG-UP")

	writeQuit := NewVirtWriteCloser(writer, Quit)
	if n, err = writeQuit.Write(msg); err != nil {
		return
	}

	if n != len(msg) {
		return errors.New(fmt.Sprintf("Failed to send complete %s message", string(msg)))
	}
	return
}
