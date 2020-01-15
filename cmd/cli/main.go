package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/crypto/ssh"
)

// remote PODMAN_HOST=ssh://<user>@<host>[:port]/run/podman/podman.sock
// local  PODMAN_HOST=unix://run/podman/podman.sock

var (
	DefaultURL = "unix://root@localhost/run/podman/podman.sock"
)

func main() {
	connectionURL := DefaultURL
	if value, found := os.LookupEnv("PODMAN_HOST"); found {
		connectionURL = value
	}

	_url, err := url.Parse(connectionURL)
	if err != nil {
		die("Value of PODMAN_HOST is not a valid url: %s\n", connectionURL)
	}

	if _url.Scheme != "ssh" && _url.Scheme != "unix" {
		die("Scheme from PODMAN_HOST is not supported: %s\n", _url.Scheme)
	}

	// Now we setup the http client to use the connection above
	client := &http.Client{}
	if _url.Scheme == "ssh" {
		var auth ssh.AuthMethod
		if value, found := os.LookupEnv("PODMAN_SSHKEY"); found {
			auth, err = publicKey(value)
			if err != nil {
				die("Failed to parse %s: %v\n", value, err)
			}
		} else {
			die("PODMAN_SSHKEY was not defined\n")
		}

		// Connect to sshd
		bastion, err := ssh.Dial("tcp",
			net.JoinHostPort(_url.Hostname(), _url.Port()),
			&ssh.ClientConfig{
				User:            _url.User.Username(),
				Auth:            []ssh.AuthMethod{auth},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		)
		if err != nil {
			die("Failed to build ssh tunnel")
		}
		defer bastion.Close()

		client.Transport = &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				// Now we make the connection to the unix domain socket on the server using the ssh tunnel
				return bastion.Dial("unix", _url.Path)
			},
		}
	} else {
		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "unix", _url.Path)
			},
			DisableCompression: true,
		}
	}

	resp, err := client.Get("http://localhost/v1.24/images/json")
	if err != nil {
		die(err.Error())
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var output bytes.Buffer
	_ = json.Indent(&output, body, "", "  ")
	fmt.Printf("%s\n", output.String())
	os.Exit(0)
}

func die(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

func publicKey(path string) (ssh.AuthMethod, error) {
	key, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(signer), nil
}
