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

// Package clusterresolver contains EDS balancer implementation.
package clusterresolver

import (
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/internal/buffer"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/grpcsync"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
	"google.golang.org/grpc/xds/internal/balancer/priority"
	"google.golang.org/grpc/xds/internal/xdsclient"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

// Name is the name of the cluster_resolver balancer.
const Name = "cluster_resolver_experimental"

var (
	errBalancerClosed = errors.New("cdsBalancer is closed")
	newChildBalancer  = func(bb balancer.Builder, cc balancer.ClientConn, o balancer.BuildOptions) balancer.Balancer {
		return bb.Build(cc, o)
	}
)

func init() {
	balancer.Register(bb{})
}

type bb struct{}

// Build helps implement the balancer.Builder interface.
func (bb) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	priorityBuilder := balancer.Get(priority.Name)
	if priorityBuilder == nil {
		logger.Errorf("priority balancer is needed but not registered")
		return nil
	}
	priorityConfigParser, ok := priorityBuilder.(balancer.ConfigParser)
	if !ok {
		logger.Errorf("priority balancer builder is not a config parser")
		return nil
	}

	b := &clusterResolverBalancer{
		bOpts:    opts,
		updateCh: buffer.NewUnbounded(),
		closed:   grpcsync.NewEvent(),
		done:     grpcsync.NewEvent(),

		priorityBuilder:      priorityBuilder,
		priorityConfigParser: priorityConfigParser,
	}
	b.logger = prefixLogger(b)
	b.logger.Infof("Created")

	b.resourceWatcher = newResourceResolver(b)
	b.cc = &ccWrapper{
		ClientConn:      cc,
		resourceWatcher: b.resourceWatcher,
	}

	go b.run()
	return b
}

func (bb) Name() string {
	return Name
}

func (bb) ParseConfig(c json.RawMessage) (serviceconfig.LoadBalancingConfig, error) {
	var cfg LBConfig
	if err := json.Unmarshal(c, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal balancer config %s into cluster-resolver config, error: %v", string(c), err)
	}
	return &cfg, nil
}

// ccUpdate wraps a clientConn update received from gRPC (pushed from the
// xdsResolver).
type ccUpdate struct {
	state balancer.ClientConnState
	err   error
}

// scUpdate wraps a subConn update received from gRPC. This is directly passed
// on to the child balancer.
type scUpdate struct {
	subConn balancer.SubConn
	state   balancer.SubConnState
}

type exitIdle struct{}

// clusterResolverBalancer manages xdsClient and the actual EDS balancer implementation that
// does load balancing.
//
// It currently has only an clusterResolverBalancer. Later, we may add fallback.
type clusterResolverBalancer struct {
	cc              balancer.ClientConn
	bOpts           balancer.BuildOptions
	updateCh        *buffer.Unbounded // Channel for updates from gRPC.
	resourceWatcher *resourceResolver
	logger          *grpclog.PrefixLogger
	closed          *grpcsync.Event
	done            *grpcsync.Event

	priorityBuilder      balancer.Builder
	priorityConfigParser balancer.ConfigParser

	config          *LBConfig
	configRaw       *serviceconfig.ParseResult
	xdsClient       xdsclient.XDSClient    // xDS client to watch EDS resource.
	attrsWithClient *attributes.Attributes // Attributes with xdsClient attached to be passed to the child policies.

	child               balancer.Balancer
	priorities          []priorityConfig
	watchUpdateReceived bool
}

