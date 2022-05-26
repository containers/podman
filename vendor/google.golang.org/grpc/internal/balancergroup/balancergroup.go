/*
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
 */

// Package balancergroup implements a utility struct to bind multiple balancers
// into one balancer.
package balancergroup

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/internal/cache"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/resolver"
)

// subBalancerWrapper is used to keep the configurations that will be used to start
// the underlying balancer. It can be called to start/stop the underlying
// balancer.
//
// When the config changes, it will pass the update to the underlying balancer
// if it exists.
//
// TODO: move to a separate file?
type subBalancerWrapper struct {
	// subBalancerWrapper is passed to the sub-balancer as a ClientConn
	// wrapper, only to keep the state and picker.  When sub-balancer is
	// restarted while in cache, the picker needs to be resent.
	//
	// It also contains the sub-balancer ID, so the parent balancer group can
	// keep track of SubConn/pickers and the sub-balancers they belong to. Some
	// of the actions are forwarded to the parent ClientConn with no change.
	// Some are forward to balancer group with the sub-balancer ID.
	balancer.ClientConn
	id    string
	group *BalancerGroup

	mu    sync.Mutex
	state balancer.State

	// The static part of sub-balancer. Keeps balancerBuilders and addresses.
	// To be used when restarting sub-balancer.
	builder balancer.Builder
	// Options to be passed to sub-balancer at the time of creation.
	buildOpts balancer.BuildOptions
	// ccState is a cache of the addresses/balancer config, so when the balancer
	// is restarted after close, it will get the previous update. It's a pointer
	// and is set to nil at init, so when the balancer is built for the first
	// time (not a restart), it won't receive an empty update. Note that this
	// isn't reset to nil when the underlying balancer is closed.
	ccState *balancer.ClientConnState
	// The dynamic part of sub-balancer. Only used when balancer group is
	// started. Gets cleared when sub-balancer is closed.
	balancer balancer.Balancer
}

// UpdateState overrides balancer.ClientConn, to keep state and picker.
func (sbc *subBalancerWrapper) UpdateState(state balancer.State) {
	sbc.mu.Lock()
	sbc.state = state
	sbc.group.updateBalancerState(sbc.id, state)
	sbc.mu.Unlock()
}

// NewSubConn overrides balancer.ClientConn, so balancer group can keep track of
// the relation between subconns and sub-balancers.
func (sbc *subBalancerWrapper) NewSubConn(addrs []resolver.Address, opts balancer.NewSubConnOptions) (balancer.SubConn, error) {
	return sbc.group.newSubConn(sbc, addrs, opts)
}

func (sbc *subBalancerWrapper) updateBalancerStateWithCachedPicker() {
	sbc.mu.Lock()
	if sbc.state.Picker != nil {
		sbc.group.updateBalancerState(sbc.id, sbc.state)
	}
	sbc.mu.Unlock()
}

func (sbc *subBalancerWrapper) startBalancer() {
	b := sbc.builder.Build(sbc, sbc.buildOpts)
	sbc.group.logger.Infof("Created child policy %p of type %v", b, sbc.builder.Name())
	sbc.balancer = b
	if sbc.ccState != nil {
		b.UpdateClientConnState(*sbc.ccState)
	}
}

// exitIdle invokes the sub-balancer's ExitIdle method. Returns a boolean
// indicating whether or not the operation was completed.
func (sbc *subBalancerWrapper) exitIdle() (complete bool) {
	b := sbc.balancer
	if b == nil {
		return true
	}
	if ei, ok := b.(balancer.ExitIdler); ok {
		ei.ExitIdle()
		return true
	}
	return false
}

func (sbc *subBalancerWrapper) updateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	b := sbc.balancer
	if b == nil {
		// This sub-balancer was closed. This can happen when EDS removes a
		// locality. The balancer for this locality was already closed, and the
		// SubConns are being deleted. But SubConn state change can still
		// happen.
		return
	}
	b.UpdateSubConnState(sc, state)
}

func (sbc *subBalancerWrapper) updateClientConnState(s balancer.ClientConnState) error {
	sbc.ccState = &s
	b := sbc.balancer
	if b == nil {
		// This sub-balancer was closed. This should never happen because
		// sub-balancers are closed when the locality is removed from EDS, or
		// the balancer group is closed. There should be no further address
		// updates when either of this happened.
		//
		// This will be a common case with priority support, because a
		// sub-balancer (and the whole balancer group) could be closed because
		// it's the lower priority, but it can still get address updates.
		return nil
	}
	return b.UpdateClientConnState(s)
}

