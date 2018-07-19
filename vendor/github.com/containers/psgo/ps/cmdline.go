package ps

import (
	"bytes"
	"io/ioutil"
	"os"
)

// readCmdline can be used for mocking in unit tests.
func readCmdline(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = errNoSuchPID
		}
		return "", err
	}

	return string(data), nil
}

// parseCmdline parses a /proc/$pid/cmdline file and returns a string slice.
func parseCmdline(path string) ([]string, error) {
	raw, err := readCmdline(path)
	if err != nil {
		return nil, err
	}

	cmdLine := []string{}
	for _, rawCmd := range bytes.Split([]byte(raw), []byte{0}) {
		cmdLine = append(cmdLine, string(rawCmd))
	}
	return cmdLine, nil
}
