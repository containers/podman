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

// Package clusterimpl implements the xds_cluster_impl balancing policy. It
// handles the cluster features (e.g. circuit_breaking, RPC dropping).
//
// Note that it doesn't handle name resolution, which is done by policy
// xds_cluster_resolver.
package clusterimpl

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/internal"
	"google.golang.org/grpc/internal/buffer"
	"google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/grpcsync"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"
	xdsinternal "google.golang.org/grpc/xds/internal"
	"google.golang.org/grpc/xds/internal/balancer/loadstore"
	"google.golang.org/grpc/xds/internal/xdsclient"
	"google.golang.org/grpc/xds/internal/xdsclient/load"
)

const (
	// Name is the name of the cluster_impl balancer.
	Name                   = "xds_cluster_impl_experimental"
	defaultRequestCountMax = 1024
)

func init() {
	balancer.Register(bb{})
}

type bb struct{}

func (bb) Build(cc balancer.ClientConn, bOpts balancer.BuildOptions) balancer.Balancer {
	b := &clusterImplBalancer{
		ClientConn:      cc,
		bOpts:           bOpts,
		closed:          grpcsync.NewEvent(),
		done:            grpcsync.NewEvent(),
		loadWrapper:     loadstore.NewWrapper(),
		scWrappers:      make(map[balancer.SubConn]*scWrapper),
		pickerUpdateCh:  buffer.NewUnbounded(),
		requestCountMax: defaultRequestCountMax,
	}
	b.logger = prefixLogger(b)
	go b.run()
	b.logger.Infof("Created")
	return b
}

func (bb) Name() string {
	return Name
}

func (bb) ParseConfig(c json.RawMessage) (serviceconfig.LoadBalancingConfig, error) {
	return parseConfig(c)
}

type clusterImplBalancer struct {
	balancer.ClientConn

	// mu guarantees mutual exclusion between Close() and handling of picker
	// update to the parent ClientConn in run(). It's to make sure that the
	// run() goroutine doesn't send picker update to parent after the balancer
	// is closed.
	//
	// It's only used by the run() goroutine, but not the other exported
	// functions. Because the exported functions are guaranteed to be
	// synchronized with Close().
	mu     sync.Mutex
	closed *grpcsync.Event
	done   *grpcsync.Event

	bOpts     balancer.BuildOptions
	logger    *grpclog.PrefixLogger
	xdsClient xdsclient.XDSClient

	config           *LBConfig
	childLB          balancer.Balancer
	cancelLoadReport func()
	edsServiceName   string
	lrsServerName    *string
	loadWrapper      *loadstore.Wrapper

	clusterNameMu sync.Mutex
	clusterName   string

	scWrappersMu sync.Mutex
	// The SubConns passed to the child policy are wrapped in a wrapper, to keep
	// locality ID. But when the parent ClientConn sends updates, it's going to
	// give the original SubConn, not the wrapper. But the child policies only
	// know about the wrapper, so when forwarding SubConn updates, they must be
	// sent for the wrappers.
	//
	// This keeps a map from original SubConn to wrapper, so that when
	// forwarding the SubConn state update, the child policy will get the
	// wrappers.
	scWrappers map[balancer.SubConn]*scWrapper

	// childState/drops/requestCounter keeps the state used by the most recently
	// generated picker. All fields can only be accessed in run(). And run() is
	// the only goroutine that sends picker to the parent ClientConn. All
	// requests to update picker need to be sent to pickerUpdateCh.
	childState            balancer.State
	dropCategories        []DropConfig // The categories for drops.
	drops                 []*dropper
	requestCounterCluster string // The cluster name for the request counter.
	requestCounterService string // The service name for the request counter.
	requestCounter        *xdsclient.ClusterRequestsCounter
	requestCountMax       uint32
	pickerUpdateCh        *buffer.Unbounded
}

