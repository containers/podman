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

// Package priority implements the priority balancer.
//
// This balancer will be kept in internal until we use it in the xds balancers,
// and are confident its functionalities are stable. It will then be exported
// for more users.
package priority

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/internal/balancergroup"
	"google.golang.org/grpc/internal/buffer"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/grpcsync"
	"google.golang.org/grpc/internal/hierarchy"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
)

// Name is the name of the priority balancer.
const Name = "priority_experimental"

func init() {
	balancer.Register(bb{})
}

type bb struct{}

func (bb) Build(cc balancer.ClientConn, bOpts balancer.BuildOptions) balancer.Balancer {
	b := &priorityBalancer{
		cc:                       cc,
		done:                     grpcsync.NewEvent(),
		childToPriority:          make(map[string]int),
		children:                 make(map[string]*childBalancer),
		childBalancerStateUpdate: buffer.NewUnbounded(),
	}

	b.logger = prefixLogger(b)
	b.bg = balancergroup.New(cc, bOpts, b, b.logger)
	b.bg.Start()
	go b.run()
	b.logger.Infof("Created")
	return b
}

func (b bb) ParseConfig(s json.RawMessage) (serviceconfig.LoadBalancingConfig, error) {
	return parseConfig(s)
}

func (bb) Name() string {
	return Name
}

// timerWrapper wraps a timer with a boolean. So that when a race happens
// between AfterFunc and Stop, the func is guaranteed to not execute.
type timerWrapper struct {
	stopped bool
	timer   *time.Timer
}

type priorityBalancer struct {
	logger                   *grpclog.PrefixLogger
	cc                       balancer.ClientConn
	bg                       *balancergroup.BalancerGroup
	done                     *grpcsync.Event
	childBalancerStateUpdate *buffer.Unbounded

	mu         sync.Mutex
	childInUse string
	// priority of the child that's current in use. Int starting from 0, and 0
	// is the higher priority.
	priorityInUse int
	// priorities is a list of child names from higher to lower priority.
	priorities []string
	// childToPriority is a map from the child name to it's priority. Priority
	// is an int start from 0, and 0 is the higher priority.
	childToPriority map[string]int
	// children is a map from child name to sub-balancers.
	children map[string]*childBalancer
	// The timer to give a priority some time to connect. And if the priority
	// doesn't go into Ready/Failure, the next priority will be started.
	//
	// One timer is enough because there can be at most one priority in init
	// state.
	priorityInitTimer *timerWrapper
}

func (b *priorityBalancer) UpdateClientConnState(s balancer.ClientConnState) error {
	b.logger.Infof("Received update from resolver, balancer config: %+v", pretty.ToJSON(s.BalancerConfig))
	newConfig, ok := s.BalancerConfig.(*LBConfig)
	if !ok {
		return fmt.Errorf("unexpected balancer config with type: %T", s.BalancerConfig)
	}
	addressesSplit := hierarchy.Group(s.ResolverState.Addresses)

	b.mu.Lock()
	defer b.mu.Unlock()
	// Create and remove children, since we know all children from the config
	// are used by some priority.
	for name, newSubConfig := range newConfig.Children {
		bb := balancer.Get(newSubConfig.Config.Name)
		if bb == nil {
			b.logger.Errorf("balancer name %v from config is not registered", newSubConfig.Config.Name)
			continue
		}

		currentChild, ok := b.children[name]
		if !ok {
			// This is a new child, add it to the children list. But note that
			// the balancer isn't built, because this child can be a low
			// priority. If necessary, it will be built when syncing priorities.
			cb := newChildBalancer(name, b, bb)
			cb.updateConfig(newSubConfig, resolver.State{
				Addresses:     addressesSplit[name],
				ServiceConfig: s.ResolverState.ServiceConfig,
				Attributes:    s.ResolverState.Attributes,
			})
			b.children[name] = cb
			continue
		}

		// This is not a new child. But the config/addresses could change.

		// The balancing policy name is changed, close the old child. But don't
		// rebuild, rebuild will happen when syncing priorities.
		if currentChild.bb.Name() != bb.Name() {
			currentChild.stop()
			currentChild.updateBuilder(bb)
		}

		// Update config and address, but note that this doesn't send the
		// updates to child balancer (the child balancer might not be built, if
		// it's a low priority).
		currentChild.updateConfig(newSubConfig, resolver.State{
			Addresses:     addressesSplit[name],
			ServiceConfig: s.ResolverState.ServiceConfig,
			Attributes:    s.ResolverState.Attributes,
		})
	}

	// Remove child from children if it's not in new config.
	for name, oldChild := range b.children {
		if _, ok := newConfig.Children[name]; !ok {
			oldChild.stop()
		}
	}

	// Update priorities and handle priority changes.
	b.priorities = newConfig.Priorities
	b.childToPriority = make(map[string]int, len(newConfig.Priorities))
	for pi, pName := range newConfig.Priorities {
		b.childToPriority[pName] = pi
	}
	// Sync the states of all children to the new updated priorities. This
	// include starting/stopping child balancers when necessary.
	b.syncPriority()

	return nil
}

func (b *priorityBalancer) ResolverError(err error) {
	b.bg.ResolverError(err)
}

func (b *priorityBalancer) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	b.bg.UpdateSubConnState(sc, state)
}

func (b *priorityBalancer) Close() {
	b.bg.Close()

	b.mu.Lock()
	defer b.mu.Unlock()
	b.done.Fire()
	// Clear states of the current child in use, so if there's a race in picker
	// update, it will be dropped.
	b.childInUse = ""
	b.stopPriorityInitTimer()
}

func (b *priorityBalancer) ExitIdle() {
	b.bg.ExitIdle()
}

// stopPriorityInitTimer stops the priorityInitTimer if it's not nil, and set it
// to nil.
//
// Caller must hold b.mu.
func (b *priorityBalancer) stopPriorityInitTimer() {
	timerW := b.priorityInitTimer
	if timerW == nil {
		return
	}
	b.priorityInitTimer = nil
	timerW.stopped = true
	timerW.timer.Stop()
}

// UpdateState implements balancergroup.BalancerStateAggregator interface. The
// balancer group sends new connectivity state and picker here.
func (b *priorityBalancer) UpdateState(childName string, state balancer.State) {
	b.childBalancerStateUpdate.Put(&childBalancerState{
		name: childName,
		s:    state,
	})
}

type childBalancerState struct {
	name string
	s    balancer.State
}

// run handles child update in a separate goroutine, so if the child sends
// updates inline (when called by parent), it won't cause deadlocks (by trying
// to hold the same mutex).
func (b *priorityBalancer) run() {
	for {
		select {
		case u := <-b.childBalancerStateUpdate.Get():
			b.childBalancerStateUpdate.Load()
			s := u.(*childBalancerState)
			// Needs to handle state update in a goroutine, because each state
			// update needs to start/close child policy, could result in
			// deadlock.
			b.handleChildStateUpdate(s.name, s.s)
		case <-b.done.Done():
			return
		}
	}
}
