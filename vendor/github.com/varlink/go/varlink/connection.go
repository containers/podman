package varlink

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/varlink/go/varlink/internal/ctxio"
)

// Message flags for Send(). More indicates that the client accepts more than one method
// reply to this call. Oneway requests, that the service must not send a method reply to
// this call. Continues indicates that the service will send more than one reply.
const (
	More      = 1 << iota
	Oneway    = 1 << iota
	Continues = 1 << iota
	Upgrade   = 1 << iota
)

// Error is a varlink error returned from a method call.
type Error struct {
	Name       string
	Parameters interface{}
}

func (e *Error) DispatchError() error {
	errorRawParameters := e.Parameters.(*json.RawMessage)

	switch e.Name {
	case "org.varlink.service.InterfaceNotFound":
		var param InterfaceNotFound
		if errorRawParameters != nil {
			err := json.Unmarshal(*errorRawParameters, &param)
			if err != nil {
				return e
			}
		}
		return &param
	case "org.varlink.service.MethodNotFound":
		var param MethodNotFound
		if errorRawParameters != nil {
			err := json.Unmarshal(*errorRawParameters, &param)
			if err != nil {
				return e
			}
		}
		return &param
	case "org.varlink.service.MethodNotImplemented":
		var param MethodNotImplemented
		if errorRawParameters != nil {
			err := json.Unmarshal(*errorRawParameters, &param)
			if err != nil {
				return e
			}
		}
		return &param
	case "org.varlink.service.InvalidParameter":
		var param InvalidParameter
		if errorRawParameters != nil {
			err := json.Unmarshal(*errorRawParameters, &param)
			if err != nil {
				return e
			}
		}
		return &param
	}
	return e
}

// Error returns the fully-qualified varlink error name.
func (e *Error) Error() string {
	return e.Name
}

// ReadWriterContext describes the capabilities of the
// underlying varlink connection.
type ReadWriterContext interface {
	Write(context.Context, []byte) (int, error)
	Read(context.Context, []byte) (int, error)
	ReadBytes(ctx context.Context, delim byte) ([]byte, error)
}

// Connection is a connection from a client to a service.
type Connection struct {
	io.Closer
	address string
	conn    *ctxio.Conn
}

// Send sends a method call. It returns a receive() function which is called to retrieve the method reply.
// If Send() is called with the `More` flag and the receive() function carries the `Continues` flag, receive()
// can be called multiple times to retrieve multiple replies.
func (c *Connection) Send(ctx context.Context, method string, parameters interface{}, flags uint64) (func(context.Context, interface{}) (uint64, error), error) {
	type call struct {
		Method     string      `json:"method"`
		Parameters interface{} `json:"parameters,omitempty"`
		More       bool        `json:"more,omitempty"`
		Oneway     bool        `json:"oneway,omitempty"`
		Upgrade    bool        `json:"upgrade,omitempty"`
	}

	if (flags&More != 0) && (flags&Oneway != 0) {
		return nil, &Error{
			Name:       "org.varlink.InvalidParameter",
			Parameters: "oneway",
		}
	}

	if (flags&More != 0) && (flags&Upgrade != 0) {
		return nil, &Error{
			Name:       "org.varlink.InvalidParameter",
			Parameters: "more",
		}
	}

	m := call{
		Method:     method,
		Parameters: parameters,
		More:       flags&More != 0,
		Oneway:     flags&Oneway != 0,
		Upgrade:    flags&Upgrade != 0,
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	b = append(b, 0)

	_, err = c.conn.Write(ctx, b)
	if err != nil {
		if err == io.EOF {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}

	receive := func(ctx context.Context, outParameters interface{}) (uint64, error) {
		type reply struct {
			Parameters *json.RawMessage `json:"parameters"`
			Continues  bool             `json:"continues"`
			Error      string           `json:"error"`
		}

		out, err := c.conn.ReadBytes(ctx, '\x00')
		if err != nil {
			if err == io.EOF {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}

		var m reply
		err = json.Unmarshal(out[:len(out)-1], &m)
		if err != nil {
			return 0, err
		}

		if m.Error != "" {
			e := &Error{
				Name:       m.Error,
				Parameters: m.Parameters,
			}
			return 0, e.DispatchError()
		}

		if m.Parameters != nil {
			json.Unmarshal(*m.Parameters, outParameters)
		}

		if m.Continues {
			return Continues, nil
		}

		return 0, nil
	}

	return receive, nil
}

// Call sends a method call and returns the method reply.
func (c *Connection) Call(ctx context.Context, method string, parameters interface{}, outParameters interface{}) error {
	receive, err := c.Send(ctx, method, &parameters, 0)
	if err != nil {
		return err
	}

	_, err = receive(ctx, outParameters)
	return err
}

// GetInterfaceDescription requests the interface description string from the service.
func (c *Connection) GetInterfaceDescription(ctx context.Context, name string) (string, error) {
	type request struct {
		Interface string `json:"interface"`
	}
	type reply struct {
		Description string `json:"description"`
	}

	var r reply
	err := c.Call(ctx, "org.varlink.service.GetInterfaceDescription", request{Interface: name}, &r)
	if err != nil {
		return "", err
	}

	return r.Description, nil
}

// GetInfo requests information about the service.
func (c *Connection) GetInfo(ctx context.Context, vendor *string, product *string, version *string, url *string, interfaces *[]string) error {
	type reply struct {
		Vendor     string   `json:"vendor"`
		Product    string   `json:"product"`
		Version    string   `json:"version"`
		URL        string   `json:"url"`
		Interfaces []string `json:"interfaces"`
	}

	var r reply
	err := c.Call(ctx, "org.varlink.service.GetInfo", nil, &r)
	if err != nil {
		return err
	}

	if vendor != nil {
		*vendor = r.Vendor
	}
	if product != nil {
		*product = r.Product
	}
	if version != nil {
		*version = r.Version
	}
	if url != nil {
		*url = r.URL
	}
	if interfaces != nil {
		*interfaces = r.Interfaces
	}

	return nil
}

// Upgrade attempts to upgrade the connection using the provided method and parameters.
// If successful, the connection cannot be reused later, and must be closed.
func (c *Connection) Upgrade(ctx context.Context, method string, parameters interface{}) (func(context.Context, interface{}) (uint64, ReadWriterContext, error), error) {
	reply, err := c.Send(ctx, method, parameters, Upgrade)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, out interface{}) (uint64, ReadWriterContext, error) {
		flags, err := reply(ctx, out)
		if err != nil {
			return 0, nil, err
		}

		return flags, c.conn, nil
	}, nil
}

// Close terminates the connection.
func (c *Connection) Close() error {
	return c.conn.Close()
}

// NewConnection returns a new connection to the given address. The context
// is used when dialling. Once successfully connected, any expiration
// of the context will not affect the connection.
func NewConnection(ctx context.Context, address string) (*Connection, error) {
	words := strings.SplitN(address, ":", 2)

	if len(words) != 2 {
		return nil, fmt.Errorf("Protocol missing")
	}

	protocol := words[0]
	addr := words[1]

	// Ignore parameters after ';'
	words = strings.SplitN(addr, ";", 2)
	if words != nil {
		addr = words[0]
	}

	switch protocol {
	case "unix":
		break

	case "tcp":
		break
	}

	c := Connection{}
	var d net.Dialer
	conn, err := d.DialContext(ctx, protocol, addr)
	if err != nil {
		return nil, err
	}

	c.address = address
	c.conn = ctxio.NewConn(conn)

	return &c, nil
}
