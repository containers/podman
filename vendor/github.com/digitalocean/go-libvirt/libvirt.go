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

// Package libvirt is a pure Go implementation of the libvirt RPC protocol.
// For more information on the protocol, see https://libvirt.org/internals/l.html
package libvirt

// We'll use c-for-go to extract the consts and typedefs from the libvirt
// sources so we don't have to duplicate them here.
//go:generate scripts/gen-consts.sh

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/digitalocean/go-libvirt/internal/constants"
	"github.com/digitalocean/go-libvirt/internal/event"
	xdr "github.com/digitalocean/go-libvirt/internal/go-xdr/xdr2"
)

// ErrEventsNotSupported is returned by Events() if event streams
// are unsupported by either QEMU or libvirt.
var ErrEventsNotSupported = errors.New("event monitor is not supported")

// Libvirt implements libvirt's remote procedure call protocol.
type Libvirt struct {
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
	mu   *sync.Mutex

	// method callbacks
	cmux      sync.RWMutex
	callbacks map[int32]chan response

	// event listeners
	emux   sync.RWMutex
	events map[int32]*event.Stream

	// next request serial number
	s int32
}

// DomainEvent represents a libvirt domain event.
type DomainEvent struct {
	CallbackID   int32
	Domain       Domain
	Event        string
	Seconds      uint64
	Microseconds uint32
	Padding      uint8
	Details      []byte
}

// GetCallbackID returns the callback ID of a QEMU domain event.
func (de DomainEvent) GetCallbackID() int32 {
	return de.CallbackID
}

// GetCallbackID returns the callback ID of a libvirt lifecycle event.
func (m DomainEventCallbackLifecycleMsg) GetCallbackID() int32 {
	return m.CallbackID
}

// qemuError represents a QEMU process error.
type qemuError struct {
	Error struct {
		Class       string `json:"class"`
		Description string `json:"desc"`
	} `json:"error"`
}

// Capabilities returns an XML document describing the host's capabilties.
func (l *Libvirt) Capabilities() ([]byte, error) {
	caps, err := l.ConnectGetCapabilities()
	return []byte(caps), err
}

// Connect establishes communication with the libvirt server.
// The underlying libvirt socket connection must be previously established.
func (l *Libvirt) Connect() error {
	payload := struct {
		Padding [3]byte
		Name    string
		Flags   uint32
	}{
		Padding: [3]byte{0x1, 0x0, 0x0},
		Name:    "qemu:///system",
		Flags:   0,
	}

	buf, err := encode(&payload)
	if err != nil {
		return err
	}

	// libvirt requires that we call auth-list prior to connecting,
	// event when no authentication is used.
	_, err = l.request(constants.ProcAuthList, constants.Program, buf)
	if err != nil {
		return err
	}

	_, err = l.request(constants.ProcConnectOpen, constants.Program, buf)
	if err != nil {
		return err
	}

	return nil
}

// Disconnect shuts down communication with the libvirt server and closes the
// underlying net.Conn.
func (l *Libvirt) Disconnect() error {
	// close event streams
	for _, ev := range l.events {
		l.unsubscribeEvents(ev)
	}

	// Deregister all callbacks to prevent blocking on clients with
	// outstanding requests
	l.deregisterAll()

	_, err := l.request(constants.ProcConnectClose, constants.Program, nil)
	if err != nil {
		return err
	}

	return l.conn.Close()
}

// Domains returns a list of all domains managed by libvirt.
//
// Deprecated: use ConnectListAllDomains instead.
func (l *Libvirt) Domains() ([]Domain, error) {
	// these are the flags as passed by `virsh list --all`
	flags := ConnectListDomainsActive | ConnectListDomainsInactive
	domains, _, err := l.ConnectListAllDomains(1, flags)
	return domains, err
}

// DomainState returns state of the domain managed by libvirt.
//
// Deprecated: use DomainGetState instead.
func (l *Libvirt) DomainState(dom string) (DomainState, error) {
	d, err := l.lookup(dom)
	if err != nil {
		return DomainNostate, err
	}

	state, _, err := l.DomainGetState(d, 0)
	return DomainState(state), err
}

