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

// Package ringhash implements the ringhash balancer.
package ringhash

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/balancer/weightedroundrobin"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

// Name is the name of the ring_hash balancer.
const Name = "ring_hash_experimental"

func init() {
	balancer.Register(bb{})
}

type bb struct{}

func (bb) Build(cc balancer.ClientConn, bOpts balancer.BuildOptions) balancer.Balancer {
	b := &ringhashBalancer{
		cc:       cc,
		subConns: make(map[resolver.Address]*subConn),
		scStates: make(map[balancer.SubConn]*subConn),
		csEvltr:  &connectivityStateEvaluator{},
	}
	b.logger = prefixLogger(b)
	b.logger.Infof("Created")
	return b
}

func (bb) Name() string {
	return Name
}

func (bb) ParseConfig(c json.RawMessage) (serviceconfig.LoadBalancingConfig, error) {
	return parseConfig(c)
}

type subConn struct {
	addr string
	sc   balancer.SubConn

	mu sync.RWMutex
	// This is the actual state of this SubConn (as updated by the ClientConn).
	// The effective state can be different, see comment of attemptedToConnect.
	state connectivity.State
	// failing is whether this SubConn is in a failing state. A subConn is
	// considered to be in a failing state if it was previously in
	// TransientFailure.
	//
	// This affects the effective connectivity state of this SubConn, e.g.
	// - if the actual state is Idle or Connecting, but this SubConn is failing,
	// the effective state is TransientFailure.
	//
	// This is used in pick(). E.g. if a subConn is Idle, but has failing as
	// true, pick() will
	// - consider this SubConn as TransientFailure, and check the state of the
	// next SubConn.
	// - trigger Connect() (note that normally a SubConn in real
	// TransientFailure cannot Connect())
	//
	// A subConn starts in non-failing (failing is false). A transition to
	// TransientFailure sets failing to true (and it stays true). A transition
	// to Ready sets failing to false.
	failing bool
	// connectQueued is true if a Connect() was queued for this SubConn while
	// it's not in Idle (most likely was in TransientFailure). A Connect() will
	// be triggered on this SubConn when it turns Idle.
	//
	// When connectivity state is updated to Idle for this SubConn, if
	// connectQueued is true, Connect() will be called on the SubConn.
	connectQueued bool
}

// setState updates the state of this SubConn.
//
// It also handles the queued Connect(). If the new state is Idle, and a
// Connect() was queued, this SubConn will be triggered to Connect().
func (sc *subConn) setState(s connectivity.State) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	switch s {
	case connectivity.Idle:
		// Trigger Connect() if new state is Idle, and there is a queued connect.
		if sc.connectQueued {
			sc.connectQueued = false
			sc.sc.Connect()
		}
	case connectivity.Connecting:
		// Clear connectQueued if the SubConn isn't failing. This state
		// transition is unlikely to happen, but handle this just in case.
		sc.connectQueued = false
	case connectivity.Ready:
		// Clear connectQueued if the SubConn isn't failing. This state
		// transition is unlikely to happen, but handle this just in case.
		sc.connectQueued = false
		// Set to a non-failing state.
		sc.failing = false
	case connectivity.TransientFailure:
		// Set to a failing state.
		sc.failing = true
	}
	sc.state = s
}

// effectiveState returns the effective state of this SubConn. It can be
// different from the actual state, e.g. Idle while the subConn is failing is
// considered TransientFailure. Read comment of field failing for other cases.
func (sc *subConn) effectiveState() connectivity.State {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if sc.failing && (sc.state == connectivity.Idle || sc.state == connectivity.Connecting) {
		return connectivity.TransientFailure
	}
	return sc.state
}

// queueConnect sets a boolean so that when the SubConn state changes to Idle,
// it's Connect() will be triggered. If the SubConn state is already Idle, it
// will just call Connect().
func (sc *subConn) queueConnect() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.state == connectivity.Idle {
		sc.sc.Connect()
		return
	}
	// Queue this connect, and when this SubConn switches back to Idle (happens
	// after backoff in TransientFailure), it will Connect().
	sc.connectQueued = true
}

type ringhashBalancer struct {
	cc     balancer.ClientConn
	logger *grpclog.PrefixLogger

	config *LBConfig

	subConns map[resolver.Address]*subConn // `attributes` is stripped from the keys of this map (the addresses)
	scStates map[balancer.SubConn]*subConn

	// ring is always in sync with subConns. When subConns change, a new ring is
	// generated. Note that address weights updates (they are keys in the
	// subConns map) also regenerates the ring.
	ring    *ring
	picker  balancer.Picker
	csEvltr *connectivityStateEvaluator
	state   connectivity.State

	resolverErr error // the last error reported by the resolver; cleared on successful resolution
	connErr     error // the last connection error; cleared upon leaving TransientFailure
}

