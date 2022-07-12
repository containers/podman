package containers

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/moby/term"
	"github.com/sirupsen/logrus"
	terminal "golang.org/x/term"
)

// The CloseWriter interface is used to determine whether we can do a  one-sided
// close of a hijacked connection.
type CloseWriter interface {
	CloseWrite() error
}

// Attach attaches to a running container
func Attach(ctx context.Context, nameOrID string, stdin io.Reader, stdout io.Writer, stderr io.Writer, attachReady chan bool, options *AttachOptions) error {
	if options == nil {
		options = new(AttachOptions)
	}
	isSet := struct {
		stdin  bool
		stdout bool
		stderr bool
	}{
		stdin:  !(stdin == nil || reflect.ValueOf(stdin).IsNil()),
		stdout: !(stdout == nil || reflect.ValueOf(stdout).IsNil()),
		stderr: !(stderr == nil || reflect.ValueOf(stderr).IsNil()),
	}
	// Ensure golang can determine that interfaces are "really" nil
	if !isSet.stdin {
		stdin = (io.Reader)(nil)
	}
	if !isSet.stdout {
		stdout = (io.Writer)(nil)
	}
	if !isSet.stderr {
		stderr = (io.Writer)(nil)
	}

	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	// Do we need to wire in stdin?
	ctnr, err := Inspect(ctx, nameOrID, new(InspectOptions).WithSize(false))
	if err != nil {
		return err
	}

	params, err := options.ToParams()
	if err != nil {
		return err
	}
	detachKeysInBytes := []byte{}
	if options.Changed("DetachKeys") {
		params.Add("detachKeys", options.GetDetachKeys())

		detachKeysInBytes, err = term.ToBytes(options.GetDetachKeys())
		if err != nil {
			return fmt.Errorf("invalid detach keys: %w", err)
		}
	}
	if isSet.stdin {
		params.Add("stdin", "true")
	}
	if isSet.stdout {
		params.Add("stdout", "true")
	}
	if isSet.stderr {
		params.Add("stderr", "true")
	}

	// Unless all requirements are met, don't use "stdin" is a terminal
	file, ok := stdin.(*os.File)
	outFile, outOk := stdout.(*os.File)
	needTTY := ok && outOk && terminal.IsTerminal(int(file.Fd())) && ctnr.Config.Tty
	if needTTY {
		state, err := setRawTerminal(file)
		if err != nil {
			return err
		}
		defer func() {
			if err := terminal.Restore(int(file.Fd()), state); err != nil {
				logrus.Errorf("Unable to restore terminal: %q", err)
			}
			logrus.SetFormatter(&logrus.TextFormatter{})
		}()
	}

	headers := make(http.Header)
	headers.Add("Connection", "Upgrade")
	headers.Add("Upgrade", "tcp")

	var socket net.Conn
	socketSet := false
	dialContext := conn.Client.Transport.(*http.Transport).DialContext
	t := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			c, err := dialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			if !socketSet {
				socket = c
				socketSet = true
			}
			return c, err
		},
		IdleConnTimeout: time.Duration(0),
	}
	conn.Client.Transport = t
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/containers/%s/attach", params, headers, nameOrID)
	if err != nil {
		return err
	}

	if !(response.IsSuccess() || response.IsInformational()) {
		defer response.Body.Close()
		return response.Process(nil)
	}

	if needTTY {
		winChange := make(chan os.Signal, 1)
		winCtx, winCancel := context.WithCancel(ctx)
		defer winCancel()
		notifyWinChange(winCtx, winChange, file, outFile)
		attachHandleResize(ctx, winCtx, winChange, false, nameOrID, file, outFile)
	}

	// If we are attaching around a start, we need to "signal"
	// back that we are in fact attached so that started does
	// not execute before we can attach.
	if attachReady != nil {
		attachReady <- true
	}

	stdoutChan := make(chan error)
	stdinChan := make(chan error, 1) // stdin channel should not block

	if isSet.stdin {
		go func() {
			logrus.Debugf("Copying STDIN to socket")

			_, err := util.CopyDetachable(socket, stdin, detachKeysInBytes)
			if err != nil && err != define.ErrDetach {
				logrus.Errorf("Failed to write input to service: %v", err)
			}
			if err == nil {
				if closeWrite, ok := socket.(CloseWriter); ok {
					if err := closeWrite.CloseWrite(); err != nil {
						logrus.Warnf("Failed to close STDIN for writing: %v", err)
					}
				}
			}
			stdinChan <- err
		}()
	}

	buffer := make([]byte, 1024)
	if ctnr.Config.Tty {
		go func() {
			logrus.Debugf("Copying STDOUT of container in terminal mode")

			if !isSet.stdout {
				stdoutChan <- fmt.Errorf("container %q requires stdout to be set", ctnr.ID)
			}
			// If not multiplex'ed, read from server and write to stdout
			_, err := io.Copy(stdout, socket)

			stdoutChan <- err
		}()

		for {
			select {
			case err := <-stdoutChan:
				if err != nil {
					return err
				}

				return nil
			case err := <-stdinChan:
				if err != nil {
					return err
				}

				return nil
			}
		}
	} else {
		logrus.Debugf("Copying standard streams of container %q in non-terminal mode", ctnr.ID)
		for {
			// Read multiplexed channels and write to appropriate stream
			fd, l, err := DemuxHeader(socket, buffer)
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return nil
				}
				return err
			}
			frame, err := DemuxFrame(socket, buffer, l)
			if err != nil {
				return err
			}

			switch {
			case fd == 0:
				if isSet.stdout {
					if _, err := stdout.Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 1:
				if isSet.stdout {
					if _, err := stdout.Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 2:
				if isSet.stderr {
					if _, err := stderr.Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 3:
				return fmt.Errorf("from service from stream: %s", frame)
			default:
				return fmt.Errorf("unrecognized channel '%d' in header, 0-3 supported", fd)
			}
		}
	}
}

