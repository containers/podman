/*
 *
 * Copyright 2021 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package server contains internal server-side functionality used by the public
// facing xds package.
package server

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/grpclog"
	internalbackoff "google.golang.org/grpc/internal/backoff"
	"google.golang.org/grpc/internal/envconfig"
	internalgrpclog "google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/grpcsync"
	"google.golang.org/grpc/xds/internal/xdsclient/bootstrap"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

var (
	logger = grpclog.Component("xds")

	// Backoff strategy for temporary errors received from Accept(). If this
	// needs to be configurable, we can inject it through ListenerWrapperParams.
	bs = internalbackoff.Exponential{Config: backoff.Config{
		BaseDelay:  5 * time.Millisecond,
		Multiplier: 2.0,
		MaxDelay:   1 * time.Second,
	}}
	backoffFunc = bs.Backoff
)

// ServingModeCallback is the callback that users can register to get notified
// about the server's serving mode changes. The callback is invoked with the
// address of the listener and its new mode. The err parameter is set to a
// non-nil error if the server has transitioned into not-serving mode.
type ServingModeCallback func(addr net.Addr, mode connectivity.ServingMode, err error)

// DrainCallback is the callback that an xDS-enabled server registers to get
// notified about updates to the Listener configuration. The server is expected
// to gracefully shutdown existing connections, thereby forcing clients to
// reconnect and have the new configuration applied to the newly created
// connections.
type DrainCallback func(addr net.Addr)

func prefixLogger(p *listenerWrapper) *internalgrpclog.PrefixLogger {
	return internalgrpclog.NewPrefixLogger(logger, fmt.Sprintf("[xds-server-listener %p] ", p))
}

// XDSClient wraps the methods on the XDSClient which are required by
// the listenerWrapper.
type XDSClient interface {
	WatchListener(string, func(xdsresource.ListenerUpdate, error)) func()
	WatchRouteConfig(string, func(xdsresource.RouteConfigUpdate, error)) func()
	BootstrapConfig() *bootstrap.Config
}

// ListenerWrapperParams wraps parameters required to create a listenerWrapper.
type ListenerWrapperParams struct {
	// Listener is the net.Listener passed by the user that is to be wrapped.
	Listener net.Listener
	// ListenerResourceName is the xDS Listener resource to request.
	ListenerResourceName string
	// XDSCredsInUse specifies whether or not the user expressed interest to
	// receive security configuration from the control plane.
	XDSCredsInUse bool
	// XDSClient provides the functionality from the XDSClient required here.
	XDSClient XDSClient
	// ModeCallback is the callback to invoke when the serving mode changes.
	ModeCallback ServingModeCallback
	// DrainCallback is the callback to invoke when the Listener gets a LDS
	// update.
	DrainCallback DrainCallback
}

// NewListenerWrapper creates a new listenerWrapper with params. It returns a
// net.Listener and a channel which is written to, indicating that the former is
// ready to be passed to grpc.Serve().
//
// Only TCP listeners are supported.
func NewListenerWrapper(params ListenerWrapperParams) (net.Listener, <-chan struct{}) {
	lw := &listenerWrapper{
		Listener:          params.Listener,
		name:              params.ListenerResourceName,
		xdsCredsInUse:     params.XDSCredsInUse,
		xdsC:              params.XDSClient,
		modeCallback:      params.ModeCallback,
		drainCallback:     params.DrainCallback,
		isUnspecifiedAddr: params.Listener.Addr().(*net.TCPAddr).IP.IsUnspecified(),

		closed:      grpcsync.NewEvent(),
		goodUpdate:  grpcsync.NewEvent(),
		ldsUpdateCh: make(chan ldsUpdateWithError, 1),
		rdsUpdateCh: make(chan rdsHandlerUpdate, 1),
	}
	lw.logger = prefixLogger(lw)

	// Serve() verifies that Addr() returns a valid TCPAddr. So, it is safe to
	// ignore the error from SplitHostPort().
	lisAddr := lw.Listener.Addr().String()
	lw.addr, lw.port, _ = net.SplitHostPort(lisAddr)

	lw.rdsHandler = newRDSHandler(lw.xdsC, lw.rdsUpdateCh)

	cancelWatch := lw.xdsC.WatchListener(lw.name, lw.handleListenerUpdate)
	lw.logger.Infof("Watch started on resource name %v", lw.name)
	lw.cancelWatch = func() {
		cancelWatch()
		lw.logger.Infof("Watch cancelled on resource name %v", lw.name)
	}
	go lw.run()
	return lw, lw.goodUpdate.Done()
}

type ldsUpdateWithError struct {
	update xdsresource.ListenerUpdate
	err    error
}

// listenerWrapper wraps the net.Listener associated with the listening address
// passed to Serve(). It also contains all other state associated with this
// particular invocation of Serve().
type listenerWrapper struct {
	net.Listener
	logger *internalgrpclog.PrefixLogger

	name          string
	xdsCredsInUse bool
	xdsC          XDSClient
	cancelWatch   func()
	modeCallback  ServingModeCallback
	drainCallback DrainCallback

	// Set to true if the listener is bound to the IP_ANY address (which is
	// "0.0.0.0" for IPv4 and "::" for IPv6).
	isUnspecifiedAddr bool
	// Listening address and port. Used to validate the socket address in the
	// Listener resource received from the control plane.
	addr, port string

	// This is used to notify that a good update has been received and that
	// Serve() can be invoked on the underlying gRPC server. Using an event
	// instead of a vanilla channel simplifies the update handler as it need not
	// keep track of whether the received update is the first one or not.
	goodUpdate *grpcsync.Event
	// A small race exists in the XDSClient code between the receipt of an xDS
	// response and the user cancelling the associated watch. In this window,
	// the registered callback may be invoked after the watch is canceled, and
	// the user is expected to work around this. This event signifies that the
	// listener is closed (and hence the watch is cancelled), and we drop any
	// updates received in the callback if this event has fired.
	closed *grpcsync.Event

	// mu guards access to the current serving mode and the filter chains. The
	// reason for using an rw lock here is that these fields are read in
	// Accept() for all incoming connections, but writes happen rarely (when we
	// get a Listener resource update).
	mu sync.RWMutex
	// Current serving mode.
	mode connectivity.ServingMode
	// Filter chains received as part of the last good update.
	filterChains *xdsresource.FilterChainManager

	// rdsHandler is used for any dynamic RDS resources specified in a LDS
	// update.
	rdsHandler *rdsHandler
	// rdsUpdates are the RDS resources received from the management
	// server, keyed on the RouteName of the RDS resource.
	rdsUpdates unsafe.Pointer // map[string]xdsclient.RouteConfigUpdate
	// ldsUpdateCh is a channel for XDSClient LDS updates.
	ldsUpdateCh chan ldsUpdateWithError
	// rdsUpdateCh is a channel for XDSClient RDS updates.
	rdsUpdateCh chan rdsHandlerUpdate
}

// Accept blocks on an Accept() on the underlying listener, and wraps the
// returned net.connWrapper with the configured certificate providers.
func (l *listenerWrapper) Accept() (net.Conn, error) {
	var retries int
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			// Temporary() method is implemented by certain error types returned
			// from the net package, and it is useful for us to not shutdown the
			// server in these conditions. The listen queue being full is one
			// such case.
			if ne, ok := err.(interface{ Temporary() bool }); !ok || !ne.Temporary() {
				return nil, err
			}
			retries++
			timer := time.NewTimer(backoffFunc(retries))
			select {
			case <-timer.C:
			case <-l.closed.Done():
				timer.Stop()
				// Continuing here will cause us to call Accept() again
				// which will return a non-temporary error.
				continue
			}
			continue
		}
		// Reset retries after a successful Accept().
		retries = 0

		// Since the net.Conn represents an incoming connection, the source and
		// destination address can be retrieved from the local address and
		// remote address of the net.Conn respectively.
		destAddr, ok1 := conn.LocalAddr().(*net.TCPAddr)
		srcAddr, ok2 := conn.RemoteAddr().(*net.TCPAddr)
		if !ok1 || !ok2 {
			// If the incoming connection is not a TCP connection, which is
			// really unexpected since we check whether the provided listener is
			// a TCP listener in Serve(), we return an error which would cause
			// us to stop serving.
			return nil, fmt.Errorf("received connection with non-TCP address (local: %T, remote %T)", conn.LocalAddr(), conn.RemoteAddr())
		}

		l.mu.RLock()
		if l.mode == connectivity.ServingModeNotServing {
			// Close connections as soon as we accept them when we are in
			// "not-serving" mode. Since we accept a net.Listener from the user
			// in Serve(), we cannot close the listener when we move to
			// "not-serving". Closing the connection immediately upon accepting
			// is one of the other ways to implement the "not-serving" mode as
			// outlined in gRFC A36.
			l.mu.RUnlock()
			conn.Close()
			continue
		}
		fc, err := l.filterChains.Lookup(xdsresource.FilterChainLookupParams{
			IsUnspecifiedListener: l.isUnspecifiedAddr,
			DestAddr:              destAddr.IP,
			SourceAddr:            srcAddr.IP,
			SourcePort:            srcAddr.Port,
		})
		l.mu.RUnlock()
		if err != nil {
			// When a matching filter chain is not found, we close the
			// connection right away, but do not return an error back to
			// `grpc.Serve()` from where this Accept() was invoked. Returning an
			// error to `grpc.Serve()` causes the server to shutdown. If we want
			// to avoid the server from shutting down, we would need to return
			// an error type which implements the `Temporary() bool` method,
			// which is invoked by `grpc.Serve()` to see if the returned error
			// represents a temporary condition. In the case of a temporary
			// error, `grpc.Serve()` method sleeps for a small duration and
			// therefore ends up blocking all connection attempts during that
			// time frame, which is also not ideal for an error like this.
			l.logger.Warningf("connection from %s to %s failed to find any matching filter chain", conn.RemoteAddr().String(), conn.LocalAddr().String())
			conn.Close()
			continue
		}
		if !envconfig.XDSRBAC {
			return &connWrapper{Conn: conn, filterChain: fc, parent: l}, nil
		}
		var rc xdsresource.RouteConfigUpdate
		if fc.InlineRouteConfig != nil {
			rc = *fc.InlineRouteConfig
		} else {
			rcPtr := atomic.LoadPointer(&l.rdsUpdates)
			rcuPtr := (*map[string]xdsresource.RouteConfigUpdate)(rcPtr)
			// This shouldn't happen, but this error protects against a panic.
			if rcuPtr == nil {
				return nil, errors.New("route configuration pointer is nil")
			}
			rcu := *rcuPtr
			rc = rcu[fc.RouteConfigName]
		}
		// The filter chain will construct a usuable route table on each
		// connection accept. This is done because preinstantiating every route
		// table before it is needed for a connection would potentially lead to
		// a lot of cpu time and memory allocated for route tables that will
		// never be used. There was also a thought to cache this configuration,
		// and reuse it for the next accepted connection. However, this would
		// lead to a lot of code complexity (RDS Updates for a given route name
		// can come it at any time), and connections aren't accepted too often,
		// so this reinstantation of the Route Configuration is an acceptable
		// tradeoff for simplicity.
		vhswi, err := fc.ConstructUsableRouteConfiguration(rc)
		if err != nil {
			l.logger.Warningf("route configuration construction: %v", err)
			conn.Close()
			continue
		}
		return &connWrapper{Conn: conn, filterChain: fc, parent: l, virtualHosts: vhswi}, nil
	}
}

// Close closes the underlying listener. It also cancels the xDS watch
// registered in Serve() and closes any certificate provider instances created
// based on security configuration received in the LDS response.
func (l *listenerWrapper) Close() error {
	l.closed.Fire()
	l.Listener.Close()
	if l.cancelWatch != nil {
		l.cancelWatch()
	}
	l.rdsHandler.close()
	return nil
}

// run is a long running goroutine which handles all xds updates. LDS and RDS
// push updates onto a channel which is read and acted upon from this goroutine.
func (l *listenerWrapper) run() {
	for {
		select {
		case <-l.closed.Done():
			return
		case u := <-l.ldsUpdateCh:
			l.handleLDSUpdate(u)
		case u := <-l.rdsUpdateCh:
			l.handleRDSUpdate(u)
		}
	}
}

// handleLDSUpdate is the callback which handles LDS Updates. It writes the
// received update to the update channel, which is picked up by the run
// goroutine.
func (l *listenerWrapper) handleListenerUpdate(update xdsresource.ListenerUpdate, err error) {
	if l.closed.HasFired() {
		l.logger.Warningf("Resource %q received update: %v with error: %v, after listener was closed", l.name, update, err)
		return
	}
	// Remove any existing entry in ldsUpdateCh and replace with the new one, as the only update
	// listener cares about is most recent update.
	select {
	case <-l.ldsUpdateCh:
	default:
	}
	l.ldsUpdateCh <- ldsUpdateWithError{update: update, err: err}
}

// handleRDSUpdate handles a full rds update from rds handler. On a successful
// update, the server will switch to ServingModeServing as the full
// configuration (both LDS and RDS) has been received.
func (l *listenerWrapper) handleRDSUpdate(update rdsHandlerUpdate) {
	if l.closed.HasFired() {
		l.logger.Warningf("RDS received update: %v with error: %v, after listener was closed", update.updates, update.err)
		return
	}
	if update.err != nil {
		l.logger.Warningf("Received error for rds names specified in resource %q: %+v", l.name, update.err)
		if xdsresource.ErrType(update.err) == xdsresource.ErrorTypeResourceNotFound {
			l.switchMode(nil, connectivity.ServingModeNotServing, update.err)
		}
		// For errors which are anything other than "resource-not-found", we
		// continue to use the old configuration.
		return
	}
	atomic.StorePointer(&l.rdsUpdates, unsafe.Pointer(&update.updates))

	l.switchMode(l.filterChains, connectivity.ServingModeServing, nil)
	l.goodUpdate.Fire()
}

func (l *listenerWrapper) handleLDSUpdate(update ldsUpdateWithError) {
	if update.err != nil {
		l.logger.Warningf("Received error for resource %q: %+v", l.name, update.err)
		if xdsresource.ErrType(update.err) == xdsresource.ErrorTypeResourceNotFound {
			l.switchMode(nil, connectivity.ServingModeNotServing, update.err)
		}
		// For errors which are anything other than "resource-not-found", we
		// continue to use the old configuration.
		return
	}
	l.logger.Infof("Received update for resource %q: %+v", l.name, update.update)

	// Make sure that the socket address on the received Listener resource
	// matches the address of the net.Listener passed to us by the user. This
	// check is done here instead of at the XDSClient layer because of the
	// following couple of reasons:
	// - XDSClient cannot know the listening address of every listener in the
	//   system, and hence cannot perform this check.
	// - this is a very context-dependent check and only the server has the
	//   appropriate context to perform this check.
	//
	// What this means is that the XDSClient has ACKed a resource which can push
	// the server into a "not serving" mode. This is not ideal, but this is
	// what we have decided to do. See gRPC A36 for more details.
	ilc := update.update.InboundListenerCfg
	if ilc.Address != l.addr || ilc.Port != l.port {
		l.switchMode(nil, connectivity.ServingModeNotServing, fmt.Errorf("address (%s:%s) in Listener update does not match listening address: (%s:%s)", ilc.Address, ilc.Port, l.addr, l.port))
		return
	}

	// "Updates to a Listener cause all older connections on that Listener to be
	// gracefully shut down with a grace period of 10 minutes for long-lived
	// RPC's, such that clients will reconnect and have the updated
	// configuration apply." - A36 Note that this is not the same as moving the
	// Server's state to ServingModeNotServing. That prevents new connections
	// from being accepted, whereas here we simply want the clients to reconnect
	// to get the updated configuration.
	if envconfig.XDSRBAC {
		if l.drainCallback != nil {
			l.drainCallback(l.Listener.Addr())
		}
	}
	l.rdsHandler.updateRouteNamesToWatch(ilc.FilterChains.RouteConfigNames)
	// If there are no dynamic RDS Configurations still needed to be received
	// from the management server, this listener has all the configuration
	// needed, and is ready to serve.
	if len(ilc.FilterChains.RouteConfigNames) == 0 {
		l.switchMode(ilc.FilterChains, connectivity.ServingModeServing, nil)
		l.goodUpdate.Fire()
	}
}

func (l *listenerWrapper) switchMode(fcs *xdsresource.FilterChainManager, newMode connectivity.ServingMode, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.filterChains = fcs
	l.mode = newMode
	if l.modeCallback != nil {
		l.modeCallback(l.Listener.Addr(), newMode, err)
	}
	l.logger.Warningf("Listener %q entering mode: %q due to error: %v", l.Addr(), newMode, err)
}
