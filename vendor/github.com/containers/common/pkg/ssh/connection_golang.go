package ssh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/storage/pkg/homedir"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

func golangConnectionCreate(options ConnectionCreateOptions) error {
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

	if uri.Path == "" || uri.Path == "/" {
		if uri.Path, err = getUDS(uri, options.Identity); err != nil {
			return err
		}
		dst.URI += uri.Path
	}

	cfg, err := config.ReadCustomConfig()
	if err != nil {
		return err
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

func golangConnectionDial(options ConnectionDialOptions) (*ConnectionDialReport, error) {
	_, uri, err := Validate(options.User, options.Host, options.Port, options.Identity)
	if err != nil {
		return nil, err
	}
	cfg, err := ValidateAndConfigure(uri, options.Identity)
	if err != nil {
		return nil, err
	}

	dial, err := ssh.Dial("tcp", uri.Host, cfg) // dial the client
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &ConnectionDialReport{dial}, nil
}

func golangConnectionExec(options ConnectionExecOptions) (*ConnectionExecReport, error) {
	_, uri, err := Validate(options.User, options.Host, options.Port, options.Identity)
	if err != nil {
		return nil, err
	}

	cfg, err := ValidateAndConfigure(uri, options.Identity)
	if err != nil {
		return nil, err
	}
	dialAdd, err := ssh.Dial("tcp", uri.Host, cfg) // dial the client
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	out, err := ExecRemoteCommand(dialAdd, strings.Join(options.Args, " "))
	if err != nil {
		return nil, err
	}
	return &ConnectionExecReport{Response: string(out)}, nil
}

func golangConnectionScp(options ConnectionScpOptions) (*ConnectionScpReport, error) {
	host, remoteFile, localFile, swap, err := ParseScpArgs(options)
	if err != nil {
		return nil, err
	}

	_, uri, err := Validate(options.User, host, options.Port, options.Identity)
	if err != nil {
		return nil, err
	}
	cfg, err := ValidateAndConfigure(uri, options.Identity)
	if err != nil {
		return nil, err
	}

	dial, err := ssh.Dial("tcp", uri.Host, cfg) // dial the client
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	sc, err := sftp.NewClient(dial)
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(localFile, (os.O_RDWR | os.O_CREATE), 0o644)
	if err != nil {
		return nil, err
	}

	parent := filepath.Dir(remoteFile)
	path := string(filepath.Separator)
	dirs := strings.Split(parent, path)
	for _, dir := range dirs {
		path = filepath.Join(path, dir)
		// ignore errors due to most of the dirs already existing
		_ = sc.Mkdir(path)
	}

	remote, err := sc.OpenFile(remoteFile, (os.O_RDWR | os.O_CREATE))
	if err != nil {
		return nil, err
	}
	defer remote.Close()

	if !swap {
		_, err = io.Copy(remote, f)
		if err != nil {
			return nil, err
		}
	} else {
		_, err = io.Copy(f, remote)
		if err != nil {
			return nil, err
		}
	}
	return &ConnectionScpReport{Response: remote.Name()}, nil
}

// ExecRemoteCommand takes a ssh client connection and a command to run and executes the
// command on the specified client. The function returns the Stdout from the client or the Stderr
func ExecRemoteCommand(dial *ssh.Client, run string) ([]byte, error) {
	sess, err := dial.NewSession() // new ssh client session
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	var buffer bytes.Buffer
	var bufferErr bytes.Buffer
	sess.Stdout = &buffer                 // output from client funneled into buffer
	sess.Stderr = &bufferErr              // err form client funneled into buffer
	if err := sess.Run(run); err != nil { // run the command on the ssh client
		return nil, fmt.Errorf("%v: %w", bufferErr.String(), err)
	}
	return buffer.Bytes(), nil
}

func GetUserInfo(uri *url.URL) (*url.Userinfo, error) {
	var (
		usr *user.User
		err error
	)
	if u, found := os.LookupEnv("_CONTAINERS_ROOTLESS_UID"); found {
		usr, err = user.LookupId(u)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup rootless user: %w", err)
		}
	} else {
		usr, err = user.Current()
		if err != nil {
			return nil, fmt.Errorf("failed to obtain current user: %w", err)
		}
	}

	pw, set := uri.User.Password()
	if set {
		return url.UserPassword(usr.Username, pw), nil
	}
	return url.User(usr.Username), nil
}

// ValidateAndConfigure will take a ssh url and an identity key (rsa and the like) and ensure the information given is valid
// iden iden can be blank to mean no identity key
// once the function validates the information it creates and returns an ssh.ClientConfig.
func ValidateAndConfigure(uri *url.URL, iden string) (*ssh.ClientConfig, error) {
	var signers []ssh.Signer
	passwd, passwdSet := uri.User.Password()
	if iden != "" { // iden might be blank if coming from image scp or if no validation is needed
		value := iden
		s, err := PublicKey(value, []byte(passwd))
		if err != nil {
			return nil, fmt.Errorf("failed to read identity %q: %w", value, err)
		}
		signers = append(signers, s)
		logrus.Debugf("SSH Ident Key %q %s %s", value, ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
	} else if sock, found := os.LookupEnv("SSH_AUTH_SOCK"); found { // validate ssh information, specifically the unix file socket used by the ssh agent.
		logrus.Debugf("Found SSH_AUTH_SOCK %q, ssh-agent signer enabled", sock)

		c, err := net.Dial("unix", sock)
		if err != nil {
			return nil, err
		}
		agentSigners, err := agent.NewClient(c).Signers()
		if err != nil {
			return nil, err
		}

		signers = append(signers, agentSigners...)

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			for _, s := range agentSigners {
				logrus.Debugf("SSH Agent Key %s %s", ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
			}
		}
	}
	var authMethods []ssh.AuthMethod // now we validate and check for the authorization methods, most notaibly public key authorization
	if len(signers) > 0 {
		dedup := make(map[string]ssh.Signer)
		for _, s := range signers {
			fp := ssh.FingerprintSHA256(s.PublicKey())
			if _, found := dedup[fp]; found {
				logrus.Debugf("Dedup SSH Key %s %s", ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
			}
			dedup[fp] = s
		}

		var uniq []ssh.Signer
		for _, s := range dedup {
			uniq = append(uniq, s)
		}
		authMethods = append(authMethods, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return uniq, nil
		}))
	}
	if passwdSet { // if password authentication is given and valid, add to the list
		authMethods = append(authMethods, ssh.Password(passwd))
	}
	if len(authMethods) == 0 {
		authMethods = append(authMethods, ssh.PasswordCallback(func() (string, error) {
			pass, err := ReadPassword(fmt.Sprintf("%s's login password:", uri.User.Username()))
			return string(pass), err
		}))
	}
	tick, err := time.ParseDuration("40s")
	if err != nil {
		return nil, err
	}
	keyFilePath := filepath.Join(homedir.Get(), ".ssh", "known_hosts")
	known, err := knownhosts.New(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("creating host key callback function for %s: %w", keyFilePath, err)
	}

	cfg := &ssh.ClientConfig{
		User:            uri.User.Username(),
		Auth:            authMethods,
		HostKeyCallback: known,
		Timeout:         tick,
	}
	return cfg, nil
}

func getUDS(uri *url.URL, iden string) (string, error) {
	cfg, err := ValidateAndConfigure(uri, iden)
	if err != nil {
		return "", fmt.Errorf("failed to validate: %w", err)
	}
	dial, err := ssh.Dial("tcp", uri.Host, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to connect: %w", err)
	}
	defer dial.Close()

	session, err := dial.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create new ssh session on %q: %w", uri.Host, err)
	}
	defer session.Close()

	// Override podman binary for testing etc
	podman := "podman"
	if v, found := os.LookupEnv("PODMAN_BINARY"); found {
		podman = v
	}
	infoJSON, err := ExecRemoteCommand(dial, podman+" info --format=json")
	if err != nil {
		return "", err
	}

	var info Info
	if err := json.Unmarshal(infoJSON, &info); err != nil {
		return "", fmt.Errorf("failed to parse 'podman info' results: %w", err)
	}

	if info.Host.RemoteSocket == nil || len(info.Host.RemoteSocket.Path) == 0 {
		return "", fmt.Errorf("remote podman %q failed to report its UDS socket", uri.Host)
	}
	return info.Host.RemoteSocket.Path, nil
}
