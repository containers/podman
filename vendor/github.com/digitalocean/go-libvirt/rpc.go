// Copyright 2018 The go-libvirt Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package libvirt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/digitalocean/go-libvirt/internal/constants"
	"github.com/digitalocean/go-libvirt/internal/event"
	xdr "github.com/digitalocean/go-libvirt/internal/go-xdr/xdr2"
)

// ErrUnsupported is returned if a procedure is not supported by libvirt
var ErrUnsupported = errors.New("unsupported procedure requested")

// request and response types
const (
	// Call is used when making calls to the remote server.
	Call = iota

	// Reply indicates a server reply.
	Reply

	// Message is an asynchronous notification.
	Message

	// Stream represents a stream data packet.
	Stream

	// CallWithFDs is used by a client to indicate the request has
	// arguments with file descriptors.
	CallWithFDs

	// ReplyWithFDs is used by a server to indicate the request has
	// arguments with file descriptors.
	ReplyWithFDs
)

// request and response statuses
const (
	// StatusOK is always set for method calls or events.
	// For replies it indicates successful completion of the method.
	// For streams it indicates confirmation of the end of file on the stream.
	StatusOK = iota

	// StatusError for replies indicates that the method call failed
	// and error information is being returned. For streams this indicates
	// that not all data was sent and the stream has aborted.
	StatusError

	// StatusContinue is only used for streams.
	// This indicates that further data packets will be following.
	StatusContinue
)

// header is a libvirt rpc packet header
type header struct {
	// Program identifier
	Program uint32

	// Program version
	Version uint32

	// Remote procedure identifier
	Procedure uint32

	// Call type, e.g., Reply
	Type uint32

	// Call serial number
	Serial int32

	// Request status, e.g., StatusOK
	Status uint32
}

// packet represents a RPC request or response.
type packet struct {
	// Size of packet, in bytes, including length.
	// Len + Header + Payload
	Len    uint32
	Header header
}

// Global packet instance, for use with unsafe.Sizeof()
var _p packet

// internal rpc response
type response struct {
	Payload []byte
	Status  uint32
}

// libvirt error response
type libvirtError struct {
	Code     uint32
	DomainID uint32
	Padding  uint8
	Message  string
	Level    uint32
}

func (e libvirtError) Error() string {
	return e.Message
}

// checkError is used to check whether an error is a libvirtError, and if it is,
// whether its error code matches the one passed in. It will return false if
// these conditions are not met.
func checkError(err error, expectedError errorNumber) bool {
	e, ok := err.(libvirtError)
	if ok {
		return e.Code == uint32(expectedError)
	}
	return false
}

// IsNotFound detects libvirt's ERR_NO_DOMAIN.
func IsNotFound(err error) bool {
	return checkError(err, errNoDomain)
}

// listen processes incoming data and routes
// responses to their respective callback handler.
func (l *Libvirt) listen() {
	for {
		// response packet length
		length, err := pktlen(l.r)
		if err != nil {
			// When the underlying connection EOFs or is closed, stop
			// this goroutine
			if err == io.EOF || strings.Contains(err.Error(), "use of closed network connection") {
				return
			}

			// invalid packet
			continue
		}

		// response header
		h, err := extractHeader(l.r)
		if err != nil {
			// invalid packet
			continue
		}

		// payload: packet length minus what was previously read
		size := int(length) - int(unsafe.Sizeof(_p))
		buf := make([]byte, size)
		_, err = io.ReadFull(l.r, buf)
		if err != nil {
			// invalid packet
			continue
		}

		// route response to caller
		l.route(h, buf)
	}
}

// callback sends RPC responses to respective callers.
func (l *Libvirt) callback(id int32, res response) {
	l.cmux.Lock()
	defer l.cmux.Unlock()

	c, ok := l.callbacks[id]
	if !ok {
		return
	}

	c <- res
}

// route sends incoming packets to their listeners.
func (l *Libvirt) route(h *header, buf []byte) {
	// route events to their respective listener
	var event event.Event

	switch {
	case h.Program == constants.QEMUProgram && h.Procedure == constants.QEMUProcDomainMonitorEvent:
		event = &DomainEvent{}
	case h.Program == constants.Program && h.Procedure == constants.ProcDomainEventCallbackLifecycle:
		event = &DomainEventCallbackLifecycleMsg{}
	}

	if event != nil {
		err := eventDecoder(buf, event)
		if err != nil { // event was malformed, drop.
			return
		}

		l.stream(event)
		return
	}

	// send response to caller
	l.callback(h.Serial, response{Payload: buf, Status: h.Status})
}

// serial provides atomic access to the next sequential request serial number.
func (l *Libvirt) serial() int32 {
	return atomic.AddInt32(&l.s, 1)
}