// updateLoadStore checks the config for load store, and decides whether it
// needs to restart the load reporting stream.
func (b *clusterImplBalancer) updateLoadStore(newConfig *LBConfig) error {
	var updateLoadClusterAndService bool

	// ClusterName is different, restart. ClusterName is from ClusterName and
	// EDSServiceName.
	clusterName := b.getClusterName()
	if clusterName != newConfig.Cluster {
		updateLoadClusterAndService = true
		b.setClusterName(newConfig.Cluster)
		clusterName = newConfig.Cluster
	}
	if b.edsServiceName != newConfig.EDSServiceName {
		updateLoadClusterAndService = true
		b.edsServiceName = newConfig.EDSServiceName
	}
	if updateLoadClusterAndService {
		// This updates the clusterName and serviceName that will be reported
		// for the loads. The update here is too early, the perfect timing is
		// when the picker is updated with the new connection. But from this
		// balancer's point of view, it's impossible to tell.
		//
		// On the other hand, this will almost never happen. Each LRS policy
		// shouldn't get updated config. The parent should do a graceful switch
		// when the clusterName or serviceName is changed.
		b.loadWrapper.UpdateClusterAndService(clusterName, b.edsServiceName)
	}

	var (
		stopOldLoadReport  bool
		startNewLoadReport bool
	)

	// Check if it's necessary to restart load report.
	if b.lrsServerName == nil {
		if newConfig.LoadReportingServerName != nil {
			// Old is nil, new is not nil, start new LRS.
			b.lrsServerName = newConfig.LoadReportingServerName
			startNewLoadReport = true
		}
		// Old is nil, new is nil, do nothing.
	} else if newConfig.LoadReportingServerName == nil {
		// Old is not nil, new is nil, stop old, don't start new.
		b.lrsServerName = newConfig.LoadReportingServerName
		stopOldLoadReport = true
	} else {
		// Old is not nil, new is not nil, compare string values, if
		// different, stop old and start new.
		if *b.lrsServerName != *newConfig.LoadReportingServerName {
			b.lrsServerName = newConfig.LoadReportingServerName
			stopOldLoadReport = true
			startNewLoadReport = true
		}
	}

	if stopOldLoadReport {
		if b.cancelLoadReport != nil {
			b.cancelLoadReport()
			b.cancelLoadReport = nil
			if !startNewLoadReport {
				// If a new LRS stream will be started later, no need to update
				// it to nil here.
				b.loadWrapper.UpdateLoadStore(nil)
			}
		}
	}
	if startNewLoadReport {
		var loadStore *load.Store
		if b.xdsClient != nil {
			loadStore, b.cancelLoadReport = b.xdsClient.ReportLoad(*b.lrsServerName)
		}
		b.loadWrapper.UpdateLoadStore(loadStore)
	}

	return nil
}

func (b *clusterImplBalancer) UpdateClientConnState(s balancer.ClientConnState) error {
	if b.closed.HasFired() {
		b.logger.Warningf("xds: received ClientConnState {%+v} after clusterImplBalancer was closed", s)
		return nil
	}

	b.logger.Infof("Received update from resolver, balancer config: %+v", pretty.ToJSON(s.BalancerConfig))
	newConfig, ok := s.BalancerConfig.(*LBConfig)
	if !ok {
		return fmt.Errorf("unexpected balancer config with type: %T", s.BalancerConfig)
	}

	// Need to check for potential errors at the beginning of this function, so
	// that on errors, we reject the whole config, instead of applying part of
	// it.
	bb := balancer.Get(newConfig.ChildPolicy.Name)
	if bb == nil {
		return fmt.Errorf("balancer %q not registered", newConfig.ChildPolicy.Name)
	}

	if b.xdsClient == nil {
		c := xdsclient.FromResolverState(s.ResolverState)
		if c == nil {
			return balancer.ErrBadResolverState
		}
		b.xdsClient = c
	}

	// Update load reporting config. This needs to be done before updating the
	// child policy because we need the loadStore from the updated client to be
	// passed to the ccWrapper, so that the next picker from the child policy
	// will pick up the new loadStore.
	if err := b.updateLoadStore(newConfig); err != nil {
		return err
	}

	// If child policy is a different type, recreate the sub-balancer.
	if b.config == nil || b.config.ChildPolicy.Name != newConfig.ChildPolicy.Name {
		if b.childLB != nil {
			b.childLB.Close()
		}
		b.childLB = bb.Build(b, b.bOpts)
	}
	b.config = newConfig

	if b.childLB == nil {
		// This is not an expected situation, and should be super rare in
		// practice.
		//
		// When this happens, we already applied all the other configurations
		// (drop/circuit breaking), but there's no child policy. This balancer
		// will be stuck, and we report the error to the parent.
		return fmt.Errorf("child policy is nil, this means balancer %q's Build() returned nil", newConfig.ChildPolicy.Name)
	}

	// Notify run() of this new config, in case drop and request counter need
	// update (which means a new picker needs to be generated).
	b.pickerUpdateCh.Put(newConfig)

	// Addresses and sub-balancer config are sent to sub-balancer.
	return b.childLB.UpdateClientConnState(balancer.ClientConnState{
		ResolverState:  s.ResolverState,
		BalancerConfig: b.config.ChildPolicy.Config,
	})
}

func (b *clusterImplBalancer) ResolverError(err error) {
	if b.closed.HasFired() {
		b.logger.Warningf("xds: received resolver error {%+v} after clusterImplBalancer was closed", err)
		return
	}

	if b.childLB != nil {
		b.childLB.ResolverError(err)
	}
}