// handleClientConnUpdate handles a ClientConnUpdate received from gRPC. Good
// updates lead to registration of EDS and DNS watches. Updates with error lead
// to cancellation of existing watch and propagation of the same error to the
// child balancer.
func (b *clusterResolverBalancer) handleClientConnUpdate(update *ccUpdate) {
	// We first handle errors, if any, and then proceed with handling the
	// update, only if the status quo has changed.
	if err := update.err; err != nil {
		b.handleErrorFromUpdate(err, true)
		return
	}

	b.logger.Infof("Receive update from resolver, balancer config: %v", pretty.ToJSON(update.state.BalancerConfig))
	cfg, _ := update.state.BalancerConfig.(*LBConfig)
	if cfg == nil {
		b.logger.Warningf("xds: unexpected LoadBalancingConfig type: %T", update.state.BalancerConfig)
		return
	}

	b.config = cfg
	b.configRaw = update.state.ResolverState.ServiceConfig
	b.resourceWatcher.updateMechanisms(cfg.DiscoveryMechanisms)

	if !b.watchUpdateReceived {
		// If update was not received, wait for it.
		return
	}
	// If eds resp was received before this, the child policy was created. We
	// need to generate a new balancer config and send it to the child, because
	// certain fields (unrelated to EDS watch) might have changed.
	if err := b.updateChildConfig(); err != nil {
		b.logger.Warningf("failed to update child policy config: %v", err)
	}
}

// handleWatchUpdate handles a watch update from the xDS Client. Good updates
// lead to clientConn updates being invoked on the underlying child balancer.
func (b *clusterResolverBalancer) handleWatchUpdate(update *resourceUpdate) {
	if err := update.err; err != nil {
		b.logger.Warningf("Watch error from xds-client %p: %v", b.xdsClient, err)
		b.handleErrorFromUpdate(err, false)
		return
	}

	b.logger.Infof("resource update: %+v", pretty.ToJSON(update.priorities))
	b.watchUpdateReceived = true
	b.priorities = update.priorities

	// A new EDS update triggers new child configs (e.g. different priorities
	// for the priority balancer), and new addresses (the endpoints come from
	// the EDS response).
	if err := b.updateChildConfig(); err != nil {
		b.logger.Warningf("failed to update child policy's balancer config: %v", err)
	}
}

// updateChildConfig builds a balancer config from eb's cached eds resp and
// service config, and sends that to the child balancer. Note that it also
// generates the addresses, because the endpoints come from the EDS resp.
//
// If child balancer doesn't already exist, one will be created.
func (b *clusterResolverBalancer) updateChildConfig() error {
	// Child was build when the first EDS resp was received, so we just build
	// the config and addresses.
	if b.child == nil {
		b.child = newChildBalancer(b.priorityBuilder, b.cc, b.bOpts)
	}

	childCfgBytes, addrs, err := buildPriorityConfigJSON(b.priorities, b.config.XDSLBPolicy)
	if err != nil {
		return fmt.Errorf("failed to build priority balancer config: %v", err)
	}
	childCfg, err := b.priorityConfigParser.ParseConfig(childCfgBytes)
	if err != nil {
		return fmt.Errorf("failed to parse generated priority balancer config, this should never happen because the config is generated: %v", err)
	}
	b.logger.Infof("build balancer config: %v", pretty.ToJSON(childCfg))
	return b.child.UpdateClientConnState(balancer.ClientConnState{
		ResolverState: resolver.State{
			Addresses:     addrs,
			ServiceConfig: b.configRaw,
			Attributes:    b.attrsWithClient,
		},
		BalancerConfig: childCfg,
	})
}

// handleErrorFromUpdate handles both the error from parent ClientConn (from CDS
// balancer) and the error from xds client (from the watcher). fromParent is
// true if error is from parent ClientConn.
//
// If the error is connection error, it should be handled for fallback purposes.
//
// If the error is resource-not-found:
// - If it's from CDS balancer (shows as a resolver error), it means LDS or CDS
// resources were removed. The EDS watch should be canceled.
// - If it's from xds client, it means EDS resource were removed. The EDS
// watcher should keep watching.
// In both cases, the sub-balancers will be receive the error.
func (b *clusterResolverBalancer) handleErrorFromUpdate(err error, fromParent bool) {
	b.logger.Warningf("Received error: %v", err)
	if fromParent && xdsresource.ErrType(err) == xdsresource.ErrorTypeResourceNotFound {
		// This is an error from the parent ClientConn (can be the parent CDS
		// balancer), and is a resource-not-found error. This means the resource
		// (can be either LDS or CDS) was removed. Stop the EDS watch.
		b.resourceWatcher.stop()
	}
	if b.child != nil {
		b.child.ResolverError(err)
	} else {
		// If eds balancer was never created, fail the RPCs with errors.
		b.cc.UpdateState(balancer.State{
			ConnectivityState: connectivity.TransientFailure,
			Picker:            base.NewErrPicker(err),
		})
	}

}