// stream decodes and relays domain events to their respective listener.
func (l *Libvirt) stream(e event.Event) {
	l.emux.RLock()
	defer l.emux.RUnlock()

	q, ok := l.events[e.GetCallbackID()]
	if !ok {
		return
	}

	q.Push(e)
}

// addStream configures the routing for an event stream.
func (l *Libvirt) addStream(s *event.Stream) {
	l.emux.Lock()
	defer l.emux.Unlock()

	l.events[s.CallbackID] = s
}

// removeStream notifies the libvirt server to stop sending events for the
// provided callback ID. Upon successful de-registration the callback handler
// is destroyed. Subsequent calls to removeStream are idempotent and return
// nil.
// TODO: Fix this comment
func (l *Libvirt) removeStream(id int32) error {
	l.emux.Lock()
	defer l.emux.Unlock()

	// if the event is already removed, just return nil
	_, ok := l.events[id]
	if ok {
		delete(l.events, id)
	}

	return nil
}

// register configures a method response callback
func (l *Libvirt) register(id int32, c chan response) {
	l.cmux.Lock()
	defer l.cmux.Unlock()

	l.callbacks[id] = c
}

// deregister destroys a method response callback. It is the responsibility of
// the caller to manage locking (l.cmux) during this call.
func (l *Libvirt) deregister(id int32) {
	_, ok := l.callbacks[id]
	if !ok {
		return
	}

	close(l.callbacks[id])
	delete(l.callbacks, id)
}

// deregisterAll closes all waiting callback channels. This is used to clean up
// if the connection to libvirt is lost. Callers waiting for responses will
// return an error when the response channel is closed, rather than just
// hanging.
func (l *Libvirt) deregisterAll() {
	l.cmux.Lock()
	defer l.cmux.Unlock()

	for id := range l.callbacks {
		l.deregister(id)
	}
}

// request performs a libvirt RPC request.
// returns response returned by server.
// if response is not OK, decodes error from it and returns it.
func (l *Libvirt) request(proc uint32, program uint32, payload []byte) (response, error) {
	return l.requestStream(proc, program, payload, nil, nil)
}

// requestStream performs a libvirt RPC request. The `out` and `in` parameters
// are optional, and should be nil when RPC endpoints don't return a stream.
func (l *Libvirt) requestStream(proc uint32, program uint32, payload []byte,
	out io.Reader, in io.Writer) (response, error) {
	serial := l.serial()
	c := make(chan response)

	l.register(serial, c)
	defer func() {
		l.cmux.Lock()
		defer l.cmux.Unlock()

		l.deregister(serial)
	}()

	err := l.sendPacket(serial, proc, program, payload, Call, StatusOK)
	if err != nil {
		return response{}, err
	}

	resp, err := l.getResponse(c)
	if err != nil {
		return resp, err
	}

	if out != nil {
		abort := make(chan bool)
		outErr := make(chan error)
		go func() {
			outErr <- l.sendStream(serial, proc, program, out, abort)
		}()

		// Even without incoming stream server sends confirmation once all data is received
		resp, err = l.processIncomingStream(c, in)
		if err != nil {
			abort <- true
			return resp, err
		}

		err = <-outErr
		if err != nil {
			return response{}, err
		}
	}

	switch in {
	case nil:
		return resp, nil
	default:
		return l.processIncomingStream(c, in)
	}
}

// processIncomingStream is called once we've successfully sent a request to
// libvirt. It writes the responses back to the stream passed by the caller
// until libvirt sends a packet with statusOK or an error.
func (l *Libvirt) processIncomingStream(c chan response, inStream io.Writer) (response, error) {
	for {
		resp, err := l.getResponse(c)
		if err != nil {
			return resp, err
		}

		// StatusOK indicates end of stream
		if resp.Status == StatusOK {
			return resp, nil
		}

		// FIXME: this smells.
		// StatusError is handled in getResponse, so this must be StatusContinue
		// StatusContinue is only valid here for stream packets
		// libvirtd breaks protocol and returns StatusContinue with an
		// empty response Payload when the stream finishes
		if len(resp.Payload) == 0 {
			return resp, nil
		}
		if inStream != nil {
			_, err = inStream.Write(resp.Payload)
			if err != nil {
				return response{}, err
			}
		}
	}
}

func (l *Libvirt) sendStream(serial int32, proc uint32, program uint32, stream io.Reader, abort chan bool) error {
	// Keep total packet length under 4 MiB to follow possible limitation in libvirt server code
	buf := make([]byte, 4*MiB-unsafe.Sizeof(_p))
	for {
		select {
		case <-abort:
			return l.sendPacket(serial, proc, program, nil, Stream, StatusError)
		default:
		}
		n, err := stream.Read(buf)
		if n > 0 {
			err2 := l.sendPacket(serial, proc, program, buf[:n], Stream, StatusContinue)
			if err2 != nil {
				return err2
			}
		}
		if err != nil {
			if err == io.EOF {
				return l.sendPacket(serial, proc, program, nil, Stream, StatusOK)
			}
			// keep original error
			err2 := l.sendPacket(serial, proc, program, nil, Stream, StatusError)
			if err2 != nil {
				return err2
			}
			return err
		}
	}
}

