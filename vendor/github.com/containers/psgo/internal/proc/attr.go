package proc

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// ParseAttrCurrent returns the contents of /proc/$pid/attr/current of "?" if
// labeling is not supported on the host.
func ParseAttrCurrent(pid string) (string, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%s/attr/current", pid))
	if err != nil {
		_, err = os.Stat(fmt.Sprintf("/proc/%s", pid))
		if os.IsNotExist(err) {
			// PID doesn't exist
			return "", err
		}
		// PID exists but labeling seems to be unsupported
		return "?", nil
	}
	return strings.Trim(string(data), "\n"), nil
}
