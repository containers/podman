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

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	controllerversion "google.golang.org/grpc/xds/internal/xdsclient/controller/version"
	xdsresourceversion "google.golang.org/grpc/xds/internal/xdsclient/controller/version"
	"google.golang.org/grpc/xds/internal/xdsclient/load"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

// AddWatch adds a watch for an xDS resource given its type and name.
func (t *Controller) AddWatch(rType xdsresource.ResourceType, resourceName string) {
	t.sendCh.Put(&watchAction{
		rType:    rType,
		remove:   false,
		resource: resourceName,
	})
}

// RemoveWatch cancels an already registered watch for an xDS resource
// given its type and name.
func (t *Controller) RemoveWatch(rType xdsresource.ResourceType, resourceName string) {
	t.sendCh.Put(&watchAction{
		rType:    rType,
		remove:   true,
		resource: resourceName,
	})
}

// run starts an ADS stream (and backs off exponentially, if the previous
// stream failed without receiving a single reply) and runs the sender and
// receiver routines to send and receive data from the stream respectively.
func (t *Controller) run(ctx context.Context) {
	go t.send(ctx)
	// TODO: start a goroutine monitoring ClientConn's connectivity state, and
	// report error (and log) when stats is transient failure.

	retries := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if retries != 0 {
			timer := time.NewTimer(t.backoff(retries))
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			}
		}

		retries++
		stream, err := t.vClient.NewStream(ctx, t.cc)
		if err != nil {
			t.updateHandler.NewConnectionError(err)
			t.logger.Warningf("xds: ADS stream creation failed: %v", err)
			continue
		}
		t.logger.Infof("ADS stream created")

		select {
		case <-t.streamCh:
		default:
		}
		t.streamCh <- stream
		if t.recv(stream) {
			retries = 0
		}
	}
}

// send is a separate goroutine for sending watch requests on the xds stream.
//
// It watches the stream channel for new streams, and the request channel for
// new requests to send on the stream.
//
// For each new request (watchAction), it's
//  - processed and added to the watch map
//    - so resend will pick them up when there are new streams
//  - sent on the current stream if there's one
//    - the current stream is cleared when any send on it fails
//
// For each new stream, all the existing requests will be resent.
//
// Note that this goroutine doesn't do anything to the old stream when there's a
// new one. In fact, there should be only one stream in progress, and new one
// should only be created when the old one fails (recv returns an error).
func (t *Controller) send(ctx context.Context) {
	var stream grpc.ClientStream
	for {
		select {
		case <-ctx.Done():
			return
		case stream = <-t.streamCh:
			if !t.sendExisting(stream) {
				// send failed, clear the current stream.
				stream = nil
			}
		case u := <-t.sendCh.Get():
			t.sendCh.Load()

			var (
				target                 []string
				rType                  xdsresource.ResourceType
				version, nonce, errMsg string
				send                   bool
			)
			switch update := u.(type) {
			case *watchAction:
				target, rType, version, nonce = t.processWatchInfo(update)
			case *ackAction:
				target, rType, version, nonce, send = t.processAckInfo(update, stream)
				if !send {
					continue
				}
				errMsg = update.errMsg
			}
			if stream == nil {
				// There's no stream yet. Skip the request. This request
				// will be resent to the new streams. If no stream is
				// created, the watcher will timeout (same as server not
				// sending response back).
				continue
			}
			if err := t.vClient.SendRequest(stream, target, rType, version, nonce, errMsg); err != nil {
				t.logger.Warningf("ADS request for {target: %q, type: %v, version: %q, nonce: %q} failed: %v", target, rType, version, nonce, err)
				// send failed, clear the current stream.
				stream = nil
			}
		}
	}
}