// run is a long-running goroutine which handles all updates from gRPC and
// xdsClient. All methods which are invoked directly by gRPC or xdsClient simply
// push an update onto a channel which is read and acted upon right here.
func (b *clusterResolverBalancer) run() {
	for {
		select {
		case u := <-b.updateCh.Get():
			b.updateCh.Load()
			switch update := u.(type) {
			case *ccUpdate:
				b.handleClientConnUpdate(update)
			case *scUpdate:
				// SubConn updates are simply handed over to the underlying
				// child balancer.
				if b.child == nil {
					b.logger.Errorf("xds: received scUpdate {%+v} with no child balancer", update)
					break
				}
				b.child.UpdateSubConnState(update.subConn, update.state)
			case exitIdle:
				if b.child == nil {
					b.logger.Errorf("xds: received ExitIdle with no child balancer")
					break
				}
				// This implementation assumes the child balancer supports
				// ExitIdle (but still checks for the interface's existence to
				// avoid a panic if not).  If the child does not, no subconns
				// will be connected.
				if ei, ok := b.child.(balancer.ExitIdler); ok {
					ei.ExitIdle()
				}
			}
		case u := <-b.resourceWatcher.updateChannel:
			b.handleWatchUpdate(u)

		// Close results in cancellation of the EDS watch and closing of the
		// underlying child policy and is the only way to exit this goroutine.
		case <-b.closed.Done():
			b.resourceWatcher.stop()

			if b.child != nil {
				b.child.Close()
				b.child = nil
			}
			// This is the *ONLY* point of return from this function.
			b.logger.Infof("Shutdown")
			b.done.Fire()
			return
		}
	}
}

// Following are methods to implement the balancer interface.

// UpdateClientConnState receives the serviceConfig (which contains the
// clusterName to watch for in CDS) and the xdsClient object from the
// xdsResolver.
func (b *clusterResolverBalancer) UpdateClientConnState(state balancer.ClientConnState) error {
	if b.closed.HasFired() {
		b.logger.Warningf("xds: received ClientConnState {%+v} after clusterResolverBalancer was closed", state)
		return errBalancerClosed
	}

	if b.xdsClient == nil {
		c := xdsclient.FromResolverState(state.ResolverState)
		if c == nil {
			return balancer.ErrBadResolverState
		}
		b.xdsClient = c
		b.attrsWithClient = state.ResolverState.Attributes
	}

	b.updateCh.Put(&ccUpdate{state: state})
	return nil
}

// ResolverError handles errors reported by the xdsResolver.
func (b *clusterResolverBalancer) ResolverError(err error) {
	if b.closed.HasFired() {
		b.logger.Warningf("xds: received resolver error {%v} after clusterResolverBalancer was closed", err)
		return
	}
	b.updateCh.Put(&ccUpdate{err: err})
}

// UpdateSubConnState handles subConn updates from gRPC.
func (b *clusterResolverBalancer) UpdateSubConnState(sc balancer.SubConn, state balancer.SubConnState) {
	if b.closed.HasFired() {
		b.logger.Warningf("xds: received subConn update {%v, %v} after clusterResolverBalancer was closed", sc, state)
		return
	}
	b.updateCh.Put(&scUpdate{subConn: sc, state: state})
}

// Close closes the cdsBalancer and the underlying child balancer.
func (b *clusterResolverBalancer) Close() {
	b.closed.Fire()
	<-b.done.Done()
}

func (b *clusterResolverBalancer) ExitIdle() {
	b.updateCh.Put(exitIdle{})
}

// ccWrapper overrides ResolveNow(), so that re-resolution from the child
// policies will trigger the DNS resolver in cluster_resolver balancer.
type ccWrapper struct {
	balancer.ClientConn
	resourceWatcher *resourceResolver
}

func (c *ccWrapper) ResolveNow(resolver.ResolveNowOptions) {
	c.resourceWatcher.resolveNow()
}
