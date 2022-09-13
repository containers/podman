package varlink

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Call is a method call retrieved by a Service. The connection from the
// client can be terminated by returning an error from the call instead
// of sending a reply or error reply.
type Call struct {
	Conn      ReadWriterContext
	Request   *[]byte
	In        *serviceCall
	Continues bool
	Upgrade   bool
}

// WantsMore indicates if the calling client accepts more than one reply to this method call.
func (c *Call) WantsMore() bool {
	return c.In.More
}

// WantsUpgrade indicates that the calling client wants the connection to be upgraded.
func (c *Call) WantsUpgrade() bool {
	return c.In.Upgrade
}

// IsOneway indicate that the calling client does not expect a reply.
func (c *Call) IsOneway() bool {
	return c.In.Oneway
}

// GetParameters retrieves the method call parameters.
func (c *Call) GetParameters(p interface{}) error {
	if c.In.Parameters == nil {
		return fmt.Errorf("empty parameters")
	}
	return json.Unmarshal(*c.In.Parameters, p)
}

func (c *Call) sendMessage(ctx context.Context, r *serviceReply) error {
	if c.In.Oneway {
		return nil
	}

	b, err := json.Marshal(r)
	if err != nil {
		return err
	}

	b = append(b, 0)

	_, err = c.Conn.Write(ctx, b)
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// Reply sends a reply to this method call.
func (c *Call) Reply(ctx context.Context, parameters interface{}) error {
	if !c.Continues {
		return c.sendMessage(ctx, &serviceReply{
			Parameters: parameters,
		})
	}

	if !c.In.More {
		return fmt.Errorf("call did not set more, it does not expect continues")
	}

	return c.sendMessage(ctx, &serviceReply{
		Continues:  true,
		Parameters: parameters,
	})
}

// ReplyError sends an error reply to this method call.
func (c *Call) ReplyError(ctx context.Context, name string, parameters interface{}) error {
	r := strings.LastIndex(name, ".")
	if r <= 0 {
		return fmt.Errorf("invalid error name")
	}
	if name[:r] == "org.varlink.service" {
		return fmt.Errorf("refused to send org.varlink.service errors")
	}
	return c.sendMessage(ctx, &serviceReply{
		Error:      name,
		Parameters: parameters,
	})
}
