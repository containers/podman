/*
 *
 * Copyright 2020 gRPC authors.
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

package xdsclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/xds/internal/xdsclient/bootstrap"
)

const (
	defaultWatchExpiryTimeout         = 15 * time.Second
	defaultIdleAuthorityDeleteTimeout = 5 * time.Minute
)

// This is the Client returned by New(). It contains one client implementation,
// and maintains the refcount.
var singletonClient = &clientRefCounted{}

// To override in tests.
var bootstrapNewConfig = bootstrap.NewConfig

// clientRefCounted is ref-counted, and to be shared by the xds resolver and
// balancer implementations, across multiple ClientConns and Servers.
type clientRefCounted struct {
	*clientImpl

	// This mu protects all the fields, including the embedded clientImpl above.
	mu       sync.Mutex
	refCount int
}

// New returns a new xdsClient configured by the bootstrap file specified in env
// variable GRPC_XDS_BOOTSTRAP or GRPC_XDS_BOOTSTRAP_CONFIG.
//
// The returned xdsClient is a singleton. This function creates the xds client
// if it doesn't already exist.
//
// Note that the first invocation of New() or NewWithConfig() sets the client
// singleton. The following calls will return the singleton xds client without
// checking or using the config.
func New() (XDSClient, error) {
	// This cannot just return newRefCounted(), because in error cases, the
	// returned nil is a typed nil (*clientRefCounted), which may cause nil
	// checks fail.
	c, err := newRefCounted()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func newRefCounted() (*clientRefCounted, error) {
	singletonClient.mu.Lock()
	defer singletonClient.mu.Unlock()
	// If the client implementation was created, increment ref count and return
	// the client.
	if singletonClient.clientImpl != nil {
		singletonClient.refCount++
		return singletonClient, nil
	}

	// Create the new client implementation.
	config, err := bootstrapNewConfig()
	if err != nil {
		return nil, fmt.Errorf("xds: failed to read bootstrap file: %v", err)
	}
	c, err := newWithConfig(config, defaultWatchExpiryTimeout, defaultIdleAuthorityDeleteTimeout)
	if err != nil {
		return nil, err
	}

	singletonClient.clientImpl = c
	singletonClient.refCount++
	return singletonClient, nil
}

// NewWithConfig returns a new xdsClient configured by the given config.
//
// The returned xdsClient is a singleton. This function creates the xds client
// if it doesn't already exist.
//
// Note that the first invocation of New() or NewWithConfig() sets the client
// singleton. The following calls will return the singleton xds client without
// checking or using the config.
//
// This function is internal only, for c2p resolver and testing to use. DO NOT
// use this elsewhere. Use New() instead.
func NewWithConfig(config *bootstrap.Config) (XDSClient, error) {
	singletonClient.mu.Lock()
	defer singletonClient.mu.Unlock()
	// If the client implementation was created, increment ref count and return
	// the client.
	if singletonClient.clientImpl != nil {
		singletonClient.refCount++
		return singletonClient, nil
	}

	// Create the new client implementation.
	c, err := newWithConfig(config, defaultWatchExpiryTimeout, defaultIdleAuthorityDeleteTimeout)
	if err != nil {
		return nil, err
	}

	singletonClient.clientImpl = c
	singletonClient.refCount++
	return singletonClient, nil
}

// Close closes the client. It does ref count of the xds client implementation,
// and closes the gRPC connection to the management server when ref count
// reaches 0.
func (c *clientRefCounted) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refCount--
	if c.refCount == 0 {
		c.clientImpl.Close()
		// Set clientImpl back to nil. So if New() is called after this, a new
		// implementation will be created.
		c.clientImpl = nil
	}
}

// NewWithConfigForTesting is exported for testing only.
//
// Note that this function doesn't set the singleton, so that the testing states
// don't leak.
func NewWithConfigForTesting(config *bootstrap.Config, watchExpiryTimeout time.Duration) (XDSClient, error) {
	cl, err := newWithConfig(config, watchExpiryTimeout, defaultIdleAuthorityDeleteTimeout)
	if err != nil {
		return nil, err
	}
	return &clientRefCounted{clientImpl: cl, refCount: 1}, nil
}

// NewClientWithBootstrapContents returns an xds client for this config,
// separate from the global singleton.  This should be used for testing
// purposes only.
func NewClientWithBootstrapContents(contents []byte) (XDSClient, error) {
	// Normalize the contents
	buf := bytes.Buffer{}
	err := json.Indent(&buf, contents, "", "")
	if err != nil {
		return nil, fmt.Errorf("xds: error normalizing JSON: %v", err)
	}
	contents = bytes.TrimSpace(buf.Bytes())

	clientsMu.Lock()
	defer clientsMu.Unlock()
	if c := clients[string(contents)]; c != nil {
		c.mu.Lock()
		// Since we don't remove the *Client from the map when it is closed, we
		// need to recreate the impl if the ref count dropped to zero.
		if c.refCount > 0 {
			c.refCount++
			c.mu.Unlock()
			return c, nil
		}
		c.mu.Unlock()
	}

	bcfg, err := bootstrap.NewConfigFromContents(contents)
	if err != nil {
		return nil, fmt.Errorf("xds: error with bootstrap config: %v", err)
	}

	cImpl, err := newWithConfig(bcfg, defaultWatchExpiryTimeout, defaultIdleAuthorityDeleteTimeout)
	if err != nil {
		return nil, err
	}

	c := &clientRefCounted{clientImpl: cImpl, refCount: 1}
	clients[string(contents)] = c
	return c, nil
}

var (
	clients   = map[string]*clientRefCounted{}
	clientsMu sync.Mutex
)