// DemuxHeader reads header for stream from server multiplexed stdin/stdout/stderr/2nd error channel
func DemuxHeader(r io.Reader, buffer []byte) (fd, sz int, err error) {
	n, err := io.ReadFull(r, buffer[0:8])
	if err != nil {
		return
	}
	if n < 8 {
		err = io.ErrUnexpectedEOF
		return
	}

	fd = int(buffer[0])
	if fd < 0 || fd > 3 {
		err = fmt.Errorf(`channel "%d" found, 0-3 supported: %w`, fd, ErrLostSync)
		return
	}

	sz = int(binary.BigEndian.Uint32(buffer[4:8]))
	return
}

// DemuxFrame reads contents for frame from server multiplexed stdin/stdout/stderr/2nd error channel
func DemuxFrame(r io.Reader, buffer []byte, length int) (frame []byte, err error) {
	if len(buffer) < length {
		buffer = append(buffer, make([]byte, length-len(buffer)+1)...)
	}

	n, err := io.ReadFull(r, buffer[0:length])
	if err != nil {
		return nil, err
	}
	if n < length {
		err = io.ErrUnexpectedEOF
		return
	}

	return buffer[0:length], nil
}

// ResizeContainerTTY sets container's TTY height and width in characters
func ResizeContainerTTY(ctx context.Context, nameOrID string, options *ResizeTTYOptions) error {
	if options == nil {
		options = new(ResizeTTYOptions)
	}
	return resizeTTY(ctx, bindings.JoinURL("containers", nameOrID, "resize"), options.Height, options.Width)
}

// ResizeExecTTY sets session's TTY height and width in characters
func ResizeExecTTY(ctx context.Context, nameOrID string, options *ResizeExecTTYOptions) error {
	if options == nil {
		options = new(ResizeExecTTYOptions)
	}
	return resizeTTY(ctx, bindings.JoinURL("exec", nameOrID, "resize"), options.Height, options.Width)
}

// resizeTTY set size of TTY of container
func resizeTTY(ctx context.Context, endpoint string, height *int, width *int) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	params := url.Values{}
	if height != nil {
		params.Set("h", strconv.Itoa(*height))
	}
	if width != nil {
		params.Set("w", strconv.Itoa(*width))
	}
	params.Set("running", "true")
	rsp, err := conn.DoRequest(ctx, nil, http.MethodPost, endpoint, params, nil)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	return rsp.Process(nil)
}

type rawFormatter struct {
	logrus.TextFormatter
}

func (f *rawFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	buffer, err := f.TextFormatter.Format(entry)
	if err != nil {
		return buffer, err
	}
	return append(buffer, '\r'), nil
}

// This is intended to not be run as a goroutine, handling resizing for a container
// or exec session. It will call resize once and then starts a goroutine which calls resize on winChange
func attachHandleResize(ctx, winCtx context.Context, winChange chan os.Signal, isExec bool, id string, file *os.File, outFile *os.File) {
	resize := func() {
		w, h, err := getTermSize(file, outFile)
		if err != nil {
			logrus.Warnf("Failed to obtain TTY size: %v", err)
		}

		var resizeErr error
		if isExec {
			resizeErr = ResizeExecTTY(ctx, id, new(ResizeExecTTYOptions).WithHeight(h).WithWidth(w))
		} else {
			resizeErr = ResizeContainerTTY(ctx, id, new(ResizeTTYOptions).WithHeight(h).WithWidth(w))
		}
		if resizeErr != nil {
			logrus.Debugf("Failed to resize TTY: %v", resizeErr)
		}
	}

	resize()

	go func() {
		for {
			select {
			case <-winCtx.Done():
				return
			case <-winChange:
				resize()
			}
		}
	}()
}

// Configure the given terminal for raw mode
func setRawTerminal(file *os.File) (*terminal.State, error) {
	state, err := makeRawTerm(file)
	if err != nil {
		return nil, err
	}

	logrus.SetFormatter(&rawFormatter{})

	return state, err
}

