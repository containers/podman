package main

import "github.com/containers/podman/v4/libpod/define"

type clientInfo struct {
	OSArch   string `json:"OS"`
	Provider string `json:"provider"`
	Version  string `json:"version"`
}

func getClientInfo() (*clientInfo, error) {
	p, err := getProvider()
	if err != nil {
		return nil, err
	}
	vinfo, err := define.GetVersion()
	if err != nil {
		return nil, err
	}
	return &clientInfo{
		OSArch:   vinfo.OsArch,
		Provider: p,
		Version:  vinfo.Version,
	}, nil
}