// sendExisting sends out xDS requests for registered watchers when recovering
// from a broken stream.
//
// We call stream.Send() here with the lock being held. It should be OK to do
// that here because the stream has just started and Send() usually returns
// quickly (once it pushes the message onto the transport layer) and is only
// ever blocked if we don't have enough flow control quota.
func (t *Controller) sendExisting(stream grpc.ClientStream) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Reset the ack versions when the stream restarts.
	t.versionMap = make(map[xdsresource.ResourceType]string)
	t.nonceMap = make(map[xdsresource.ResourceType]string)

	for rType, s := range t.watchMap {
		if err := t.vClient.SendRequest(stream, mapToSlice(s), rType, "", "", ""); err != nil {
			t.logger.Warningf("ADS request failed: %v", err)
			return false
		}
	}

	return true
}

// recv receives xDS responses on the provided ADS stream and branches out to
// message specific handlers.
func (t *Controller) recv(stream grpc.ClientStream) bool {
	success := false
	for {
		resp, err := t.vClient.RecvResponse(stream)
		if err != nil {
			t.updateHandler.NewConnectionError(err)
			t.logger.Warningf("ADS stream is closed with error: %v", err)
			return success
		}

		rType, version, nonce, err := t.handleResponse(resp)

		if e, ok := err.(xdsresourceversion.ErrResourceTypeUnsupported); ok {
			t.logger.Warningf("%s", e.ErrStr)
			continue
		}
		if err != nil {
			t.sendCh.Put(&ackAction{
				rType:   rType,
				version: "",
				nonce:   nonce,
				errMsg:  err.Error(),
				stream:  stream,
			})
			t.logger.Warningf("Sending NACK for response type: %v, version: %v, nonce: %v, reason: %v", rType, version, nonce, err)
			continue
		}
		t.sendCh.Put(&ackAction{
			rType:   rType,
			version: version,
			nonce:   nonce,
			stream:  stream,
		})
		t.logger.Infof("Sending ACK for response type: %v, version: %v, nonce: %v", rType, version, nonce)
		success = true
	}
}

func (t *Controller) handleResponse(resp proto.Message) (xdsresource.ResourceType, string, string, error) {
	rType, resource, version, nonce, err := t.vClient.ParseResponse(resp)
	if err != nil {
		return rType, version, nonce, err
	}
	opts := &xdsresource.UnmarshalOptions{
		Version:         version,
		Resources:       resource,
		Logger:          t.logger,
		UpdateValidator: t.updateValidator,
	}
	var md xdsresource.UpdateMetadata
	switch rType {
	case xdsresource.ListenerResource:
		var update map[string]xdsresource.ListenerUpdateErrTuple
		update, md, err = xdsresource.UnmarshalListener(opts)
		t.updateHandler.NewListeners(update, md)
	case xdsresource.RouteConfigResource:
		var update map[string]xdsresource.RouteConfigUpdateErrTuple
		update, md, err = xdsresource.UnmarshalRouteConfig(opts)
		t.updateHandler.NewRouteConfigs(update, md)
	case xdsresource.ClusterResource:
		var update map[string]xdsresource.ClusterUpdateErrTuple
		update, md, err = xdsresource.UnmarshalCluster(opts)
		t.updateHandler.NewClusters(update, md)
	case xdsresource.EndpointsResource:
		var update map[string]xdsresource.EndpointsUpdateErrTuple
		update, md, err = xdsresource.UnmarshalEndpoints(opts)
		t.updateHandler.NewEndpoints(update, md)
	default:
		return rType, "", "", xdsresourceversion.ErrResourceTypeUnsupported{
			ErrStr: fmt.Sprintf("Resource type %v unknown in response from server", rType),
		}
	}
	return rType, version, nonce, err
}

func mapToSlice(m map[string]bool) []string {
	ret := make([]string, 0, len(m))
	for i := range m {
		ret = append(ret, i)
	}
	return ret
}

type watchAction struct {
	rType    xdsresource.ResourceType
	remove   bool // Whether this is to remove watch for the resource.
	resource string
}

