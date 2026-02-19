package ssh

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

func Create(options *ConnectionCreateOptions, kind EngineMode) error {
	if kind == NativeMode {
		return nativeConnectionCreate(*options)
	}
	return golangConnectionCreate(*options)
}

func Dial(options *ConnectionDialOptions, kind EngineMode) (*ssh.Client, error) {
	var rep *ConnectionDialReport
	var err error
	if kind == NativeMode {
		return nil, fmt.Errorf("ssh dial failed: you cannot create a dial-able client with native ssh")
	}
	rep, err = golangConnectionDial(*options)
	if err != nil {
		return nil, err
	}
	return rep.Client, nil
}

func Exec(options *ConnectionExecOptions, kind EngineMode) (string, error) {
	var rep *ConnectionExecReport
	var err error
	if kind == NativeMode {
		rep, err = nativeConnectionExec(*options)
		if err != nil {
			return "", err
		}
	} else {
		rep, err = golangConnectionExec(*options)
		if err != nil {
			return "", err
		}
	}
	return rep.Response, nil
}

func Scp(options *ConnectionScpOptions, kind EngineMode) (string, error) {
	var rep *ConnectionScpReport
	var err error
	if kind == NativeMode {
		if rep, err = nativeConnectionScp(*options); err != nil {
			return "", err
		}
		return rep.Response, nil
	}
	if rep, err = golangConnectionScp(*options); err != nil {
		return "", err
	}
	return rep.Response, nil
}
