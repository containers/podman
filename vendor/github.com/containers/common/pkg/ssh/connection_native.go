package ssh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/containers/common/pkg/config"
)

func nativeConnectionCreate(options ConnectionCreateOptions) error {
	var match bool
	var err error
	if match, err = regexp.Match("^[A-Za-z][A-Za-z0-9+.-]*://", []byte(options.Path)); err != nil {
		return fmt.Errorf("invalid destination: %w", err)
	}

	if !match {
		options.Path = "ssh://" + options.Path
	}

	if len(options.Socket) > 0 {
		options.Path += options.Socket
	}

	dst, uri, err := Validate(options.User, options.Path, options.Port, options.Identity)
	if err != nil {
		return err
	}

	// test connection
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("no ssh binary found")
	}

	if strings.Contains(uri.Host, "/run") {
		uri.Host = strings.Split(uri.Host, "/run")[0]
	}
	conf, err := config.Default()
	if err != nil {
		return err
	}

	args := []string{uri.User.String() + "@" + uri.Hostname()}

	if len(dst.Identity) > 0 {
		args = append(args, "-i", dst.Identity)
	}
	if len(conf.Engine.SSHConfig) > 0 {
		args = append(args, "-F", conf.Engine.SSHConfig)
	}

	output := &bytes.Buffer{}
	args = append(args, "podman", "info", "--format", "json")
	info := exec.Command(ssh, args...)
	info.Stdout = output
	err = info.Run()
	if err != nil {
		return err
	}

	remoteInfo := &Info{}
	if err := json.Unmarshal(output.Bytes(), &remoteInfo); err != nil {
		return fmt.Errorf("failed to parse 'podman info' results: %w", err)
	}

	if remoteInfo.Host.RemoteSocket == nil || len(remoteInfo.Host.RemoteSocket.Path) == 0 {
		return fmt.Errorf("remote podman %q failed to report its UDS socket", uri.Host)
	}

	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
	}
	if options.Default {
		cfg.Engine.ActiveService = options.Name
	}

	if cfg.Engine.ServiceDestinations == nil {
		cfg.Engine.ServiceDestinations = map[string]config.Destination{
			options.Name: *dst,
		}
		cfg.Engine.ActiveService = options.Name
	} else {
		cfg.Engine.ServiceDestinations[options.Name] = *dst
	}

	return cfg.Write()
}

func nativeConnectionExec(options ConnectionExecOptions) (*ConnectionExecReport, error) {
	dst, uri, err := Validate(options.User, options.Host, options.Port, options.Identity)
	if err != nil {
		return nil, err
	}

	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("no ssh binary found")
	}

	output := &bytes.Buffer{}
	errors := &bytes.Buffer{}
	if strings.Contains(uri.Host, "/run") {
		uri.Host = strings.Split(uri.Host, "/run")[0]
	}

	options.Args = append([]string{uri.User.String() + "@" + uri.Hostname()}, options.Args...)
	conf, err := config.Default()
	if err != nil {
		return nil, err
	}

	args := []string{}
	if len(dst.Identity) > 0 {
		args = append(args, "-i", dst.Identity)
	}
	if len(conf.Engine.SSHConfig) > 0 {
		args = append(args, "-F", conf.Engine.SSHConfig)
	}
	args = append(args, options.Args...)
	info := exec.Command(ssh, args...)
	info.Stdout = output
	info.Stderr = errors
	err = info.Run()
	if err != nil {
		return nil, err
	}
	return &ConnectionExecReport{Response: output.String()}, nil
}

func nativeConnectionScp(options ConnectionScpOptions) (*ConnectionScpReport, error) {
	host, remotePath, localPath, swap, err := ParseScpArgs(options)
	if err != nil {
		return nil, err
	}
	dst, uri, err := Validate(options.User, host, options.Port, options.Identity)
	if err != nil {
		return nil, err
	}

	scp, err := exec.LookPath("scp")
	if err != nil {
		return nil, fmt.Errorf("no scp binary found")
	}

	conf, err := config.Default()
	if err != nil {
		return nil, err
	}

	args := []string{}
	if len(dst.Identity) > 0 {
		args = append(args, "-i", dst.Identity)
	}
	if len(conf.Engine.SSHConfig) > 0 {
		args = append(args, "-F", conf.Engine.SSHConfig)
	}

	userString := ""
	if !strings.Contains(host, "@") {
		userString = uri.User.String() + "@"
	}
	// meaning, we are copying from a remote host
	if swap {
		args = append(args, userString+host+":"+remotePath, localPath)
	} else {
		args = append(args, localPath, userString+host+":"+remotePath)
	}

	info := exec.Command(scp, args...)
	err = info.Run()
	if err != nil {
		return nil, err
	}

	return &ConnectionScpReport{Response: remotePath}, nil
}