// processWatchInfo pulls the fields needed by the request from a watchAction.
//
// It also updates the watch map.
func (t *Controller) processWatchInfo(w *watchAction) (target []string, rType xdsresource.ResourceType, ver, nonce string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var current map[string]bool
	current, ok := t.watchMap[w.rType]
	if !ok {
		current = make(map[string]bool)
		t.watchMap[w.rType] = current
	}

	if w.remove {
		delete(current, w.resource)
		if len(current) == 0 {
			delete(t.watchMap, w.rType)
		}
	} else {
		current[w.resource] = true
	}

	rType = w.rType
	target = mapToSlice(current)
	// We don't reset version or nonce when a new watch is started. The version
	// and nonce from previous response are carried by the request unless the
	// stream is recreated.
	ver = t.versionMap[rType]
	nonce = t.nonceMap[rType]
	return target, rType, ver, nonce
}

type ackAction struct {
	rType   xdsresource.ResourceType
	version string // NACK if version is an empty string.
	nonce   string
	errMsg  string // Empty unless it's a NACK.
	// ACK/NACK are tagged with the stream it's for. When the stream is down,
	// all the ACK/NACK for this stream will be dropped, and the version/nonce
	// won't be updated.
	stream grpc.ClientStream
}

// processAckInfo pulls the fields needed by the ack request from a ackAction.
//
// If no active watch is found for this ack, it returns false for send.
func (t *Controller) processAckInfo(ack *ackAction, stream grpc.ClientStream) (target []string, rType xdsresource.ResourceType, version, nonce string, send bool) {
	if ack.stream != stream {
		// If ACK's stream isn't the current sending stream, this means the ACK
		// was pushed to queue before the old stream broke, and a new stream has
		// been started since. Return immediately here so we don't update the
		// nonce for the new stream.
		return nil, xdsresource.UnknownResource, "", "", false
	}
	rType = ack.rType

	t.mu.Lock()
	defer t.mu.Unlock()

	// Update the nonce no matter if we are going to send the ACK request on
	// wire. We may not send the request if the watch is canceled. But the nonce
	// needs to be updated so the next request will have the right nonce.
	nonce = ack.nonce
	t.nonceMap[rType] = nonce

	s, ok := t.watchMap[rType]
	if !ok || len(s) == 0 {
		// We don't send the request ack if there's no active watch (this can be
		// either the server sends responses before any request, or the watch is
		// canceled while the ackAction is in queue), because there's no resource
		// name. And if we send a request with empty resource name list, the
		// server may treat it as a wild card and send us everything.
		return nil, xdsresource.UnknownResource, "", "", false
	}
	send = true
	target = mapToSlice(s)

	version = ack.version
	if version == "" {
		// This is a nack, get the previous acked version.
		version = t.versionMap[rType]
		// version will still be an empty string if rType isn't
		// found in versionMap, this can happen if there wasn't any ack
		// before.
	} else {
		t.versionMap[rType] = version
	}
	return target, rType, version, nonce, send
}

// reportLoad starts an LRS stream to report load data to the management server.
// It blocks until the context is cancelled.
func (t *Controller) reportLoad(ctx context.Context, cc *grpc.ClientConn, opts controllerversion.LoadReportingOptions) {
	retries := 0
	for {
		if ctx.Err() != nil {
			return
		}

		if retries != 0 {
			timer := time.NewTimer(t.backoff(retries))
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return
			}
		}

		retries++
		stream, err := t.vClient.NewLoadStatsStream(ctx, cc)
		if err != nil {
			t.logger.Warningf("lrs: failed to create stream: %v", err)
			continue
		}
		t.logger.Infof("lrs: created LRS stream")

		if err := t.vClient.SendFirstLoadStatsRequest(stream); err != nil {
			t.logger.Warningf("lrs: failed to send first request: %v", err)
			continue
		}

		clusters, interval, err := t.vClient.HandleLoadStatsResponse(stream)
		if err != nil {
			t.logger.Warningf("%v", err)
			continue
		}

		retries = 0
		t.sendLoads(ctx, stream, opts.LoadStore, clusters, interval)
	}
}

func (t *Controller) sendLoads(ctx context.Context, stream grpc.ClientStream, store *load.Store, clusterNames []string, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
		case <-ctx.Done():
			return
		}
		if err := t.vClient.SendLoadStatsRequest(stream, store.Stats(clusterNames)); err != nil {
			t.logger.Warningf("%v", err)
			return
		}
	}
}