func (sbc *subBalancerWrapper) resolverError(err error) {
	b := sbc.balancer
	if b == nil {
		// This sub-balancer was closed. This should never happen because
		// sub-balancers are closed when the locality is removed from EDS, or
		// the balancer group is closed. There should be no further address
		// updates when either of this happened.
		//
		// This will be a common case with priority support, because a
		// sub-balancer (and the whole balancer group) could be closed because
		// it's the lower priority, but it can still get address updates.
		return
	}
	b.ResolverError(err)
}

func (sbc *subBalancerWrapper) stopBalancer() {
	sbc.balancer.Close()
	sbc.balancer = nil
}

// BalancerGroup takes a list of balancers, and make them into one balancer.
//
// Note that this struct doesn't implement balancer.Balancer, because it's not
// intended to be used directly as a balancer. It's expected to be used as a
// sub-balancer manager by a high level balancer.
//
// Updates from ClientConn are forwarded to sub-balancers
//  - service config update
//  - address update
//  - subConn state change
//     - find the corresponding balancer and forward
//
// Actions from sub-balances are forwarded to parent ClientConn
//  - new/remove SubConn
//  - picker update and health states change
//     - sub-pickers are sent to an aggregator provided by the parent, which
//     will group them into a group-picker. The aggregated connectivity state is
//     also handled by the aggregator.
//  - resolveNow
//
// Sub-balancers are only built when the balancer group is started. If the
// balancer group is closed, the sub-balancers are also closed. And it's
// guaranteed that no updates will be sent to parent ClientConn from a closed
// balancer group.
type BalancerGroup struct {
	cc        balancer.ClientConn
	buildOpts balancer.BuildOptions
	logger    *grpclog.PrefixLogger

	// stateAggregator is where the state/picker updates will be sent to. It's
	// provided by the parent balancer, to build a picker with all the
	// sub-pickers.
	stateAggregator BalancerStateAggregator

	// outgoingMu guards all operations in the direction:
	// ClientConn-->Sub-balancer. Including start, stop, resolver updates and
	// SubConn state changes.
	//
	// The corresponding boolean outgoingStarted is used to stop further updates
	// to sub-balancers after they are closed.
	outgoingMu         sync.Mutex
	outgoingStarted    bool
	idToBalancerConfig map[string]*subBalancerWrapper
	// Cache for sub-balancers when they are removed.
	balancerCache *cache.TimeoutCache

	// incomingMu is to make sure this balancer group doesn't send updates to cc
	// after it's closed.
	//
	// We don't share the mutex to avoid deadlocks (e.g. a call to sub-balancer
	// may call back to balancer group inline. It causes deaclock if they
	// require the same mutex).
	//
	// We should never need to hold multiple locks at the same time in this
	// struct. The case where two locks are held can only happen when the
	// underlying balancer calls back into balancer group inline. So there's an
	// implicit lock acquisition order that outgoingMu is locked before
	// incomingMu.

	// incomingMu guards all operations in the direction:
	// Sub-balancer-->ClientConn. Including NewSubConn, RemoveSubConn. It also
	// guards the map from SubConn to balancer ID, so updateSubConnState needs
	// to hold it shortly to find the sub-balancer to forward the update.
	//
	// UpdateState is called by the balancer state aggretator, and it will
	// decide when and whether to call.
	//
	// The corresponding boolean incomingStarted is used to stop further updates
	// from sub-balancers after they are closed.
	incomingMu      sync.Mutex
	incomingStarted bool // This boolean only guards calls back to ClientConn.
	scToSubBalancer map[balancer.SubConn]*subBalancerWrapper
}

// DefaultSubBalancerCloseTimeout is defined as a variable instead of const for
// testing.
//
// TODO: make it a parameter for New().
var DefaultSubBalancerCloseTimeout = 15 * time.Minute

// New creates a new BalancerGroup. Note that the BalancerGroup
// needs to be started to work.
func New(cc balancer.ClientConn, bOpts balancer.BuildOptions, stateAggregator BalancerStateAggregator, logger *grpclog.PrefixLogger) *BalancerGroup {
	return &BalancerGroup{
		cc:              cc,
		buildOpts:       bOpts,
		logger:          logger,
		stateAggregator: stateAggregator,

		idToBalancerConfig: make(map[string]*subBalancerWrapper),
		balancerCache:      cache.NewTimeoutCache(DefaultSubBalancerCloseTimeout),
		scToSubBalancer:    make(map[balancer.SubConn]*subBalancerWrapper),
	}
}

// Start starts the balancer group, including building all the sub-balancers,
// and send the existing addresses to them.
//
// A BalancerGroup can be closed and started later. When a BalancerGroup is
// closed, it can still receive address updates, which will be applied when
// restarted.
func (bg *BalancerGroup) Start() {
	bg.incomingMu.Lock()
	bg.incomingStarted = true
	bg.incomingMu.Unlock()

	bg.outgoingMu.Lock()
	if bg.outgoingStarted {
		bg.outgoingMu.Unlock()
		return
	}

	for _, config := range bg.idToBalancerConfig {
		config.startBalancer()
	}
	bg.outgoingStarted = true
	bg.outgoingMu.Unlock()
}