func (b *clusterImplBalancer) UpdateSubConnState(sc balancer.SubConn, s balancer.SubConnState) {
	if b.closed.HasFired() {
		b.logger.Warningf("xds: received subconn state change {%+v, %+v} after clusterImplBalancer was closed", sc, s)
		return
	}

	// Trigger re-resolution when a SubConn turns transient failure. This is
	// necessary for the LogicalDNS in cluster_resolver policy to re-resolve.
	//
	// Note that this happens not only for the addresses from DNS, but also for
	// EDS (cluster_impl doesn't know if it's DNS or EDS, only the parent
	// knows). The parent priority policy is configured to ignore re-resolution
	// signal from the EDS children.
	if s.ConnectivityState == connectivity.TransientFailure {
		b.ClientConn.ResolveNow(resolver.ResolveNowOptions{})
	}

	b.scWrappersMu.Lock()
	if scw, ok := b.scWrappers[sc]; ok {
		sc = scw
		if s.ConnectivityState == connectivity.Shutdown {
			// Remove this SubConn from the map on Shutdown.
			delete(b.scWrappers, scw.SubConn)
		}
	}
	b.scWrappersMu.Unlock()
	if b.childLB != nil {
		b.childLB.UpdateSubConnState(sc, s)
	}
}

func (b *clusterImplBalancer) Close() {
	b.mu.Lock()
	b.closed.Fire()
	b.mu.Unlock()

	if b.childLB != nil {
		b.childLB.Close()
		b.childLB = nil
	}
	<-b.done.Done()
	b.logger.Infof("Shutdown")
}

func (b *clusterImplBalancer) ExitIdle() {
	if b.childLB == nil {
		return
	}
	if ei, ok := b.childLB.(balancer.ExitIdler); ok {
		ei.ExitIdle()
		return
	}
	// Fallback for children that don't support ExitIdle -- connect to all
	// SubConns.
	for _, sc := range b.scWrappers {
		sc.Connect()
	}
}

// Override methods to accept updates from the child LB.

func (b *clusterImplBalancer) UpdateState(state balancer.State) {
	// Instead of updating parent ClientConn inline, send state to run().
	b.pickerUpdateCh.Put(state)
}

func (b *clusterImplBalancer) setClusterName(n string) {
	b.clusterNameMu.Lock()
	defer b.clusterNameMu.Unlock()
	b.clusterName = n
}

func (b *clusterImplBalancer) getClusterName() string {
	b.clusterNameMu.Lock()
	defer b.clusterNameMu.Unlock()
	return b.clusterName
}

// scWrapper is a wrapper of SubConn with locality ID. The locality ID can be
// retrieved from the addresses when creating SubConn.
//
// All SubConns passed to the child policies are wrapped in this, so that the
// picker can get the localityID from the picked SubConn, and do load reporting.
//
// After wrapping, all SubConns to and from the parent ClientConn (e.g. for
// SubConn state update, update/remove SubConn) must be the original SubConns.
// All SubConns to and from the child policy (NewSubConn, forwarding SubConn
// state update) must be the wrapper. The balancer keeps a map from the original
// SubConn to the wrapper for this purpose.
type scWrapper struct {
	balancer.SubConn
	// locality needs to be atomic because it can be updated while being read by
	// the picker.
	locality atomic.Value // type xdsinternal.LocalityID
}

func (scw *scWrapper) updateLocalityID(lID xdsinternal.LocalityID) {
	scw.locality.Store(lID)
}

func (scw *scWrapper) localityID() xdsinternal.LocalityID {
	lID, _ := scw.locality.Load().(xdsinternal.LocalityID)
	return lID
}

func (b *clusterImplBalancer) NewSubConn(addrs []resolver.Address, opts balancer.NewSubConnOptions) (balancer.SubConn, error) {
	clusterName := b.getClusterName()
	newAddrs := make([]resolver.Address, len(addrs))
	var lID xdsinternal.LocalityID
	for i, addr := range addrs {
		newAddrs[i] = internal.SetXDSHandshakeClusterName(addr, clusterName)
		lID = xdsinternal.GetLocalityID(newAddrs[i])
	}
	sc, err := b.ClientConn.NewSubConn(newAddrs, opts)
	if err != nil {
		return nil, err
	}
	// Wrap this SubConn in a wrapper, and add it to the map.
	b.scWrappersMu.Lock()
	ret := &scWrapper{SubConn: sc}
	ret.updateLocalityID(lID)
	b.scWrappers[sc] = ret
	b.scWrappersMu.Unlock()
	return ret, nil
}

