package varlink

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
)

// Error is a varlink error returned from a method call.
type Error struct {
	Name       string
	Parameters interface{}
}

// Error returns the fully-qualified varlink error name.
func (e *Error) Error() string {
	return e.Name
}

// Connection is a connection from a client to a service.
type Connection struct {
	address string
	conn    net.Conn
	reader  *bufio.Reader
	writer  *bufio.Writer
}

// Send sends a method call.
func (c *Connection) Send(method string, parameters interface{}, more bool, oneway bool) error {
	type call struct {
		Method     string      `json:"method"`
		Parameters interface{} `json:"parameters,omitempty"`
		More       bool        `json:"more,omitempty"`
		Oneway     bool        `json:"oneway,omitempty"`
	}

	if more && oneway {
		return &Error{
			Name:       "org.varlink.InvalidParameter",
			Parameters: "oneway",
		}
	}

	m := call{
		Method:     method,
		Parameters: parameters,
		More:       more,
		Oneway:     oneway,
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}

	b = append(b, 0)
	_, err = c.writer.Write(b)
	if err != nil {
		return err
	}

	return c.writer.Flush()
}

// Receive receives a method reply.
func (c *Connection) Receive(parameters interface{}) (bool, error) {
	type reply struct {
		Parameters *json.RawMessage `json:"parameters"`
		Continues  bool             `json:"continues"`
		Error      string           `json:"error"`
	}

	out, err := c.reader.ReadBytes('\x00')
	if err != nil {
		return false, err
	}

	var m reply
	err = json.Unmarshal(out[:len(out)-1], &m)
	if err != nil {
		return false, err
	}

	if m.Error != "" {
		return false, &Error{
			Name:       m.Error,
			Parameters: m.Parameters,
		}
	}

	if parameters != nil && m.Parameters != nil {
		return m.Continues, json.Unmarshal(*m.Parameters, parameters)
	}

	return m.Continues, nil
}

// Call sends a method call and returns the result of the call.
func (c *Connection) Call(method string, parameters interface{}, result interface{}) error {
	err := c.Send(method, &parameters, false, false)
	if err != nil {
		return err
	}

	_, err = c.Receive(result)
	return err
}

// GetInterfaceDescription requests the interface description string from the service.
func (c *Connection) GetInterfaceDescription(name string) (string, error) {
	type request struct {
		Interface string `json:"interface"`
	}
	type reply struct {
		Description string `json:"description"`
	}

	var r reply
	err := c.Call("org.varlink.service.GetInterfaceDescription", request{Interface: name}, &r)
	if err != nil {
		return "", err
	}

	return r.Description, nil
}

// GetInfo requests information about the service.
func (c *Connection) GetInfo(vendor *string, product *string, version *string, url *string, interfaces *[]string) error {
	type reply struct {
		Vendor     string   `json:"vendor"`
		Product    string   `json:"product"`
		Version    string   `json:"version"`
		URL        string   `json:"url"`
		Interfaces []string `json:"interfaces"`
	}

	var r reply
	err := c.Call("org.varlink.service.GetInfo", nil, &r)
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

// Close terminates the connection.
func (c *Connection) Close() error {
	return c.conn.Close()
}

// NewConnection returns a new connection to the given address.
func NewConnection(address string) (*Connection, error) {
	var err error

	words := strings.SplitN(address, ":", 2)
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
	c.conn, err = net.Dial(protocol, addr)
	if err != nil {
		return nil, err
	}

	c.address = address
	c.reader = bufio.NewReader(c.conn)
	c.writer = bufio.NewWriter(c.conn)

	return &c, nil
}
