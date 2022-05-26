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

package clustermanager

import (
	"fmt"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/internal/grpclog"
)

type subBalancerState struct {
	state balancer.State
	// stateToAggregate is the connectivity state used only for state
	// aggregation. It could be different from state.ConnectivityState. For
	// example when a sub-balancer transitions from TransientFailure to
	// connecting, state.ConnectivityState is Connecting, but stateToAggregate
	// is still TransientFailure.
	stateToAggregate connectivity.State
}

func (s *subBalancerState) String() string {
	return fmt.Sprintf("picker:%p,state:%v,stateToAggregate:%v", s.state.Picker, s.state.ConnectivityState, s.stateToAggregate)
}

type balancerStateAggregator struct {
	cc     balancer.ClientConn
	logger *grpclog.PrefixLogger

	mu sync.Mutex
	// If started is false, no updates should be sent to the parent cc. A closed
	// sub-balancer could still send pickers to this aggregator. This makes sure
	// that no updates will be forwarded to parent when the whole balancer group
	// and states aggregator is closed.
	started bool
	// All balancer IDs exist as keys in this map, even if balancer group is not
	// started.
	//
	// If an ID is not in map, it's either removed or never added.
	idToPickerState map[string]*subBalancerState
}

func newBalancerStateAggregator(cc balancer.ClientConn, logger *grpclog.PrefixLogger) *balancerStateAggregator {
	return &balancerStateAggregator{
		cc:              cc,
		logger:          logger,
		idToPickerState: make(map[string]*subBalancerState),
	}
}

// Start starts the aggregator. It can be called after Close to restart the
// aggretator.
func (bsa *balancerStateAggregator) start() {
	bsa.mu.Lock()
	defer bsa.mu.Unlock()
	bsa.started = true
}

// Close closes the aggregator. When the aggregator is closed, it won't call
// parent ClientConn to update balancer state.
func (bsa *balancerStateAggregator) close() {
	bsa.mu.Lock()
	defer bsa.mu.Unlock()
	bsa.started = false
	bsa.clearStates()
}

// add adds a sub-balancer state with weight. It adds a place holder, and waits
// for the real sub-balancer to update state.
//
// This is called when there's a new child.
func (bsa *balancerStateAggregator) add(id string) {
	bsa.mu.Lock()
	defer bsa.mu.Unlock()
	bsa.idToPickerState[id] = &subBalancerState{
		// Start everything in CONNECTING, so if one of the sub-balancers
		// reports TransientFailure, the RPCs will still wait for the other
		// sub-balancers.
		state: balancer.State{
			ConnectivityState: connectivity.Connecting,
			Picker:            base.NewErrPicker(balancer.ErrNoSubConnAvailable),
		},
		stateToAggregate: connectivity.Connecting,
	}
}

// remove removes the sub-balancer state. Future updates from this sub-balancer,
// if any, will be ignored.
//
// This is called when a child is removed.
func (bsa *balancerStateAggregator) remove(id string) {
	bsa.mu.Lock()
	defer bsa.mu.Unlock()
	if _, ok := bsa.idToPickerState[id]; !ok {
		return
	}
	// Remove id and picker from picker map. This also results in future updates
	// for this ID to be ignored.
	delete(bsa.idToPickerState, id)
}

// UpdateState is called to report a balancer state change from sub-balancer.
// It's usually called by the balancer group.
//
// It calls parent ClientConn's UpdateState with the new aggregated state.
func (bsa *balancerStateAggregator) UpdateState(id string, state balancer.State) {
	bsa.mu.Lock()
	defer bsa.mu.Unlock()
	pickerSt, ok := bsa.idToPickerState[id]
	if !ok {
		// All state starts with an entry in pickStateMap. If ID is not in map,
		// it's either removed, or never existed.
		return
	}
	if !(pickerSt.state.ConnectivityState == connectivity.TransientFailure && state.ConnectivityState == connectivity.Connecting) {
		// If old state is TransientFailure, and new state is Connecting, don't
		// update the state, to prevent the aggregated state from being always
		// CONNECTING. Otherwise, stateToAggregate is the same as
		// state.ConnectivityState.
		pickerSt.stateToAggregate = state.ConnectivityState
	}
	pickerSt.state = state

	if !bsa.started {
		return
	}
	bsa.cc.UpdateState(bsa.build())
}

// clearState Reset everything to init state (Connecting) but keep the entry in
// map (to keep the weight).
//
// Caller must hold bsa.mu.
func (bsa *balancerStateAggregator) clearStates() {
	for _, pState := range bsa.idToPickerState {
		pState.state = balancer.State{
			ConnectivityState: connectivity.Connecting,
			Picker:            base.NewErrPicker(balancer.ErrNoSubConnAvailable),
		}
		pState.stateToAggregate = connectivity.Connecting
	}
}

// buildAndUpdate combines the sub-state from each sub-balancer into one state,
// and update it to parent ClientConn.
func (bsa *balancerStateAggregator) buildAndUpdate() {
	bsa.mu.Lock()
	defer bsa.mu.Unlock()
	if !bsa.started {
		return
	}
	bsa.cc.UpdateState(bsa.build())
}

// build combines sub-states into one. The picker will do a child pick.
//
// Caller must hold bsa.mu.
func (bsa *balancerStateAggregator) build() balancer.State {
	// TODO: the majority of this function (and UpdateState) is exactly the same
	// as weighted_target's state aggregator. Try to make a general utility
	// function/struct to handle the logic.
	//
	// One option: make a SubBalancerState that handles Update(State), including
	// handling the special connecting after ready, as in UpdateState(). Then a
	// function to calculate the aggregated connectivity state as in this
	// function.
	//
	// TODO: use balancer.ConnectivityStateEvaluator to calculate the aggregated
	// state.
	var readyN, connectingN, idleN int
	for _, ps := range bsa.idToPickerState {
		switch ps.stateToAggregate {
		case connectivity.Ready:
			readyN++
		case connectivity.Connecting:
			connectingN++
		case connectivity.Idle:
			idleN++
		}
	}
	var aggregatedState connectivity.State
	switch {
	case readyN > 0:
		aggregatedState = connectivity.Ready
	case connectingN > 0:
		aggregatedState = connectivity.Connecting
	case idleN > 0:
		aggregatedState = connectivity.Idle
	default:
		aggregatedState = connectivity.TransientFailure
	}

	// The picker's return error might not be consistent with the
	// aggregatedState. Because for this LB policy, we want to always build
	// picker with all sub-pickers (not only ready sub-pickers), so even if the
	// overall state is Ready, pick for certain RPCs can behave like Connecting
	// or TransientFailure.
	bsa.logger.Infof("Child pickers: %+v", bsa.idToPickerState)
	return balancer.State{
		ConnectivityState: aggregatedState,
		Picker:            newPickerGroup(bsa.idToPickerState),
	}
}