// updateAddresses creates new SubConns and removes SubConns, based on the
// address update.
//
// The return value is whether the new address list is different from the
// previous. True if
// - an address was added
// - an address was removed
// - an address's weight was updated
//
// Note that this function doesn't trigger SubConn connecting, so all the new
// SubConn states are Idle.
func (b *ringhashBalancer) updateAddresses(addrs []resolver.Address) bool {
	var addrsUpdated bool
	// addrsSet is the set converted from addrs, it's used for quick lookup of
	// an address.
	//
	// Addresses in this map all have attributes stripped, but metadata set to
	// the weight. So that weight change can be detected.
	//
	// TODO: this won't be necessary if there are ways to compare address
	// attributes.
	addrsSet := make(map[resolver.Address]struct{})
	for _, a := range addrs {
		aNoAttrs := a
		// Strip attributes but set Metadata to the weight.
		aNoAttrs.Attributes = nil
		w := weightedroundrobin.GetAddrInfo(a).Weight
		if w == 0 {
			// If weight is not set, use 1.
			w = 1
		}
		aNoAttrs.Metadata = w
		addrsSet[aNoAttrs] = struct{}{}
		if scInfo, ok := b.subConns[aNoAttrs]; !ok {
			// When creating SubConn, the original address with attributes is
			// passed through. So that connection configurations in attributes
			// (like creds) will be used.
			sc, err := b.cc.NewSubConn([]resolver.Address{a}, balancer.NewSubConnOptions{HealthCheckEnabled: true})
			if err != nil {
				logger.Warningf("base.baseBalancer: failed to create new SubConn: %v", err)
				continue
			}
			scs := &subConn{addr: a.Addr, sc: sc}
			scs.setState(connectivity.Idle)
			b.state = b.csEvltr.recordTransition(connectivity.Shutdown, connectivity.Idle)
			b.subConns[aNoAttrs] = scs
			b.scStates[sc] = scs
			addrsUpdated = true
		} else {
			// Always update the subconn's address in case the attributes
			// changed. The SubConn does a reflect.DeepEqual of the new and old
			// addresses. So this is a noop if the current address is the same
			// as the old one (including attributes).
			b.subConns[aNoAttrs] = scInfo
			b.cc.UpdateAddresses(scInfo.sc, []resolver.Address{a})
		}
	}
	for a, scInfo := range b.subConns {
		// a was removed by resolver.
		if _, ok := addrsSet[a]; !ok {
			b.cc.RemoveSubConn(scInfo.sc)
			delete(b.subConns, a)
			addrsUpdated = true
			// Keep the state of this sc in b.scStates until sc's state becomes Shutdown.
			// The entry will be deleted in UpdateSubConnState.
		}
	}
	return addrsUpdated
}

func (b *ringhashBalancer) UpdateClientConnState(s balancer.ClientConnState) error {
	b.logger.Infof("Received update from resolver, balancer config: %+v", pretty.ToJSON(s.BalancerConfig))
	if b.config == nil {
		newConfig, ok := s.BalancerConfig.(*LBConfig)
		if !ok {
			return fmt.Errorf("unexpected balancer config with type: %T", s.BalancerConfig)
		}
		b.config = newConfig
	}

	// Successful resolution; clear resolver error and ensure we return nil.
	b.resolverErr = nil
	if b.updateAddresses(s.ResolverState.Addresses) {
		// If addresses were updated, no matter whether it resulted in SubConn
		// creation/deletion, or just weight update, we will need to regenerate
		// the ring.
		var err error
		b.ring, err = newRing(b.subConns, b.config.MinRingSize, b.config.MaxRingSize)
		if err != nil {
			panic(err)
		}
		b.regeneratePicker()
		b.cc.UpdateState(balancer.State{ConnectivityState: b.state, Picker: b.picker})
	}

	// If resolver state contains no addresses, return an error so ClientConn
	// will trigger re-resolve. Also records this as an resolver error, so when
	// the overall state turns transient failure, the error message will have
	// the zero address information.
	if len(s.ResolverState.Addresses) == 0 {
		b.ResolverError(errors.New("produced zero addresses"))
		return balancer.ErrBadResolverState
	}
	return nil
}

func (b *ringhashBalancer) ResolverError(err error) {
	b.resolverErr = err
	if len(b.subConns) == 0 {
		b.state = connectivity.TransientFailure
	}

	if b.state != connectivity.TransientFailure {
		// The picker will not change since the balancer does not currently
		// report an error.
		return
	}
	b.regeneratePicker()
	b.cc.UpdateState(balancer.State{
		ConnectivityState: b.state,
		Picker:            b.picker,
	})
}

