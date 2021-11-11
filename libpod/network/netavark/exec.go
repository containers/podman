package netavark

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"

	"github.com/sirupsen/logrus"
)

type netavarkError struct {
	exitCode int
	// Set the json key to "error" so we can directly unmarshal into this struct
	Msg string `json:"error"`
	err error
}

func (e *netavarkError) Error() string {
	ec := ""
	// only add the exit code the the error message if we have at least info log level
	// the normal user does not need to care about the number
	if e.exitCode > 0 && logrus.IsLevelEnabled(logrus.InfoLevel) {
		ec = " (exit code " + strconv.Itoa(e.exitCode) + ")"
	}
	msg := "netavark" + ec
	if len(msg) > 0 {
		msg += ": " + e.Msg
	}
	if e.err != nil {
		msg += ": " + e.err.Error()
	}
	return msg
}

func (e *netavarkError) Unwrap() error {
	return e.err
}

func newNetavarkError(msg string, err error) error {
	return &netavarkError{
		Msg: msg,
		err: err,
	}
}

// getRustLogEnv returns the RUST_LOG env var based on the current logrus level
func getRustLogEnv() string {
	level := logrus.GetLevel().String()
	// rust env_log uses warn instead of warning
	if level == "warning" {
		level = "warn"
	}
	// the rust netlink library is very verbose
	// make sure to only log netavark logs
	return "RUST_LOG=netavark=" + level
}

// execNetavark will execute netavark with the following arguments
// It takes the path to the binary, the list of args and an interface which is
// marshaled to json and send via stdin to netavark. The result interface is
// used to marshal the netavark output into it. This can be nil.
// All errors return by this function should be of the type netavarkError
// to provide a helpful error message.
func execNetavark(binary string, args []string, stdin, result interface{}) error {
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		return newNetavarkError("failed to create stdin pipe", err)
	}
	defer stdinR.Close()

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return newNetavarkError("failed to create stdout pipe", err)
	}
	defer stdoutR.Close()
	defer stdoutW.Close()

	cmd := exec.Command(binary, args...)
	// connect the pipes to stdin and stdout
	cmd.Stdin = stdinR
	cmd.Stdout = stdoutW
	// connect stderr to the podman stderr for logging
	cmd.Stderr = os.Stderr
	// set the netavark log level to the same as the podman
	cmd.Env = append(os.Environ(), getRustLogEnv())
	// if we run with debug log level lets also set RUST_BACKTRACE=1 so we can get the full stack trace in case of panics
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmd.Env = append(cmd.Env, "RUST_BACKTRACE=1")
	}

	err = cmd.Start()
	if err != nil {
		return newNetavarkError("failed to start process", err)
	}
	err = json.NewEncoder(stdinW).Encode(stdin)
	stdinW.Close()
	if err != nil {
		return newNetavarkError("failed to encode stdin data", err)
	}

	dec := json.NewDecoder(stdoutR)

	err = cmd.Wait()
	stdoutW.Close()
	if err != nil {
		exitError := &exec.ExitError{}
		if errors.As(err, &exitError) {
			ne := &netavarkError{}
			// lets disallow unknown fields to make sure we do not get some unexpected stuff
			dec.DisallowUnknownFields()
			// this will unmarshal the error message into the error struct
			ne.err = dec.Decode(ne)
			ne.exitCode = exitError.ExitCode()
			return ne
		}
		return newNetavarkError("unexpected failure during execution", err)
	}

	if result != nil {
		err = dec.Decode(result)
		if err != nil {
			return newNetavarkError("failed to decode result", err)
		}
	}
	return nil
}
