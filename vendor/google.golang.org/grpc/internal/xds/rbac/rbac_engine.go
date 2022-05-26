/*
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

// Package rbac provides service-level and method-level access control for a
// service. See
// https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/rbac/v3/rbac.proto#role-based-access-control-rbac
// for documentation.
package rbac

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"

	v3rbacpb "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/internal/transport"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const logLevel = 2

var logger = grpclog.Component("rbac")

var getConnection = transport.GetConnection

// ChainEngine represents a chain of RBAC Engines, used to make authorization
// decisions on incoming RPCs.
type ChainEngine struct {
	chainedEngines []*engine
}

// NewChainEngine returns a chain of RBAC engines, used to make authorization
// decisions on incoming RPCs. Returns a non-nil error for invalid policies.
func NewChainEngine(policies []*v3rbacpb.RBAC) (*ChainEngine, error) {
	engines := make([]*engine, 0, len(policies))
	for _, policy := range policies {
		engine, err := newEngine(policy)
		if err != nil {
			return nil, err
		}
		engines = append(engines, engine)
	}
	return &ChainEngine{chainedEngines: engines}, nil
}

// IsAuthorized determines if an incoming RPC is authorized based on the chain of RBAC
// engines and their associated actions.
//
// Errors returned by this function are compatible with the status package.
func (cre *ChainEngine) IsAuthorized(ctx context.Context) error {
	// This conversion step (i.e. pulling things out of ctx) can be done once,
	// and then be used for the whole chain of RBAC Engines.
	rpcData, err := newRPCData(ctx)
	if err != nil {
		logger.Errorf("newRPCData: %v", err)
		return status.Errorf(codes.Internal, "gRPC RBAC: %v", err)
	}
	for _, engine := range cre.chainedEngines {
		matchingPolicyName, ok := engine.findMatchingPolicy(rpcData)
		if logger.V(logLevel) && ok {
			logger.Infof("incoming RPC matched to policy %v in engine with action %v", matchingPolicyName, engine.action)
		}

		switch {
		case engine.action == v3rbacpb.RBAC_ALLOW && !ok:
			return status.Errorf(codes.PermissionDenied, "incoming RPC did not match an allow policy")
		case engine.action == v3rbacpb.RBAC_DENY && ok:
			return status.Errorf(codes.PermissionDenied, "incoming RPC matched a deny policy %q", matchingPolicyName)
		}
		// Every policy in the engine list must be queried. Thus, iterate to the
		// next policy.
	}
	// If the incoming RPC gets through all of the engines successfully (i.e.
	// doesn't not match an allow or match a deny engine), the RPC is authorized
	// to proceed.
	return nil
}

// engine is used for matching incoming RPCs to policies.
type engine struct {
	policies map[string]*policyMatcher
	// action must be ALLOW or DENY.
	action v3rbacpb.RBAC_Action
}

// newEngine creates an RBAC Engine based on the contents of policy. Returns a
// non-nil error if the policy is invalid.
func newEngine(config *v3rbacpb.RBAC) (*engine, error) {
	a := config.GetAction()
	if a != v3rbacpb.RBAC_ALLOW && a != v3rbacpb.RBAC_DENY {
		return nil, fmt.Errorf("unsupported action %s", config.Action)
	}

	policies := make(map[string]*policyMatcher, len(config.GetPolicies()))
	for name, policy := range config.GetPolicies() {
		matcher, err := newPolicyMatcher(policy)
		if err != nil {
			return nil, err
		}
		policies[name] = matcher
	}
	return &engine{
		policies: policies,
		action:   a,
	}, nil
}

// findMatchingPolicy determines if an incoming RPC matches a policy. On a
// successful match, it returns the name of the matching policy and a true bool
// to specify that there was a matching policy found.  It returns false in
// the case of not finding a matching policy.
func (r *engine) findMatchingPolicy(rpcData *rpcData) (string, bool) {
	for policy, matcher := range r.policies {
		if matcher.match(rpcData) {
			return policy, true
		}
	}
	return "", false
}

// newRPCData takes an incoming context (should be a context representing state
// needed for server RPC Call with metadata, peer info (used for source ip/port
// and TLS information) and connection (used for destination ip/port) piped into
// it) and the method name of the Service being called server side and populates
// an rpcData struct ready to be passed to the RBAC Engine to find a matching
// policy.
func newRPCData(ctx context.Context) (*rpcData, error) {
	// The caller should populate all of these fields (i.e. for empty headers,
	// pipe an empty md into context).
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("missing metadata in incoming context")
	}
	// ":method can be hard-coded to POST if unavailable" - A41
	md[":method"] = []string{"POST"}
	// "If the transport exposes TE in Metadata, then RBAC must special-case the
	// header to treat it as not present." - A41
	delete(md, "TE")

	pi, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("missing peer info in incoming context")
	}

	// The methodName will be available in the passed in ctx from a unary or streaming
	// interceptor, as grpc.Server pipes in a transport stream which contains the methodName
	// into contexts available in both unary or streaming interceptors.
	mn, ok := grpc.Method(ctx)
	if !ok {
		return nil, errors.New("missing method in incoming context")
	}

	// The connection is needed in order to find the destination address and
	// port of the incoming RPC Call.
	conn := getConnection(ctx)
	if conn == nil {
		return nil, errors.New("missing connection in incoming context")
	}
	_, dPort, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return nil, fmt.Errorf("error parsing local address: %v", err)
	}
	dp, err := strconv.ParseUint(dPort, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error parsing local address: %v", err)
	}

	var authType string
	var peerCertificates []*x509.Certificate
	if pi.AuthInfo != nil {
		tlsInfo, ok := pi.AuthInfo.(credentials.TLSInfo)
		if ok {
			authType = pi.AuthInfo.AuthType()
			peerCertificates = tlsInfo.State.PeerCertificates
		}
	}

	return &rpcData{
		md:              md,
		peerInfo:        pi,
		fullMethod:      mn,
		destinationPort: uint32(dp),
		localAddr:       conn.LocalAddr(),
		authType:        authType,
		certs:           peerCertificates,
	}, nil
}

// rpcData wraps data pulled from an incoming RPC that the RBAC engine needs to
// find a matching policy.
type rpcData struct {
	// md is the HTTP Headers that are present in the incoming RPC.
	md metadata.MD
	// peerInfo is information about the downstream peer.
	peerInfo *peer.Peer
	// fullMethod is the method name being called on the upstream service.
	fullMethod string
	// destinationPort is the port that the RPC is being sent to on the
	// server.
	destinationPort uint32
	// localAddr is the address that the RPC is being sent to.
	localAddr net.Addr
	// authType is the type of authentication e.g. "tls".
	authType string
	// certs are the certificates presented by the peer during a TLS
	// handshake.
	certs []*x509.Certificate
}
