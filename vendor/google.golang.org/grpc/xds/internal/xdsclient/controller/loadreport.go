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
 */

package controller

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/xds/internal/xdsclient/controller/version"
	"google.golang.org/grpc/xds/internal/xdsclient/load"
)

// ReportLoad starts an load reporting stream to the given server. If the server
// is not an empty string, and is different from the management server, a new
// ClientConn will be created.
//
// The same options used for creating the Client will be used (including
// NodeProto, and dial options if necessary).
//
// It returns a Store for the user to report loads, a function to cancel the
// load reporting stream.
//
// TODO: LRS refactor; maybe a new controller should be created for a separate
// server, so that the same stream can be shared by different reporters to the
// same server, even if they originate from different Controllers.
func (c *Controller) ReportLoad(server string) (*load.Store, func()) {
	c.lrsMu.Lock()
	defer c.lrsMu.Unlock()

	// If there's already a client to this server, use it. Otherwise, create
	// one.
	lrsC, ok := c.lrsClients[server]
	if !ok {
		lrsC = newLRSClient(c, server)
		c.lrsClients[server] = lrsC
	}

	store := lrsC.ref()
	return store, func() {
		// This is a callback, need to hold lrsMu.
		c.lrsMu.Lock()
		defer c.lrsMu.Unlock()
		if lrsC.unRef() {
			// Delete the lrsClient from map if this is the last reference.
			delete(c.lrsClients, server)
		}
	}
}

// lrsClient maps to one lrsServer. It contains:
// - a ClientConn to this server (only if it's different from the management
// server)
// - a load.Store that contains loads only for this server
type lrsClient struct {
	parent *Controller
	server string

	cc           *grpc.ClientConn // nil if the server is same as the management server
	refCount     int
	cancelStream func()
	loadStore    *load.Store
}

// newLRSClient creates a new LRS stream to the server.
func newLRSClient(parent *Controller, server string) *lrsClient {
	return &lrsClient{
		parent:   parent,
		server:   server,
		refCount: 0,
	}
}

// ref increments the refCount. If this is the first ref, it starts the LRS stream.
//
// Not thread-safe, caller needs to synchronize.
func (lrsC *lrsClient) ref() *load.Store {
	lrsC.refCount++
	if lrsC.refCount == 1 {
		lrsC.startStream()
	}
	return lrsC.loadStore
}

// unRef decrements the refCount, and closes the stream if refCount reaches 0
// (and close the cc if cc is not xDS cc). It returns whether refCount reached 0
// after this call.
//
// Not thread-safe, caller needs to synchronize.
func (lrsC *lrsClient) unRef() (closed bool) {
	lrsC.refCount--
	if lrsC.refCount != 0 {
		return false
	}
	lrsC.parent.logger.Infof("Stopping load report to server: %s", lrsC.server)
	lrsC.cancelStream()
	if lrsC.cc != nil {
		lrsC.cc.Close()
	}
	return true
}

// startStream starts the LRS stream to the server. If server is not the same
// management server from the parent, it also creates a ClientConn.
func (lrsC *lrsClient) startStream() {
	var cc *grpc.ClientConn

	lrsC.parent.logger.Infof("Starting load report to server: %s", lrsC.server)
	if lrsC.server == "" || lrsC.server == lrsC.parent.config.ServerURI {
		// Reuse the xDS client if server is the same.
		cc = lrsC.parent.cc
	} else {
		lrsC.parent.logger.Infof("LRS server is different from management server, starting a new ClientConn")
		ccNew, err := grpc.Dial(lrsC.server, lrsC.parent.config.Creds)
		if err != nil {
			// An error from a non-blocking dial indicates something serious.
			lrsC.parent.logger.Infof("xds: failed to dial load report server {%s}: %v", lrsC.server, err)
			return
		}
		cc = ccNew
		lrsC.cc = ccNew
	}

	var ctx context.Context
	ctx, lrsC.cancelStream = context.WithCancel(context.Background())

	// Create the store and stream.
	lrsC.loadStore = load.NewStore()
	go lrsC.parent.reportLoad(ctx, cc, version.LoadReportingOptions{LoadStore: lrsC.loadStore})
}
