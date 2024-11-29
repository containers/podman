package rest

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"syscall"

	"github.com/crc-org/vfkit/pkg/cmdline"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// see `man unix`:
// UNIX-domain addresses are variable-length filesystem pathnames of at most 104 characters.
func maxSocketPathLen() int {
	var sockaddr syscall.RawSockaddrUnix
	// sockaddr.Path must end with '\0', it's not relevant for go strings
	return len(sockaddr.Path) - 1
}

type Endpoint struct {
	Host   string
	Path   string
	Scheme ServiceScheme
}

func NewEndpoint(input string) (*Endpoint, error) {
	uri, err := parseRestfulURI(input)
	if err != nil {
		return nil, err
	}
	scheme, err := toRestScheme(uri.Scheme)
	if err != nil {
		return nil, err
	}
	return &Endpoint{
		Host:   uri.Host,
		Path:   uri.Path,
		Scheme: scheme,
	}, nil
}

func (ep *Endpoint) ToCmdLine() ([]string, error) {
	args := []string{"--restful-uri"}
	switch ep.Scheme {
	case Unix:
		args = append(args, fmt.Sprintf("unix://%s", ep.Path))
	case TCP:
		args = append(args, fmt.Sprintf("tcp://%s%s", ep.Host, ep.Path))
	case None:
		return []string{}, nil
	default:
		return []string{}, errors.New("invalid endpoint scheme")
	}
	return args, nil
}

// VFKitService is used for the restful service; it describes
// the variables of the service like host/path but also has
// the router object
type VFKitService struct {
	*Endpoint
	router *gin.Engine
}

// Start initiates the already configured gin service
func (v *VFKitService) Start() {
	go func() {
		var err error
		switch v.Scheme {
		case TCP:
			err = v.router.Run(v.Host)
		case Unix:
			err = v.router.RunUnix(v.Path)
		}
		logrus.Fatal(err)
	}()
}

// NewServer creates a new restful service
func NewServer(inspector VirtualMachineInspector, stateHandler VirtualMachineStateHandler, endpoint string) (*VFKitService, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	ep, err := NewEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	err = r.SetTrustedProxies(nil)
	if err != nil {
		return nil, err
	}
	s := VFKitService{
		router:   r,
		Endpoint: ep,
	}

	// Handlers for the restful service.  This is where endpoints are defined.
	r.GET("/vm/state", stateHandler.GetVMState)
	r.POST("/vm/state", stateHandler.SetVMState)
	r.GET("/vm/inspect", inspector.Inspect)
	return &s, nil
}

type VirtualMachineInspector interface {
	Inspect(c *gin.Context)
}

type VirtualMachineStateHandler interface {
	GetVMState(c *gin.Context)
	SetVMState(c *gin.Context)
}

// parseRestfulURI validates the input URI and returns an URL object
func parseRestfulURI(inputURI string) (*url.URL, error) {
	restURI, err := url.ParseRequestURI(inputURI)
	if err != nil {
		return nil, err
	}
	scheme, err := toRestScheme(restURI.Scheme)
	if err != nil {
		return nil, err
	}
	if scheme == TCP && len(restURI.Host) < 1 {
		return nil, errors.New("invalid TCP uri: missing host")
	}
	if scheme == TCP && len(restURI.Path) > 0 {
		return nil, errors.New("invalid TCP uri: path is forbidden")
	}
	if scheme == TCP && restURI.Port() == "" {
		return nil, errors.New("invalid TCP uri: missing port")
	}
	if scheme == Unix && len(restURI.Path) < 1 {
		return nil, errors.New("invalid unix uri: missing path")
	}
	if scheme == Unix && len(restURI.Host) > 0 {
		return nil, errors.New("invalid unix uri: host is forbidden")
	}
	if scheme == Unix && len(restURI.Path) > maxSocketPathLen() {
		return nil, fmt.Errorf("invalid unix uri: socket path length exceeds macOS limits")
	}
	return restURI, err
}

// toRestScheme converts a string to a ServiceScheme
func toRestScheme(s string) (ServiceScheme, error) {
	switch strings.ToUpper(s) {
	case "NONE":
		return None, nil
	case "UNIX":
		return Unix, nil
	case "TCP", "HTTP":
		return TCP, nil
	}
	return None, fmt.Errorf("invalid scheme %s", s)
}

func validateRestfulURI(inputURI string) error {
	if inputURI != cmdline.DefaultRestfulURI {
		if _, err := parseRestfulURI(inputURI); err != nil {
			return err
		}
	}
	return nil
}
