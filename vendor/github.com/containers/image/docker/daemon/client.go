package daemon

import (
	"net/http"
	"path/filepath"

	"github.com/containers/image/types"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
)

const (
	// The default API version to be used in case none is explicitly specified
	defaultAPIVersion = "1.22"
)

// NewDockerClient initializes a new API client based on the passed SystemContext.
func newDockerClient(ctx *types.SystemContext) (*dockerclient.Client, error) {
	host := dockerclient.DefaultDockerHost
	if ctx != nil && ctx.DockerDaemonHost != "" {
		host = ctx.DockerDaemonHost
	}

	// Sadly, unix:// sockets don't work transparently with dockerclient.NewClient.
	// They work fine with a nil httpClient; with a non-nil httpClient, the transport’s
	// TLSClientConfig must be nil (or the client will try using HTTPS over the PF_UNIX socket
	// regardless of the values in the *tls.Config), and we would have to call sockets.ConfigureTransport.
	//
	// We don't really want to configure anything for unix:// sockets, so just pass a nil *http.Client.
	proto, _, _, err := dockerclient.ParseHost(host)
	if err != nil {
		return nil, err
	}
	var httpClient *http.Client
	if proto != "unix" {
		hc, err := tlsConfig(ctx)
		if err != nil {
			return nil, err
		}
		httpClient = hc
	}

	return dockerclient.NewClient(host, defaultAPIVersion, httpClient, nil)
}

func tlsConfig(ctx *types.SystemContext) (*http.Client, error) {
	options := tlsconfig.Options{}
	if ctx != nil && ctx.DockerDaemonInsecureSkipTLSVerify {
		options.InsecureSkipVerify = true
	}

	if ctx != nil && ctx.DockerDaemonCertPath != "" {
		options.CAFile = filepath.Join(ctx.DockerDaemonCertPath, "ca.pem")
		options.CertFile = filepath.Join(ctx.DockerDaemonCertPath, "cert.pem")
		options.KeyFile = filepath.Join(ctx.DockerDaemonCertPath, "key.pem")
	}

	tlsc, err := tlsconfig.Client(options)
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsc,
		},
		CheckRedirect: dockerclient.CheckRedirect,
	}, nil
}
