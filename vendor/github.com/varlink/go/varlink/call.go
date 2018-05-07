package varlink

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
)

// Call is a method call retrieved by a Service. The connection from the
// client can be terminated by returning an error from the call instead
// of sending a reply or error reply.
type Call struct {
	writer    *bufio.Writer
	in        *serviceCall
	Continues bool
}

// WantsMore indicates if the calling client accepts more than one reply to this method call.
func (c *Call) WantsMore() bool {
	return c.in.More
}

// IsOneway indicate that the calling client does not expect a reply.
func (c *Call) IsOneway() bool {
	return c.in.Oneway
}

// GetParameters retrieves the method call parameters.
func (c *Call) GetParameters(p interface{}) error {
	if c.in.Parameters == nil {
		return fmt.Errorf("empty parameters")
	}
	return json.Unmarshal(*c.in.Parameters, p)
}

func (c *Call) sendMessage(r *serviceReply) error {
	if c.in.Oneway {
		return nil
	}

	b, e := json.Marshal(r)
	if e != nil {
		return e
	}

	b = append(b, 0)
	_, e = c.writer.Write(b)
	if e != nil {
		return e
	}
	return c.writer.Flush()
}

// Reply sends a reply to this method call.
func (c *Call) Reply(parameters interface{}) error {
	if !c.Continues {
		return c.sendMessage(&serviceReply{
			Parameters: parameters,
		})
	}

	if !c.in.More {
		return fmt.Errorf("call did not set more, it does not expect continues")
	}

	return c.sendMessage(&serviceReply{
		Continues:  true,
		Parameters: parameters,
	})
}

// ReplyError sends an error reply to this method call.
func (c *Call) ReplyError(name string, parameters interface{}) error {
	r := strings.LastIndex(name, ".")
	if r <= 0 {
		return fmt.Errorf("invalid error name")
	}
	if name[:r] == "org.varlink.service" {
		return fmt.Errorf("refused to send org.varlink.service errors")
	}
	return c.sendMessage(&serviceReply{
		Error:      name,
		Parameters: parameters,
	})
}