// ExecStartAndAttach starts and attaches to a given exec session.
func ExecStartAndAttach(ctx context.Context, sessionID string, options *ExecStartAndAttachOptions) error {
	if options == nil {
		options = new(ExecStartAndAttachOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	// TODO: Make this configurable (can't use streams' InputStream as it's
	// buffered)
	terminalFile := os.Stdin
	terminalOutFile := os.Stdout

	logrus.Debugf("Starting & Attaching to exec session ID %q", sessionID)

	// We need to inspect the exec session first to determine whether to use
	// -t.
	resp, err := conn.DoRequest(ctx, nil, http.MethodGet, "/exec/%s/json", nil, nil, sessionID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respStruct := new(define.InspectExecSession)
	if err := resp.Process(respStruct); err != nil {
		return err
	}
	isTerm := true
	if respStruct.ProcessConfig != nil {
		isTerm = respStruct.ProcessConfig.Tty
	}

	// If we are in TTY mode, we need to set raw mode for the terminal.
	// TODO: Share all of this with Attach() for containers.
	needTTY := terminalFile != nil && terminal.IsTerminal(int(terminalFile.Fd())) && isTerm

	body := struct {
		Detach bool   `json:"Detach"`
		TTY    bool   `json:"Tty"`
		Height uint16 `json:"h"`
		Width  uint16 `json:"w"`
	}{
		Detach: false,
		TTY:    needTTY,
	}

	if needTTY {
		state, err := setRawTerminal(terminalFile)
		if err != nil {
			return err
		}
		defer func() {
			if err := terminal.Restore(int(terminalFile.Fd()), state); err != nil {
				logrus.Errorf("Unable to restore terminal: %q", err)
			}
			logrus.SetFormatter(&logrus.TextFormatter{})
		}()
		w, h, err := getTermSize(terminalFile, terminalOutFile)
		if err != nil {
			logrus.Warnf("Failed to obtain TTY size: %v", err)
		}
		body.Width = uint16(w)
		body.Height = uint16(h)
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return err
	}

	var socket net.Conn
	socketSet := false
	dialContext := conn.Client.Transport.(*http.Transport).DialContext
	t := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			c, err := dialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			if !socketSet {
				socket = c
				socketSet = true
			}
			return c, err
		},
		IdleConnTimeout: time.Duration(0),
	}
	conn.Client.Transport = t
	response, err := conn.DoRequest(ctx, bytes.NewReader(bodyJSON), http.MethodPost, "/exec/%s/start", nil, nil, sessionID)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if !(response.IsSuccess() || response.IsInformational()) {
		return response.Process(nil)
	}

	if needTTY {
		winChange := make(chan os.Signal, 1)
		winCtx, winCancel := context.WithCancel(ctx)
		defer winCancel()

		notifyWinChange(winCtx, winChange, terminalFile, terminalOutFile)
		attachHandleResize(ctx, winCtx, winChange, true, sessionID, terminalFile, terminalOutFile)
	}

	if options.GetAttachInput() {
		go func() {
			logrus.Debugf("Copying STDIN to socket")
			_, err := util.CopyDetachable(socket, options.InputStream, []byte{})
			if err != nil {
				logrus.Errorf("Failed to write input to service: %v", err)
			}

			if closeWrite, ok := socket.(CloseWriter); ok {
				logrus.Debugf("Closing STDIN")
				if err := closeWrite.CloseWrite(); err != nil {
					logrus.Warnf("Failed to close STDIN for writing: %v", err)
				}
			}
		}()
	}

	buffer := make([]byte, 1024)
	if isTerm {
		logrus.Debugf("Handling terminal attach to exec")
		if !options.GetAttachOutput() {
			return fmt.Errorf("exec session %s has a terminal and must have STDOUT enabled", sessionID)
		}
		// If not multiplex'ed, read from server and write to stdout
		_, err := util.CopyDetachable(options.GetOutputStream(), socket, []byte{})
		if err != nil {
			return err
		}
	} else {
		logrus.Debugf("Handling non-terminal attach to exec")
		for {
			// Read multiplexed channels and write to appropriate stream
			fd, l, err := DemuxHeader(socket, buffer)
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return nil
				}
				return err
			}
			frame, err := DemuxFrame(socket, buffer, l)
			if err != nil {
				return err
			}

			switch {
			case fd == 0:
				if options.GetAttachInput() {
					// Write STDIN to STDOUT (echoing characters
					// typed by another attach session)
					if _, err := options.GetOutputStream().Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 1:
				if options.GetAttachOutput() {
					if _, err := options.GetOutputStream().Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 2:
				if options.GetAttachError() {
					if _, err := options.GetErrorStream().Write(frame[0:l]); err != nil {
						return err
					}
				}
			case fd == 3:
				return fmt.Errorf("from service from stream: %s", frame)
			default:
				return fmt.Errorf("unrecognized channel '%d' in header, 0-3 supported", fd)
			}
		}
	}
	return nil
}