// SubscribeQEMUEvents streams domain events until the provided context is
// cancelled. If a problem is encountered setting up the event monitor
// connection an error will be returned. Errors encountered during streaming
// will cause the returned event channel to be closed. QEMU domain events.
func (l *Libvirt) SubscribeQEMUEvents(ctx context.Context, dom string) (<-chan DomainEvent, error) {
	d, err := l.lookup(dom)
	if err != nil {
		return nil, err
	}

	callbackID, err := l.QEMUConnectDomainMonitorEventRegister([]Domain{d}, nil, 0)
	if err != nil {
		return nil, err
	}

	stream := event.NewStream(constants.QEMUProgram, callbackID)
	l.addStream(stream)
	ch := make(chan DomainEvent)
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer l.unsubscribeQEMUEvents(stream)
		defer stream.Shutdown()
		defer func() { close(ch) }()

		for {
			select {
			case ev, ok := <-stream.Recv():
				if !ok {
					return
				}
				ch <- *ev.(*DomainEvent)
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// unsubscribeQEMUEvents stops the flow of events from QEMU through libvirt.
func (l *Libvirt) unsubscribeQEMUEvents(stream *event.Stream) error {
	err := l.QEMUConnectDomainMonitorEventDeregister(stream.CallbackID)
	l.removeStream(stream.CallbackID)

	return err
}

// SubscribeEvents allows the caller to subscribe to any of the event types
// supported by libvirt. The events will continue to be streamed until the
// caller cancels the provided context. After canceling the context, callers
// should wait until the channel is closed to be sure they're collected all the
// events.
func (l *Libvirt) SubscribeEvents(ctx context.Context, eventID DomainEventID,
	dom OptDomain) (<-chan interface{}, error) {

	callbackID, err := l.ConnectDomainEventCallbackRegisterAny(int32(eventID), nil)
	if err != nil {
		return nil, err
	}

	stream := event.NewStream(constants.QEMUProgram, callbackID)
	l.addStream(stream)

	ch := make(chan interface{})
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer l.unsubscribeEvents(stream)
		defer stream.Shutdown()
		defer func() { close(ch) }()

		for {
			select {
			case ev, ok := <-stream.Recv():
				if !ok {
					return
				}
				ch <- ev
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// unsubscribeEvents stops the flow of the specified events from libvirt. There
// are two steps to this process: a call to libvirt to deregister our callback,
// and then removing the callback from the list used by the `route` fucntion. If
// the deregister call fails, we'll return the error, but still remove the
// callback from the list. That's ok; if any events arrive after this point, the
// route function will drop them when it finds no registered handler.
func (l *Libvirt) unsubscribeEvents(stream *event.Stream) error {
	err := l.ConnectDomainEventCallbackDeregisterAny(stream.CallbackID)
	l.removeStream(stream.CallbackID)

	return err
}

// LifecycleEvents streams lifecycle events until the provided context is
// cancelled. If a problem is encountered setting up the event monitor
// connection, an error will be returned. Errors encountered during streaming
// will cause the returned event channel to be closed.
func (l *Libvirt) LifecycleEvents(ctx context.Context) (<-chan DomainEventLifecycleMsg, error) {
	callbackID, err := l.ConnectDomainEventCallbackRegisterAny(int32(DomainEventIDLifecycle), nil)
	if err != nil {
		return nil, err
	}

	stream := event.NewStream(constants.Program, callbackID)
	l.addStream(stream)

	ch := make(chan DomainEventLifecycleMsg)

	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer l.unsubscribeEvents(stream)
		defer stream.Shutdown()
		defer func() { close(ch) }()

		for {
			select {
			case ev, ok := <-stream.Recv():
				if !ok {
					return
				}
				ch <- ev.(*DomainEventCallbackLifecycleMsg).Msg
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// Run executes the given QAPI command against a domain's QEMU instance.
// For a list of available QAPI commands, see:
//	http://git.qemu.org/?p=qemu.git;a=blob;f=qapi-schema.json;hb=HEAD
func (l *Libvirt) Run(dom string, cmd []byte) ([]byte, error) {
	d, err := l.lookup(dom)
	if err != nil {
		return nil, err
	}

	payload := struct {
		Domain  Domain
		Command []byte
		Flags   uint32
	}{
		Domain:  d,
		Command: cmd,
		Flags:   0,
	}

	buf, err := encode(&payload)
	if err != nil {
		return nil, err
	}

	res, err := l.request(constants.QEMUProcDomainMonitorCommand, constants.QEMUProgram, buf)
	if err != nil {
		return nil, err
	}

	// check for QEMU process errors
	if err = getQEMUError(res); err != nil {
		return nil, err
	}

	r := bytes.NewReader(res.Payload)
	dec := xdr.NewDecoder(r)
	data, _, err := dec.DecodeFixedOpaque(int32(r.Len()))
	if err != nil {
		return nil, err
	}

	// drop QMP control characters from start of line, and drop
	// any trailing NULL characters from the end
	return bytes.TrimRight(data[4:], "\x00"), nil
}

// Secrets returns all secrets managed by the libvirt daemon.
//
// Deprecated: use ConnectListAllSecrets instead.
func (l *Libvirt) Secrets() ([]Secret, error) {
	secrets, _, err := l.ConnectListAllSecrets(1, 0)
	return secrets, err
}

// StoragePool returns the storage pool associated with the provided name.
// An error is returned if the requested storage pool is not found.
//
// Deprecated: use StoragePoolLookupByName instead.
func (l *Libvirt) StoragePool(name string) (StoragePool, error) {
	return l.StoragePoolLookupByName(name)
}

// StoragePools returns a list of defined storage pools. Pools are filtered by
// the provided flags. See StoragePools*.
//
// Deprecated: use ConnectListAllStoragePools instead.
func (l *Libvirt) StoragePools(flags ConnectListAllStoragePoolsFlags) ([]StoragePool, error) {
	pools, _, err := l.ConnectListAllStoragePools(1, flags)
	return pools, err
}

// Undefine undefines the domain specified by dom, e.g., 'prod-lb-01'.
// The flags argument allows additional options to be specified such as
// cleaning up snapshot metadata. For more information on available
// flags, see DomainUndefine*.
//
// Deprecated: use DomainUndefineFlags instead.
func (l *Libvirt) Undefine(dom string, flags DomainUndefineFlagsValues) error {
	d, err := l.lookup(dom)
	if err != nil {
		return err
	}

	return l.DomainUndefineFlags(d, flags)
}

// Destroy destroys the domain specified by dom, e.g., 'prod-lb-01'.
// The flags argument allows additional options to be specified such as
// allowing a graceful shutdown with SIGTERM than SIGKILL.
// For more information on available flags, see DomainDestroy*.
//
// Deprecated: use DomainDestroyFlags instead.
func (l *Libvirt) Destroy(dom string, flags DomainDestroyFlagsValues) error {
	d, err := l.lookup(dom)
	if err != nil {
		return err
	}

	return l.DomainDestroyFlags(d, flags)
}

// XML returns a domain's raw XML definition, akin to `virsh dumpxml <domain>`.
// See DomainXMLFlag* for optional flags.
//
// Deprecated: use DomainGetXMLDesc instead.
func (l *Libvirt) XML(dom string, flags DomainXMLFlags) ([]byte, error) {
	d, err := l.lookup(dom)
	if err != nil {
		return nil, err
	}

	xml, err := l.DomainGetXMLDesc(d, flags)
	return []byte(xml), err
}

// DefineXML defines a domain, but does not start it.
//
// Deprecated: use DomainDefineXMLFlags instead.
func (l *Libvirt) DefineXML(x []byte, flags DomainDefineFlags) error {
	_, err := l.DomainDefineXMLFlags(string(x), flags)
	return err
}

// Version returns the version of the libvirt daemon.
//
// Deprecated: use ConnectGetLibVersion instead.
func (l *Libvirt) Version() (string, error) {
	ver, err := l.ConnectGetLibVersion()
	if err != nil {
		return "", err
	}

	// The version is provided as an int following this formula:
	// version * 1,000,000 + minor * 1000 + micro
	// See src/libvirt-host.c # virConnectGetLibVersion
	major := ver / 1000000
	ver %= 1000000
	minor := ver / 1000
	ver %= 1000
	micro := ver

	versionString := fmt.Sprintf("%d.%d.%d", major, minor, micro)
	return versionString, nil
}

// Shutdown shuts down a domain. Note that the guest OS may ignore the request.
// If flags is set to 0 then the hypervisor will choose the method of shutdown it considers best.
//
// Deprecated: use DomainShutdownFlags instead.
func (l *Libvirt) Shutdown(dom string, flags DomainShutdownFlagValues) error {
	d, err := l.lookup(dom)
	if err != nil {
		return err
	}

	return l.DomainShutdownFlags(d, flags)
}

// Reboot reboots the domain. Note that the guest OS may ignore the request.
// If flags is set to zero, then the hypervisor will choose the method of shutdown it considers best.
//
// Deprecated: use DomainReboot instead.
func (l *Libvirt) Reboot(dom string, flags DomainRebootFlagValues) error {
	d, err := l.lookup(dom)
	if err != nil {
		return err
	}

	return l.DomainReboot(d, flags)
}

// Reset resets domain immediately without any guest OS shutdown
//
// Deprecated: use DomainReset instead.
func (l *Libvirt) Reset(dom string) error {
	d, err := l.lookup(dom)
	if err != nil {
		return err
	}

	return l.DomainReset(d, 0)
}

// BlockLimit contains a name and value pair for a Get/SetBlockIOTune limit. The
// Name field is the name of the limit (to see a list of the limits that can be
// applied, execute the 'blkdeviotune' command on a VM in virsh). Callers can
// use the QEMUBlockIO... constants below for the Name value. The Value field is
// the limit to apply.
type BlockLimit struct {
	Name  string
	Value uint64
}

// SetBlockIOTune changes the per-device block I/O tunables within a guest.
// Parameters are the name of the VM, the name of the disk device to which the
// limits should be applied, and 1 or more BlockLimit structs containing the
// actual limits.
//
// The limits which can be applied here are enumerated in the QEMUBlockIO...
// constants above, and you can also see the full list by executing the
// 'blkdeviotune' command on a VM in virsh.
//
// Example usage:
//  SetBlockIOTune("vm-name", "vda", BlockLimit{libvirt.QEMUBlockIOWriteBytesSec, 1000000})
//
// Deprecated: use DomainSetBlockIOTune instead.
func (l *Libvirt) SetBlockIOTune(dom string, disk string, limits ...BlockLimit) error {
	d, err := l.lookup(dom)
	if err != nil {
		return err
	}

	params := make([]TypedParam, len(limits))
	for ix, limit := range limits {
		tpval := NewTypedParamValueUllong(limit.Value)
		params[ix] = TypedParam{Field: limit.Name, Value: *tpval}
	}

	return l.DomainSetBlockIOTune(d, disk, params, uint32(DomainAffectLive))
}

// GetBlockIOTune returns a slice containing the current block I/O tunables for
// a disk.
//
// Deprecated: use DomainGetBlockIOTune instead.
func (l *Libvirt) GetBlockIOTune(dom string, disk string) ([]BlockLimit, error) {
	d, err := l.lookup(dom)
	if err != nil {
		return nil, err
	}

	lims, _, err := l.DomainGetBlockIOTune(d, []string{disk}, 32, uint32(TypedParamStringOkay))
	if err != nil {
		return nil, err
	}

	var limits []BlockLimit

	// now decode each of the returned TypedParams. To do this we read the field
	// name and type, then use the type information to decode the value.
	for _, lim := range lims {
		var l BlockLimit
		name := lim.Field
		switch lim.Value.I.(type) {
		case uint64:
			l = BlockLimit{Name: name, Value: lim.Value.I.(uint64)}
		}
		limits = append(limits, l)
	}

	return limits, nil
}

// lookup returns a domain as seen by libvirt.
func (l *Libvirt) lookup(name string) (Domain, error) {
	return l.DomainLookupByName(name)
}

// getQEMUError checks the provided response for QEMU process errors.
// If an error is found, it is extracted an returned, otherwise nil.
func getQEMUError(r response) error {
	pl := bytes.NewReader(r.Payload)
	dec := xdr.NewDecoder(pl)

	s, _, err := dec.DecodeString()
	if err != nil {
		return err
	}

	var e qemuError
	if err = json.Unmarshal([]byte(s), &e); err != nil {
		return err
	}

	if e.Error.Description != "" {
		return errors.New(e.Error.Description)
	}

	return nil
}

// New configures a new Libvirt RPC connection.
func New(conn net.Conn) *Libvirt {
	l := &Libvirt{
		conn:      conn,
		s:         0,
		r:         bufio.NewReader(conn),
		w:         bufio.NewWriter(conn),
		mu:        &sync.Mutex{},
		callbacks: make(map[int32]chan response),
		events:    make(map[int32]*event.Stream),
	}

	go l.listen()

	return l
}
