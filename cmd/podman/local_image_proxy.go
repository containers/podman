package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/containers/image/v5/pkg/blobinfocache"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/containers/podman/v3/pkg/registries"
	"github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
)

type localImageProxyOptions struct {
	tlsVerify bool
	// sockFd is file descriptor for a socketpair()
	sockFd int
	// portNum is a port to use for TCP
	portNum int
}

var (
	opts                   localImageProxyOptions
	localImageProxyCommand = &cobra.Command{
		Use:     "local-image-proxy [options]",
		Short:   "Interactive proxy for fetching container images",
		Long:    "Run a local webserver that proxies OCI Distribution HTTP requests to fetch manifests and blobs",
		RunE:    runLocalImageProxy,
		Args:    cobra.MaximumNArgs(0),
		Example: `podman local-image-proxy --sockfd N`,
	}
)

func init() {
	// Note that the local and the remote client behave the same: both
	// store credentials locally while the remote client will pass them
	// over the wire to the endpoint.
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
		Command: localImageProxyCommand,
	})
	flags := localImageProxyCommand.Flags()

	// Podman flags.
	flags.BoolVarP(&opts.tlsVerify, "tls-verify", "", false, "Require HTTPS and verify certificates when contacting registries")
	flags.IntVar(&opts.sockFd, "sockfd", -1, "Serve on opened socket pair")
	flags.IntVar(&opts.portNum, "port", -1, "Serve on TCP port (localhost)")
	loginOptions.Stdin = os.Stdin
	loginOptions.Stdout = os.Stdout
	loginOptions.AcceptUnspecifiedRegistry = true
}

type proxyHandler struct {
	transport types.ImageTransport
	cache     types.BlobInfoCache
	sysctx    *types.SystemContext
}

func (h *proxyHandler) implRequest(w http.ResponseWriter, imgname, reqtype, ref string) error {
	ctx := context.TODO()
	imgref, err := h.transport.ParseReference(imgname)
	if err != nil {
		return err
	}
	imgsrc, err := imgref.NewImageSource(ctx, h.sysctx)
	if err != nil {
		return err
	}
	if reqtype == "manifests" {
		rawManifest, _, err := imgsrc.GetManifest(ctx, nil)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(rawManifest)))
		r := bytes.NewReader(rawManifest)
		_, err = io.Copy(w, r)
		if err != nil {
			return err
		}
	} else if reqtype == "blobs" {
		d, err := digest.Parse(ref)
		if err != nil {
			return err
		}
		r, blobSize, err := imgsrc.GetBlob(ctx, types.BlobInfo{Digest: d, Size: -1}, h.cache)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", blobSize))
		_, err = io.Copy(w, r)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("unhandled request %s", reqtype)
	}

	return nil
}

// ServeHTTP handles two requests:
//
// GET /<host>/<name>/manifests/<reference>
// GET /<host>/<name>/blobs/<digest>
func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path == "" || !strings.HasPrefix(r.URL.Path, "/") {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 6 {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	imgref := fmt.Sprintf("//%s/%s/%s", parts[1], parts[2], parts[3])
	reqtype := parts[4]
	ref := parts[5]

	err := h.implRequest(w, imgref, reqtype, ref)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}

// Implementation of podman local-image-proxy
func runLocalImageProxy(cmd *cobra.Command, args []string) error {
	var skipTLS types.OptionalBool

	if cmd.Flags().Changed("tls-verify") {
		skipTLS = types.NewOptionalBool(!loginOptions.tlsVerify)
	}

	sysCtx := &types.SystemContext{
		DockerInsecureSkipTLSVerify: skipTLS,
		SystemRegistriesConfPath:    registries.SystemRegistriesConfPath(),
	}
	if opts.sockFd == -1 && opts.portNum == -1 {
		return fmt.Errorf("expected --sockfd or --port")
	}
	var err error
	var listener net.Listener
	if opts.sockFd != -1 {
		fdnum, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse datafd %s: %w", args[1], err)
		}
		fd := os.NewFile(uintptr(fdnum), "sock")
		defer fd.Close()

		listener, err = net.FileListener(fd)
		if err != nil {
			return err
		}
	} else {
		addr := net.TCPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: opts.portNum,
			Zone: "",
		}
		listener, err = net.ListenTCP("tcp", &addr)
		if err != nil {
			return err
		}
	}
	defer listener.Close()

	handler := &proxyHandler{
		transport: transports.Get("docker"),
		cache:     blobinfocache.DefaultCache(sysCtx),
		sysctx:    sysCtx,
	}

	srv := &http.Server{
		Handler: handler,
	}
	return srv.Serve(listener)
}