func (l *Libvirt) sendPacket(serial int32, proc uint32, program uint32, payload []byte, typ uint32, status uint32) error {

	p := packet{
		Header: header{
			Program:   program,
			Version:   constants.ProtocolVersion,
			Procedure: proc,
			Type:      typ,
			Serial:    serial,
			Status:    status,
		},
	}

	size := int(unsafe.Sizeof(p.Len)) + int(unsafe.Sizeof(p.Header))
	if payload != nil {
		size += len(payload)
	}
	p.Len = uint32(size)

	// write header
	l.mu.Lock()
	defer l.mu.Unlock()
	err := binary.Write(l.w, binary.BigEndian, p)
	if err != nil {
		return err
	}

	// write payload
	if payload != nil {
		err = binary.Write(l.w, binary.BigEndian, payload)
		if err != nil {
			return err
		}
	}

	return l.w.Flush()
}

func (l *Libvirt) getResponse(c chan response) (response, error) {
	resp := <-c
	if resp.Status == StatusError {
		return resp, decodeError(resp.Payload)
	}

	return resp, nil
}

// encode XDR encodes the provided data.
func encode(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	_, err := xdr.Marshal(&buf, data)

	return buf.Bytes(), err
}

// decodeError extracts an error message from the provider buffer.
func decodeError(buf []byte) error {
	var e libvirtError

	dec := xdr.NewDecoder(bytes.NewReader(buf))
	_, err := dec.Decode(&e)
	if err != nil {
		return err
	}

	if strings.Contains(e.Message, "unknown procedure") {
		return ErrUnsupported
	}
	// if libvirt returns ERR_OK, ignore the error
	if checkError(e, errOk) {
		return nil
	}

	return e
}

// eventDecoder decodes an event from a xdr buffer.
func eventDecoder(buf []byte, e interface{}) error {
	dec := xdr.NewDecoder(bytes.NewReader(buf))
	_, err := dec.Decode(e)
	return err
}

// pktlen returns the length of an incoming RPC packet.  Read errors will
// result in a returned response length of 0 and a non-nil error.
func pktlen(r io.Reader) (uint32, error) {
	buf := make([]byte, unsafe.Sizeof(_p.Len))

	// extract the packet's length from the header
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(buf), nil
}

// extractHeader returns the decoded header from an incoming response.
func extractHeader(r io.Reader) (*header, error) {
	buf := make([]byte, unsafe.Sizeof(_p.Header))

	// extract the packet's header from r
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}

	return &header{
		Program:   binary.BigEndian.Uint32(buf[0:4]),
		Version:   binary.BigEndian.Uint32(buf[4:8]),
		Procedure: binary.BigEndian.Uint32(buf[8:12]),
		Type:      binary.BigEndian.Uint32(buf[12:16]),
		Serial:    int32(binary.BigEndian.Uint32(buf[16:20])),
		Status:    binary.BigEndian.Uint32(buf[20:24]),
	}, nil
}

type typedParamDecoder struct{}

// Decode decodes a TypedParam. These are part of the libvirt spec, and not xdr
// proper. TypedParams contain a name, which is called Field for some reason,
// and a Value, which itself has a "discriminant" - an integer enum encoding the
// actual type, and a value, the length of which varies based on the actual
// type.
func (tpd typedParamDecoder) Decode(d *xdr.Decoder, v reflect.Value) (int, error) {
	// Get the name of the typed param first
	name, n, err := d.DecodeString()
	if err != nil {
		return n, err
	}
	val, n2, err := tpd.decodeTypedParamValue(d)
	n += n2
	if err != nil {
		return n, err
	}
	tp := &TypedParam{Field: name, Value: *val}
	v.Set(reflect.ValueOf(*tp))

	return n, nil
}

// decodeTypedParamValue decodes the Value part of a TypedParam.
func (typedParamDecoder) decodeTypedParamValue(d *xdr.Decoder) (*TypedParamValue, int, error) {
	// All TypedParamValues begin with a uint32 discriminant that tells us what
	// type they are.
	discriminant, n, err := d.DecodeUint()
	if err != nil {
		return nil, n, err
	}
	var n2 int
	var tpv *TypedParamValue
	switch discriminant {
	case 1:
		var val int32
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 2:
		var val uint32
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 3:
		var val int64
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 4:
		var val uint64
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 5:
		var val float64
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 6:
		var val int32
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}
	case 7:
		var val string
		n2, err = d.Decode(&val)
		tpv = &TypedParamValue{D: discriminant, I: val}

	default:
		err = fmt.Errorf("invalid parameter type %v", discriminant)
	}
	n += n2

	return tpv, n, err
}