func (b *clusterImplBalancer) RemoveSubConn(sc balancer.SubConn) {
	scw, ok := sc.(*scWrapper)
	if !ok {
		b.ClientConn.RemoveSubConn(sc)
		return
	}
	// Remove the original SubConn from the parent ClientConn.
	//
	// Note that we don't remove this SubConn from the scWrappers map. We will
	// need it to forward the final SubConn state Shutdown to the child policy.
	//
	// This entry is kept in the map until it's state is changes to Shutdown,
	// and will be deleted in UpdateSubConnState().
	b.ClientConn.RemoveSubConn(scw.SubConn)
}

func (b *clusterImplBalancer) UpdateAddresses(sc balancer.SubConn, addrs []resolver.Address) {
	clusterName := b.getClusterName()
	newAddrs := make([]resolver.Address, len(addrs))
	var lID xdsinternal.LocalityID
	for i, addr := range addrs {
		newAddrs[i] = internal.SetXDSHandshakeClusterName(addr, clusterName)
		lID = xdsinternal.GetLocalityID(newAddrs[i])
	}
	if scw, ok := sc.(*scWrapper); ok {
		scw.updateLocalityID(lID)
		// Need to get the original SubConn from the wrapper before calling
		// parent ClientConn.
		sc = scw.SubConn
	}
	b.ClientConn.UpdateAddresses(sc, newAddrs)
}

type dropConfigs struct {
	drops           []*dropper
	requestCounter  *xdsclient.ClusterRequestsCounter
	requestCountMax uint32
}

// handleDropAndRequestCount compares drop and request counter in newConfig with
// the one currently used by picker. It returns a new dropConfigs if a new
// picker needs to be generated, otherwise it returns nil.
func (b *clusterImplBalancer) handleDropAndRequestCount(newConfig *LBConfig) *dropConfigs {
	// Compare new drop config. And update picker if it's changed.
	var updatePicker bool
	if !equalDropCategories(b.dropCategories, newConfig.DropCategories) {
		b.dropCategories = newConfig.DropCategories
		b.drops = make([]*dropper, 0, len(newConfig.DropCategories))
		for _, c := range newConfig.DropCategories {
			b.drops = append(b.drops, newDropper(c))
		}
		updatePicker = true
	}

	// Compare cluster name. And update picker if it's changed, because circuit
	// breaking's stream counter will be different.
	if b.requestCounterCluster != newConfig.Cluster || b.requestCounterService != newConfig.EDSServiceName {
		b.requestCounterCluster = newConfig.Cluster
		b.requestCounterService = newConfig.EDSServiceName
		b.requestCounter = xdsclient.GetClusterRequestsCounter(newConfig.Cluster, newConfig.EDSServiceName)
		updatePicker = true
	}
	// Compare upper bound of stream count. And update picker if it's changed.
	// This is also for circuit breaking.
	var newRequestCountMax uint32 = 1024
	if newConfig.MaxConcurrentRequests != nil {
		newRequestCountMax = *newConfig.MaxConcurrentRequests
	}
	if b.requestCountMax != newRequestCountMax {
		b.requestCountMax = newRequestCountMax
		updatePicker = true
	}

	if !updatePicker {
		return nil
	}
	return &dropConfigs{
		drops:           b.drops,
		requestCounter:  b.requestCounter,
		requestCountMax: b.requestCountMax,
	}
}

func (b *clusterImplBalancer) run() {
	defer b.done.Fire()
	for {
		select {
		case update := <-b.pickerUpdateCh.Get():
			b.pickerUpdateCh.Load()
			b.mu.Lock()
			if b.closed.HasFired() {
				b.mu.Unlock()
				return
			}
			switch u := update.(type) {
			case balancer.State:
				b.childState = u
				b.ClientConn.UpdateState(balancer.State{
					ConnectivityState: b.childState.ConnectivityState,
					Picker: newPicker(b.childState, &dropConfigs{
						drops:           b.drops,
						requestCounter:  b.requestCounter,
						requestCountMax: b.requestCountMax,
					}, b.loadWrapper),
				})
			case *LBConfig:
				dc := b.handleDropAndRequestCount(u)
				if dc != nil && b.childState.Picker != nil {
					b.ClientConn.UpdateState(balancer.State{
						ConnectivityState: b.childState.ConnectivityState,
						Picker:            newPicker(b.childState, dc, b.loadWrapper),
					})
				}
			}
			b.mu.Unlock()
		case <-b.closed.Done():
			if b.cancelLoadReport != nil {
				b.cancelLoadReport()
				b.cancelLoadReport = nil
			}
			return
		}
	}
}