// UpdateSubConnState updates the per-SubConn state stored in the ring, and also
// the aggregated state.
//
// It triggers an update to cc when:
// - the new state is TransientFailure, to update the error message
//   - it's possible that this is a noop, but sending an extra update is easier
//   than comparing errors
// - the aggregated state is changed
//   - the same picker will be sent again, but this update may trigger a re-pick
//   for some RPCs.
func (b *ringhashBalancer) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	s := state.ConnectivityState
	b.logger.Infof("handle SubConn state change: %p, %v", sc, s)
	scs, ok := b.scStates[sc]
	if !ok {
		b.logger.Infof("got state changes for an unknown SubConn: %p, %v", sc, s)
		return
	}
	oldSCState := scs.effectiveState()
	scs.setState(s)
	newSCState := scs.effectiveState()

	var sendUpdate bool
	oldBalancerState := b.state
	b.state = b.csEvltr.recordTransition(oldSCState, newSCState)
	if oldBalancerState != b.state {
		sendUpdate = true
	}

	switch s {
	case connectivity.Idle:
		// When the overall state is TransientFailure, this will never get picks
		// if there's a lower priority. Need to keep the SubConns connecting so
		// there's a chance it will recover.
		if b.state == connectivity.TransientFailure {
			scs.queueConnect()
		}
		// No need to send an update. No queued RPC can be unblocked. If the
		// overall state changed because of this, sendUpdate is already true.
	case connectivity.Connecting:
		// No need to send an update. No queued RPC can be unblocked. If the
		// overall state changed because of this, sendUpdate is already true.
	case connectivity.Ready:
		// Resend the picker, there's no need to regenerate the picker because
		// the ring didn't change.
		sendUpdate = true
	case connectivity.TransientFailure:
		// Save error to be reported via picker.
		b.connErr = state.ConnectionError
		// Regenerate picker to update error message.
		b.regeneratePicker()
		sendUpdate = true
	case connectivity.Shutdown:
		// When an address was removed by resolver, b called RemoveSubConn but
		// kept the sc's state in scStates. Remove state for this sc here.
		delete(b.scStates, sc)
	}

	if sendUpdate {
		b.cc.UpdateState(balancer.State{ConnectivityState: b.state, Picker: b.picker})
	}
}

// mergeErrors builds an error from the last connection error and the last
// resolver error.  Must only be called if b.state is TransientFailure.
func (b *ringhashBalancer) mergeErrors() error {
	// connErr must always be non-nil unless there are no SubConns, in which
	// case resolverErr must be non-nil.
	if b.connErr == nil {
		return fmt.Errorf("last resolver error: %v", b.resolverErr)
	}
	if b.resolverErr == nil {
		return fmt.Errorf("last connection error: %v", b.connErr)
	}
	return fmt.Errorf("last connection error: %v; last resolver error: %v", b.connErr, b.resolverErr)
}

func (b *ringhashBalancer) regeneratePicker() {
	if b.state == connectivity.TransientFailure {
		b.picker = base.NewErrPicker(b.mergeErrors())
		return
	}
	b.picker = newPicker(b.ring, b.logger)
}

func (b *ringhashBalancer) Close() {}

// connectivityStateEvaluator takes the connectivity states of multiple SubConns
// and returns one aggregated connectivity state.
//
// It's not thread safe.
type connectivityStateEvaluator struct {
	nums [5]uint64
}

// recordTransition records state change happening in subConn and based on that
// it evaluates what aggregated state should be.
//
// - If there is at least one subchannel in READY state, report READY.
// - If there are 2 or more subchannels in TRANSIENT_FAILURE state, report TRANSIENT_FAILURE.
// - If there is at least one subchannel in CONNECTING state, report CONNECTING.
// - If there is at least one subchannel in Idle state, report Idle.
// - Otherwise, report TRANSIENT_FAILURE.
//
// Note that if there are 1 connecting, 2 transient failure, the overall state
// is transient failure. This is because the second transient failure is a
// fallback of the first failing SubConn, and we want to report transient
// failure to failover to the lower priority.
func (cse *connectivityStateEvaluator) recordTransition(oldState, newState connectivity.State) connectivity.State {
	// Update counters.
	for idx, state := range []connectivity.State{oldState, newState} {
		updateVal := 2*uint64(idx) - 1 // -1 for oldState and +1 for new.
		cse.nums[state] += updateVal
	}

	if cse.nums[connectivity.Ready] > 0 {
		return connectivity.Ready
	}
	if cse.nums[connectivity.TransientFailure] > 1 {
		return connectivity.TransientFailure
	}
	if cse.nums[connectivity.Connecting] > 0 {
		return connectivity.Connecting
	}
	if cse.nums[connectivity.Idle] > 0 {
		return connectivity.Idle
	}
	return connectivity.TransientFailure
}
