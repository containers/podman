package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type dispatcher interface {
	VarlinkDispatch(c Call, methodname string) error
	VarlinkGetName() string
	VarlinkGetDescription() string
}

type serviceCall struct {
	Method     string           `json:"method"`
	Parameters *json.RawMessage `json:"parameters,omitempty"`
	More       bool             `json:"more,omitempty"`
	Oneway     bool             `json:"oneway,omitempty"`
	Upgrade    bool             `json:"upgrade,omitempty"`
}

type serviceReply struct {
	Parameters interface{} `json:"parameters,omitempty"`
	Continues  bool        `json:"continues,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// Service represents an active varlink service. In addition to the registered custom varlink Interfaces, every service
// implements the org.varlink.service interface which allows clients to retrieve information about the
// running service.
type Service struct {
	vendor       string
	product      string
	version      string
	url          string
	interfaces   map[string]dispatcher
	names        []string
	descriptions map[string]string
	running      bool
	listener     net.Listener
	conncounter  int64
	mutex        sync.Mutex
	protocol     string
	address      string
}

// ServiceTimoutError helps API users to special-case timeouts.
type ServiceTimeoutError struct{}

func (ServiceTimeoutError) Error() string {
	return "service timeout"
}

func (s *Service) getInfo(c Call) error {
	return c.replyGetInfo(s.vendor, s.product, s.version, s.url, s.names)
}

func (s *Service) getInterfaceDescription(c Call, name string) error {
	if name == "" {
		return c.ReplyInvalidParameter("interface")
	}

	description, ok := s.descriptions[name]
	if !ok {
		return c.ReplyInvalidParameter("interface")
	}

	return c.replyGetInterfaceDescription(description)
}

func (s *Service) HandleMessage(conn *net.Conn, reader *bufio.Reader, writer *bufio.Writer, request []byte) error {
	var in serviceCall

	err := json.Unmarshal(request, &in)

	if err != nil {
		return err
	}

	c := Call{
		Conn:    conn,
		Reader:  reader,
		Writer:  writer,
		In:      &in,
		Request: &request,
	}

	r := strings.LastIndex(in.Method, ".")
	if r <= 0 {
		return c.ReplyInvalidParameter("method")
	}

	interfacename := in.Method[:r]
	methodname := in.Method[r+1:]

	if interfacename == "org.varlink.service" {
		return s.orgvarlinkserviceDispatch(c, methodname)
	}

	// Find the interface and method in our service
	iface, ok := s.interfaces[interfacename]
	if !ok {
		return c.ReplyInterfaceNotFound(interfacename)
	}

	return iface.VarlinkDispatch(c, methodname)
}

// Shutdown shuts down the listener of a running service.
func (s *Service) Shutdown() {
	s.running = false
	s.mutex.Lock()
	if s.listener != nil {
		s.listener.Close()
	}
	s.mutex.Unlock()
}

func (s *Service) handleConnection(conn net.Conn, wg *sync.WaitGroup) {
	defer func() { s.mutex.Lock(); s.conncounter--; s.mutex.Unlock(); wg.Done() }()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		request, err := reader.ReadBytes('\x00')
		if err != nil {
			break
		}

		err = s.HandleMessage(&conn, reader, writer, request[:len(request)-1])
		if err != nil {
			// FIXME: report error
			//fmt.Fprintf(os.Stderr, "handleMessage: %v", err)
			break
		}
	}

	conn.Close()
}

func (s *Service) teardown() {
	s.mutex.Lock()
	s.listener = nil
	s.running = false
	s.protocol = ""
	s.address = ""
	s.mutex.Unlock()
}

func (s *Service) parseAddress(address string) error {
	words := strings.SplitN(address, ":", 2)
	if len(words) != 2 {
		return fmt.Errorf("Unknown protocol")
	}

	s.protocol = words[0]
	s.address = words[1]

	// Ignore parameters after ';'
	words = strings.SplitN(s.address, ";", 2)
	if words != nil {
		s.address = words[0]
	}

	switch s.protocol {
	case "unix":
		break
	case "tcp":
		break

	default:
		return fmt.Errorf("Unknown protocol")
	}

	return nil
}

func (s *Service) GetListener() (*net.Listener, error) {
	s.mutex.Lock()
	l := s.listener
	s.mutex.Unlock()
	return &l, nil
}

func (s *Service) setListener() error {
	l := activationListener()
	if l == nil {
		if s.protocol == "unix" && s.address[0] != '@' {
			os.Remove(s.address)
		}

		var err error
		l, err = net.Listen(s.protocol, s.address)
		if err != nil {
			return err
		}

		if s.protocol == "unix" && s.address[0] != '@' {
			l.(*net.UnixListener).SetUnlinkOnClose(true)
		}
	}

	s.mutex.Lock()
	s.listener = l
	s.mutex.Unlock()

	return nil
}

func (s *Service) refreshTimeout(timeout time.Duration) error {
	switch l := s.listener.(type) {
	case *net.UnixListener:
		if err := l.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}
	case *net.TCPListener:
		if err := l.SetDeadline(time.Now().Add(timeout)); err != nil {
			return err
		}

	}
	return nil
}

// Listen starts a Service.
func (s *Service) Bind(address string) error {
	s.mutex.Lock()
	if s.running {
		s.mutex.Unlock()
		return fmt.Errorf("Init(): already running")
	}
	s.mutex.Unlock()

	s.parseAddress(address)

	err := s.setListener()
	if err != nil {
		return err
	}
	return nil
}

// Listen starts a Service.
func (s *Service) Listen(address string, timeout time.Duration) error {
	var wg sync.WaitGroup
	defer func() { s.teardown(); wg.Wait() }()

	err := s.Bind(address)
	if err != nil {
		return err
	}

	s.mutex.Lock()
	s.running = true
	l := s.listener
	s.mutex.Unlock()

	for s.running {
		if timeout != 0 {
			if err := s.refreshTimeout(timeout); err != nil {
				return err
			}
		}
		conn, err := l.Accept()
		if err != nil {
			if err.(net.Error).Timeout() {
				s.mutex.Lock()
				if s.conncounter == 0 {
					s.mutex.Unlock()
					return ServiceTimeoutError{}
				}
				s.mutex.Unlock()
				continue
			}
			if !s.running {
				return nil
			}
			return err
		}
		s.mutex.Lock()
		s.conncounter++
		s.mutex.Unlock()
		wg.Add(1)
		go s.handleConnection(conn, &wg)
	}

	return nil
}

// Listen starts a Service.
func (s *Service) DoListen(timeout time.Duration) error {
	var wg sync.WaitGroup
	defer func() { s.teardown(); wg.Wait() }()

	s.mutex.Lock()
	l := s.listener
	s.mutex.Unlock()

	if l == nil {
		return fmt.Errorf("No listener set")
	}

	s.mutex.Lock()
	s.running = true
	s.mutex.Unlock()

	for s.running {
		if timeout != 0 {
			if err := s.refreshTimeout(timeout); err != nil {
				return err
			}
		}
		conn, err := l.Accept()
		if err != nil {
			if err.(net.Error).Timeout() {
				s.mutex.Lock()
				if s.conncounter == 0 {
					s.mutex.Unlock()
					return ServiceTimeoutError{}
				}
				s.mutex.Unlock()
				continue
			}
			if !s.running {
				return nil
			}
			return err
		}
		s.mutex.Lock()
		s.conncounter++
		s.mutex.Unlock()
		wg.Add(1)
		go s.handleConnection(conn, &wg)
	}

	return nil
}

// RegisterInterface registers a varlink.Interface containing struct to the Service
func (s *Service) RegisterInterface(iface dispatcher) error {
	name := iface.VarlinkGetName()
	if _, ok := s.interfaces[name]; ok {
		return fmt.Errorf("interface '%s' already registered", name)
	}

	if s.running {
		return fmt.Errorf("service is already running")
	}
	s.interfaces[name] = iface
	s.descriptions[name] = iface.VarlinkGetDescription()
	s.names = append(s.names, name)

	return nil
}

// NewService creates a new Service which implements the list of given varlink interfaces.
func NewService(vendor string, product string, version string, url string) (*Service, error) {
	s := Service{
		vendor:       vendor,
		product:      product,
		version:      version,
		url:          url,
		interfaces:   make(map[string]dispatcher),
		descriptions: make(map[string]string),
	}
	err := s.RegisterInterface(orgvarlinkserviceNew())

	return &s, err
}
