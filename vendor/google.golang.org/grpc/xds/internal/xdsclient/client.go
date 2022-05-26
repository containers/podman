/*
 *
 * Copyright 2019 gRPC authors.
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

// Package xdsclient implements a full fledged gRPC client for the xDS API used
// by the xds resolver and balancer implementations.
package xdsclient

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/internal/cache"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/grpcsync"
	"google.golang.org/grpc/xds/internal/xdsclient/bootstrap"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

// clientImpl is the real implementation of the xds client. The exported Client
// is a wrapper of this struct with a ref count.
//
// Implements UpdateHandler interface.
// TODO(easwars): Make a wrapper struct which implements this interface in the
// style of ccBalancerWrapper so that the Client type does not implement these
// exported methods.
type clientImpl struct {
	done   *grpcsync.Event
	config *bootstrap.Config

	// authorityMu protects the authority fields. It's necessary because an
	// authority is created when it's used.
	authorityMu sync.Mutex
	// authorities is a map from ServerConfig to authority. So that
	// different authorities sharing the same ServerConfig can share the
	// authority.
	//
	// The key is **ServerConfig.String()**, not the authority name.
	//
	// An authority is either in authorities, or idleAuthorities,
	// never both.
	authorities map[string]*authority
	// idleAuthorities keeps the authorities that are not used (the last
	// watch on it was canceled). They are kept in the cache and will be deleted
	// after a timeout. The key is ServerConfig.String().
	//
	// An authority is either in authorities, or idleAuthorities,
	// never both.
	idleAuthorities *cache.TimeoutCache

	logger             *grpclog.PrefixLogger
	watchExpiryTimeout time.Duration
}

// newWithConfig returns a new xdsClient with the given config.
func newWithConfig(config *bootstrap.Config, watchExpiryTimeout time.Duration, idleAuthorityDeleteTimeout time.Duration) (_ *clientImpl, retErr error) {
	c := &clientImpl{
		done:               grpcsync.NewEvent(),
		config:             config,
		watchExpiryTimeout: watchExpiryTimeout,

		authorities:     make(map[string]*authority),
		idleAuthorities: cache.NewTimeoutCache(idleAuthorityDeleteTimeout),
	}

	defer func() {
		if retErr != nil {
			c.Close()
		}
	}()

	c.logger = prefixLogger(c)
	c.logger.Infof("Created ClientConn to xDS management server: %s", config.XDSServer)

	c.logger.Infof("Created")
	return c, nil
}

// BootstrapConfig returns the configuration read from the bootstrap file.
// Callers must treat the return value as read-only.
func (c *clientRefCounted) BootstrapConfig() *bootstrap.Config {
	return c.config
}

// Close closes the gRPC connection to the management server.
func (c *clientImpl) Close() {
	if c.done.HasFired() {
		return
	}
	c.done.Fire()
	// TODO: Should we invoke the registered callbacks here with an error that
	// the client is closed?

	// Note that Close needs to check for nils even if some of them are always
	// set in the constructor. This is because the constructor defers Close() in
	// error cases, and the fields might not be set when the error happens.

	c.authorityMu.Lock()
	for _, a := range c.authorities {
		a.close()
	}
	c.idleAuthorities.Clear(true)
	c.authorityMu.Unlock()

	c.logger.Infof("Shutdown")
}

func (c *clientImpl) filterChainUpdateValidator(fc *xdsresource.FilterChain) error {
	if fc == nil {
		return nil
	}
	return c.securityConfigUpdateValidator(fc.SecurityCfg)
}

func (c *clientImpl) securityConfigUpdateValidator(sc *xdsresource.SecurityConfig) error {
	if sc == nil {
		return nil
	}
	if sc.IdentityInstanceName != "" {
		if _, ok := c.config.CertProviderConfigs[sc.IdentityInstanceName]; !ok {
			return fmt.Errorf("identitiy certificate provider instance name %q missing in bootstrap configuration", sc.IdentityInstanceName)
		}
	}
	if sc.RootInstanceName != "" {
		if _, ok := c.config.CertProviderConfigs[sc.RootInstanceName]; !ok {
			return fmt.Errorf("root certificate provider instance name %q missing in bootstrap configuration", sc.RootInstanceName)
		}
	}
	return nil
}

func (c *clientImpl) updateValidator(u interface{}) error {
	switch update := u.(type) {
	case xdsresource.ListenerUpdate:
		if update.InboundListenerCfg == nil || update.InboundListenerCfg.FilterChains == nil {
			return nil
		}
		return update.InboundListenerCfg.FilterChains.Validate(c.filterChainUpdateValidator)
	case xdsresource.ClusterUpdate:
		return c.securityConfigUpdateValidator(update.SecurityCfg)
	default:
		// We currently invoke this update validation function only for LDS and
		// CDS updates. In the future, if we wish to invoke it for other xDS
		// updates, corresponding plumbing needs to be added to those unmarshal
		// functions.
	}
	return nil
}
