package ssh

func Create(options *ConnectionCreateOptions, kind EngineMode) error {
	if kind == NativeMode {
		return nativeConnectionCreate(*options)
	}
	return golangConnectionCreate(*options)
}

func Dial(options *ConnectionDialOptions, kind EngineMode) (*ConnectionDialReport, error) {
	var rep *ConnectionDialReport
	var err error
	if kind == NativeMode {
		rep, err = nativeConnectionDial(*options)
	} else {
		rep, err = golangConnectionDial(*options)
	}
	if err != nil {
		return nil, err
	}
	return rep, nil
}

func Exec(options *ConnectionExecOptions, kind EngineMode) (string, error) {
	var rep *ConnectionExecReport
	var err error
	if kind == NativeMode {
		rep, err = nativeConnectionExec(*options)
	} else {
		rep, err = golangConnectionExec(*options)
	}
	if err != nil {
		return "", err
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
