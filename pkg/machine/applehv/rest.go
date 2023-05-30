//go:build arm64 && darwin
// +build arm64,darwin

package applehv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/crc-org/vfkit/pkg/rest/define"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type Endpoint string

const (
	inspect = "/vm/inspect"
	state   = "/vm/state"
	version = "/version"
)

func (vf *VfkitHelper) get(endpoint string, payload io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, endpoint, payload)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (vf *VfkitHelper) post(endpoint string, payload io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, endpoint, payload)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// getRawState asks vfkit for virtual machine state unmodified (see state())
func (vf *VfkitHelper) getRawState() (machine.Status, error) {
	var response define.VMState
	endPoint := vf.Endpoint + state
	serverResponse, err := vf.get(endPoint, nil)
	if err != nil {
		if errors.Is(err, unix.ECONNREFUSED) {
			logrus.Debugf("connection refused: %s", endPoint)
		}
		return "", err
	}
	err = json.NewDecoder(serverResponse.Body).Decode(&response)
	if err != nil {
		return "", err
	}
	return ToMachineStatus(response.State)

}

// state asks vfkit for the virtual machine state. in case the vfkit
// service is not responding, we assume the service is not running
// and return a stopped status
func (vf *VfkitHelper) state() (machine.Status, error) {
	vmState, err := vf.getRawState()
	if err == nil {
		return vmState, err
	}
	if errors.Is(err, unix.ECONNREFUSED) {
		return machine.Stopped, nil
	}
	return "", err
}

func (vf *VfkitHelper) stateChange(newState define.StateChange) error {
	b, err := json.Marshal(define.VMState{State: string(newState)})
	if err != nil {
		return err
	}
	payload := bytes.NewReader(b)
	_, err = vf.post(vf.Endpoint+state, payload)
	return err
}

func (vf *VfkitHelper) stop(force, wait bool) error {
	waitDuration := time.Millisecond * 10
	// TODO Add ability to wait until stopped
	if force {
		if err := vf.stateChange(define.HardStop); err != nil {
			return err
		}
	}
	if err := vf.stateChange(define.Stop); err != nil {
		return err
	}
	if !wait {
		return nil
	}
	waitErr := fmt.Errorf("failed waiting for vm to stop")
	// Backoff to wait on the machine shutdown
	for i := 0; i < 11; i++ {
		_, err := vf.getRawState()
		if err != nil || errors.Is(err, unix.ECONNREFUSED) {
			waitErr = nil
			break
		}
		waitDuration = waitDuration * 2
		logrus.Debugf("backoff wait time: %s", waitDuration.String())
		time.Sleep(waitDuration)
	}
	return waitErr
}