// Add adds a balancer built by builder to the group, with given id.
func (bg *BalancerGroup) Add(id string, builder balancer.Builder) {
	// Store data in static map, and then check to see if bg is started.
	bg.outgoingMu.Lock()
	var sbc *subBalancerWrapper
	// If outgoingStarted is true, search in the cache. Otherwise, cache is
	// guaranteed to be empty, searching is unnecessary.
	if bg.outgoingStarted {
		if old, ok := bg.balancerCache.Remove(id); ok {
			sbc, _ = old.(*subBalancerWrapper)
			if sbc != nil && sbc.builder != builder {
				// If the sub-balancer in cache was built with a different
				// balancer builder, don't use it, cleanup this old-balancer,
				// and behave as sub-balancer is not found in cache.
				//
				// NOTE that this will also drop the cached addresses for this
				// sub-balancer, which seems to be reasonable.
				sbc.stopBalancer()
				// cleanupSubConns must be done before the new balancer starts,
				// otherwise new SubConns created by the new balancer might be
				// removed by mistake.
				bg.cleanupSubConns(sbc)
				sbc = nil
			}
		}
	}
	if sbc == nil {
		sbc = &subBalancerWrapper{
			ClientConn: bg.cc,
			id:         id,
			group:      bg,
			builder:    builder,
			buildOpts:  bg.buildOpts,
		}
		if bg.outgoingStarted {
			// Only start the balancer if bg is started. Otherwise, we only keep the
			// static data.
			sbc.startBalancer()
		}
	} else {
		// When brining back a sub-balancer from cache, re-send the cached
		// picker and state.
		sbc.updateBalancerStateWithCachedPicker()
	}
	bg.idToBalancerConfig[id] = sbc
	bg.outgoingMu.Unlock()
}

// Remove removes the balancer with id from the group.
//
// But doesn't close the balancer. The balancer is kept in a cache, and will be
// closed after timeout. Cleanup work (closing sub-balancer and removing
// subconns) will be done after timeout.
func (bg *BalancerGroup) Remove(id string) {
	bg.outgoingMu.Lock()
	if sbToRemove, ok := bg.idToBalancerConfig[id]; ok {
		if bg.outgoingStarted {
			bg.balancerCache.Add(id, sbToRemove, func() {
				// After timeout, when sub-balancer is removed from cache, need
				// to close the underlying sub-balancer, and remove all its
				// subconns.
				bg.outgoingMu.Lock()
				if bg.outgoingStarted {
					sbToRemove.stopBalancer()
				}
				bg.outgoingMu.Unlock()
				bg.cleanupSubConns(sbToRemove)
			})
		}
		delete(bg.idToBalancerConfig, id)
	} else {
		bg.logger.Infof("balancer group: trying to remove a non-existing locality from balancer group: %v", id)
	}
	bg.outgoingMu.Unlock()
}

// bg.remove(id) doesn't do cleanup for the sub-balancer. This function does
// cleanup after the timeout.
func (bg *BalancerGroup) cleanupSubConns(config *subBalancerWrapper) {
	bg.incomingMu.Lock()
	// Remove SubConns. This is only done after the balancer is
	// actually closed.
	//
	// NOTE: if NewSubConn is called by this (closed) balancer later, the
	// SubConn will be leaked. This shouldn't happen if the balancer
	// implementation is correct. To make sure this never happens, we need to
	// add another layer (balancer manager) between balancer group and the
	// sub-balancers.
	for sc, b := range bg.scToSubBalancer {
		if b == config {
			bg.cc.RemoveSubConn(sc)
			delete(bg.scToSubBalancer, sc)
		}
	}
	bg.incomingMu.Unlock()
}

// connect attempts to connect to all subConns belonging to sb.
func (bg *BalancerGroup) connect(sb *subBalancerWrapper) {
	bg.incomingMu.Lock()
	for sc, b := range bg.scToSubBalancer {
		if b == sb {
			sc.Connect()
		}
	}
	bg.incomingMu.Unlock()
}

// Following are actions from the parent grpc.ClientConn, forward to sub-balancers.

// UpdateSubConnState handles the state for the subconn. It finds the
// corresponding balancer and forwards the update.
func (bg *BalancerGroup) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	bg.incomingMu.Lock()
	config, ok := bg.scToSubBalancer[sc]
	if !ok {
		bg.incomingMu.Unlock()
		return
	}
	if state.ConnectivityState == connectivity.Shutdown {
		// Only delete sc from the map when state changed to Shutdown.
		delete(bg.scToSubBalancer, sc)
	}
	bg.incomingMu.Unlock()

	bg.outgoingMu.Lock()
	config.updateSubConnState(sc, state)
	bg.outgoingMu.Unlock()
}

// UpdateClientConnState handles ClientState (including balancer config and
// addresses) from resolver. It finds the balancer and forwards the update.
func (bg *BalancerGroup) UpdateClientConnState(id string, s balancer.ClientConnState) error {
	bg.outgoingMu.Lock()
	defer bg.outgoingMu.Unlock()
	if config, ok := bg.idToBalancerConfig[id]; ok {
		return config.updateClientConnState(s)
	}
	return nil
}

// ResolverError forwards resolver errors to all sub-balancers.
func (bg *BalancerGroup) ResolverError(err error) {
	bg.outgoingMu.Lock()
	for _, config := range bg.idToBalancerConfig {
		config.resolverError(err)
	}
	bg.outgoingMu.Unlock()
}

// Following are actions from sub-balancers, forward to ClientConn.

// newSubConn: forward to ClientConn, and also create a map from sc to balancer,
// so state update will find the right balancer.
//
// One note about removing SubConn: only forward to ClientConn, but not delete
// from map. Delete sc from the map only when state changes to Shutdown. Since
// it's just forwarding the action, there's no need for a removeSubConn()
// wrapper function.
func (bg *BalancerGroup) newSubConn(config *subBalancerWrapper, addrs []resolver.Address, opts balancer.NewSubConnOptions) (balancer.SubConn, error) {
	// NOTE: if balancer with id was already removed, this should also return
	// error. But since we call balancer.stopBalancer when removing the balancer, this
	// shouldn't happen.
	bg.incomingMu.Lock()
	if !bg.incomingStarted {
		bg.incomingMu.Unlock()
		return nil, fmt.Errorf("NewSubConn is called after balancer group is closed")
	}
	sc, err := bg.cc.NewSubConn(addrs, opts)
	if err != nil {
		bg.incomingMu.Unlock()
		return nil, err
	}
	bg.scToSubBalancer[sc] = config
	bg.incomingMu.Unlock()
	return sc, nil
}

// updateBalancerState: forward the new state to balancer state aggregator. The
// aggregator will create an aggregated picker and an aggregated connectivity
// state, then forward to ClientConn.
func (bg *BalancerGroup) updateBalancerState(id string, state balancer.State) {
	bg.logger.Infof("Balancer state update from locality %v, new state: %+v", id, state)

	// Send new state to the aggregator, without holding the incomingMu.
	// incomingMu is to protect all calls to the parent ClientConn, this update
	// doesn't necessary trigger a call to ClientConn, and should already be
	// protected by aggregator's mutex if necessary.
	if bg.stateAggregator != nil {
		bg.stateAggregator.UpdateState(id, state)
	}
}

// Close closes the balancer. It stops sub-balancers, and removes the subconns.
// The BalancerGroup can be restarted later.
func (bg *BalancerGroup) Close() {
	bg.incomingMu.Lock()
	if bg.incomingStarted {
		bg.incomingStarted = false
		// Also remove all SubConns.
		for sc := range bg.scToSubBalancer {
			bg.cc.RemoveSubConn(sc)
			delete(bg.scToSubBalancer, sc)
		}
	}
	bg.incomingMu.Unlock()

	// Clear(true) runs clear function to close sub-balancers in cache. It
	// must be called out of outgoing mutex.
	bg.balancerCache.Clear(true)

	bg.outgoingMu.Lock()
	if bg.outgoingStarted {
		bg.outgoingStarted = false
		for _, config := range bg.idToBalancerConfig {
			config.stopBalancer()
		}
	}
	bg.outgoingMu.Unlock()
}

// ExitIdle should be invoked when the parent LB policy's ExitIdle is invoked.
// It will trigger this on all sub-balancers, or reconnect their subconns if
// not supported.
func (bg *BalancerGroup) ExitIdle() {
	bg.outgoingMu.Lock()
	for _, config := range bg.idToBalancerConfig {
		if !config.exitIdle() {
			bg.connect(config)
		}
	}
	bg.outgoingMu.Unlock()
}

// ExitIdleOne instructs the sub-balancer `id` to exit IDLE state, if
// appropriate and possible.
func (bg *BalancerGroup) ExitIdleOne(id string) {
	bg.outgoingMu.Lock()
	if config := bg.idToBalancerConfig[id]; config != nil {
		if !config.exitIdle() {
			bg.connect(config)
		}
	}
	bg.outgoingMu.Unlock()
}
