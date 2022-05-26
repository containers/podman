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

package xds

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/internal"
	"google.golang.org/grpc/internal/buffer"
	"google.golang.org/grpc/internal/envconfig"
	internalgrpclog "google.golang.org/grpc/internal/grpclog"
	"google.golang.org/grpc/internal/grpcsync"
	iresolver "google.golang.org/grpc/internal/resolver"
	"google.golang.org/grpc/internal/transport"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/xds/internal/server"
	"google.golang.org/grpc/xds/internal/xdsclient"
	"google.golang.org/grpc/xds/internal/xdsclient/bootstrap"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource"
)

const serverPrefix = "[xds-server %p] "

var (
	// These new functions will be overridden in unit tests.
	newXDSClient = func() (xdsclient.XDSClient, error) {
		return xdsclient.New()
	}
	newGRPCServer = func(opts ...grpc.ServerOption) grpcServer {
		return grpc.NewServer(opts...)
	}

	grpcGetServerCreds    = internal.GetServerCredentials.(func(*grpc.Server) credentials.TransportCredentials)
	drainServerTransports = internal.DrainServerTransports.(func(*grpc.Server, string))
	logger                = grpclog.Component("xds")
)

func prefixLogger(p *GRPCServer) *internalgrpclog.PrefixLogger {
	return internalgrpclog.NewPrefixLogger(logger, fmt.Sprintf(serverPrefix, p))
}

// grpcServer contains methods from grpc.Server which are used by the
// GRPCServer type here. This is useful for overriding in unit tests.
type grpcServer interface {
	RegisterService(*grpc.ServiceDesc, interface{})
	Serve(net.Listener) error
	Stop()
	GracefulStop()
	GetServiceInfo() map[string]grpc.ServiceInfo
}

// GRPCServer wraps a gRPC server and provides server-side xDS functionality, by
// communication with a management server using xDS APIs. It implements the
// grpc.ServiceRegistrar interface and can be passed to service registration
// functions in IDL generated code.
type GRPCServer struct {
	gs            grpcServer
	quit          *grpcsync.Event
	logger        *internalgrpclog.PrefixLogger
	xdsCredsInUse bool
	opts          *serverOptions

	// clientMu is used only in initXDSClient(), which is called at the
	// beginning of Serve(), where we have to decide if we have to create a
	// client or use an existing one.
	clientMu sync.Mutex
	xdsC     xdsclient.XDSClient
}

// NewGRPCServer creates an xDS-enabled gRPC server using the passed in opts.
// The underlying gRPC server has no service registered and has not started to
// accept requests yet.
func NewGRPCServer(opts ...grpc.ServerOption) *GRPCServer {
	newOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(xdsUnaryInterceptor),
		grpc.ChainStreamInterceptor(xdsStreamInterceptor),
	}
	newOpts = append(newOpts, opts...)
	s := &GRPCServer{
		gs:   newGRPCServer(newOpts...),
		quit: grpcsync.NewEvent(),
		opts: handleServerOptions(opts),
	}
	s.logger = prefixLogger(s)
	s.logger.Infof("Created xds.GRPCServer")

	// We type assert our underlying gRPC server to the real grpc.Server here
	// before trying to retrieve the configured credentials. This approach
	// avoids performing the same type assertion in the grpc package which
	// provides the implementation for internal.GetServerCredentials, and allows
	// us to use a fake gRPC server in tests.
	if gs, ok := s.gs.(*grpc.Server); ok {
		creds := grpcGetServerCreds(gs)
		if xc, ok := creds.(interface{ UsesXDS() bool }); ok && xc.UsesXDS() {
			s.xdsCredsInUse = true
		}
	}

	s.logger.Infof("xDS credentials in use: %v", s.xdsCredsInUse)
	return s
}

// handleServerOptions iterates through the list of server options passed in by
// the user, and handles the xDS server specific options.
func handleServerOptions(opts []grpc.ServerOption) *serverOptions {
	so := &serverOptions{}
	for _, opt := range opts {
		if o, ok := opt.(*serverOption); ok {
			o.apply(so)
		}
	}
	return so
}

// RegisterService registers a service and its implementation to the underlying
// gRPC server. It is called from the IDL generated code. This must be called
// before invoking Serve.
func (s *GRPCServer) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	s.gs.RegisterService(sd, ss)
}

// GetServiceInfo returns a map from service names to ServiceInfo.
// Service names include the package names, in the form of <package>.<service>.
func (s *GRPCServer) GetServiceInfo() map[string]grpc.ServiceInfo {
	return s.gs.GetServiceInfo()
}

// initXDSClient creates a new xdsClient if there is no existing one available.
func (s *GRPCServer) initXDSClient() error {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()

	if s.xdsC != nil {
		return nil
	}

	newXDSClient := newXDSClient
	if s.opts.bootstrapContents != nil {
		newXDSClient = func() (xdsclient.XDSClient, error) {
			return xdsclient.NewClientWithBootstrapContents(s.opts.bootstrapContents)
		}
	}
	client, err := newXDSClient()
	if err != nil {
		return fmt.Errorf("xds: failed to create xds-client: %v", err)
	}
	s.xdsC = client
	s.logger.Infof("Created an xdsClient")
	return nil
}

// Serve gets the underlying gRPC server to accept incoming connections on the
// listener lis, which is expected to be listening on a TCP port.
//
// A connection to the management server, to receive xDS configuration, is
// initiated here.
//
// Serve will return a non-nil error unless Stop or GracefulStop is called.
func (s *GRPCServer) Serve(lis net.Listener) error {
	s.logger.Infof("Serve() passed a net.Listener on %s", lis.Addr().String())
	if _, ok := lis.Addr().(*net.TCPAddr); !ok {
		return fmt.Errorf("xds: GRPCServer expects listener to return a net.TCPAddr. Got %T", lis.Addr())
	}

	// If this is the first time Serve() is being called, we need to initialize
	// our xdsClient. If not, we can use the existing one.
	if err := s.initXDSClient(); err != nil {
		return err
	}
	cfg := s.xdsC.BootstrapConfig()
	if cfg == nil {
		return errors.New("bootstrap configuration is empty")
	}

	// If xds credentials were specified by the user, but bootstrap configs do
	// not contain any certificate provider configuration, it is better to fail
	// right now rather than failing when attempting to create certificate
	// providers after receiving an LDS response with security configuration.
	if s.xdsCredsInUse {
		if len(cfg.CertProviderConfigs) == 0 {
			return errors.New("xds: certificate_providers config missing in bootstrap file")
		}
	}

	// The server listener resource name template from the bootstrap
	// configuration contains a template for the name of the Listener resource
	// to subscribe to for a gRPC server. If the token `%s` is present in the
	// string, it will be replaced with the server's listening "IP:port" (e.g.,
	// "0.0.0.0:8080", "[::]:8080"). The absence of a template will be treated
	// as an error since we do not have any default value for this.
	if cfg.ServerListenerResourceNameTemplate == "" {
		return errors.New("missing server_listener_resource_name_template in the bootstrap configuration")
	}
	name := bootstrap.PopulateResourceTemplate(cfg.ServerListenerResourceNameTemplate, lis.Addr().String())

	modeUpdateCh := buffer.NewUnbounded()
	go func() {
		s.handleServingModeChanges(modeUpdateCh)
	}()

	// Create a listenerWrapper which handles all functionality required by
	// this particular instance of Serve().
	lw, goodUpdateCh := server.NewListenerWrapper(server.ListenerWrapperParams{
		Listener:             lis,
		ListenerResourceName: name,
		XDSCredsInUse:        s.xdsCredsInUse,
		XDSClient:            s.xdsC,
		ModeCallback: func(addr net.Addr, mode connectivity.ServingMode, err error) {
			modeUpdateCh.Put(&modeChangeArgs{
				addr: addr,
				mode: mode,
				err:  err,
			})
		},
		DrainCallback: func(addr net.Addr) {
			if gs, ok := s.gs.(*grpc.Server); ok {
				drainServerTransports(gs, addr.String())
			}
		},
	})

	// Block until a good LDS response is received or the server is stopped.
	select {
	case <-s.quit.Done():
		// Since the listener has not yet been handed over to gs.Serve(), we
		// need to explicitly close the listener. Cancellation of the xDS watch
		// is handled by the listenerWrapper.
		lw.Close()
		return nil
	case <-goodUpdateCh:
	}
	return s.gs.Serve(lw)
}

// modeChangeArgs wraps argument required for invoking mode change callback.
type modeChangeArgs struct {
	addr net.Addr
	mode connectivity.ServingMode
	err  error
}

// handleServingModeChanges runs as a separate goroutine, spawned from Serve().
// It reads a channel on to which mode change arguments are pushed, and in turn
// invokes the user registered callback. It also calls an internal method on the
// underlying grpc.Server to gracefully close existing connections, if the
// listener moved to a "not-serving" mode.
func (s *GRPCServer) handleServingModeChanges(updateCh *buffer.Unbounded) {
	for {
		select {
		case <-s.quit.Done():
			return
		case u := <-updateCh.Get():
			updateCh.Load()
			args := u.(*modeChangeArgs)
			if args.mode == connectivity.ServingModeNotServing {
				// We type assert our underlying gRPC server to the real
				// grpc.Server here before trying to initiate the drain
				// operation. This approach avoids performing the same type
				// assertion in the grpc package which provides the
				// implementation for internal.GetServerCredentials, and allows
				// us to use a fake gRPC server in tests.
				if gs, ok := s.gs.(*grpc.Server); ok {
					drainServerTransports(gs, args.addr.String())
				}
			}
			if s.opts.modeCallback != nil {
				s.opts.modeCallback(args.addr, ServingModeChangeArgs{
					Mode: args.mode,
					Err:  args.err,
				})
			}
		}
	}
}

// Stop stops the underlying gRPC server. It immediately closes all open
// connections. It cancels all active RPCs on the server side and the
// corresponding pending RPCs on the client side will get notified by connection
// errors.
func (s *GRPCServer) Stop() {
	s.quit.Fire()
	s.gs.Stop()
	if s.xdsC != nil {
		s.xdsC.Close()
	}
}

// GracefulStop stops the underlying gRPC server gracefully. It stops the server
// from accepting new connections and RPCs and blocks until all the pending RPCs
// are finished.
func (s *GRPCServer) GracefulStop() {
	s.quit.Fire()
	s.gs.GracefulStop()
	if s.xdsC != nil {
		s.xdsC.Close()
	}
}

// routeAndProcess routes the incoming RPC to a configured route in the route
// table and also processes the RPC by running the incoming RPC through any HTTP
// Filters configured.
func routeAndProcess(ctx context.Context) error {
	conn := transport.GetConnection(ctx)
	cw, ok := conn.(interface {
		VirtualHosts() []xdsresource.VirtualHostWithInterceptors
	})
	if !ok {
		return errors.New("missing virtual hosts in incoming context")
	}
	mn, ok := grpc.Method(ctx)
	if !ok {
		return errors.New("missing method name in incoming context")
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("missing metadata in incoming context")
	}
	// A41 added logic to the core grpc implementation to guarantee that once
	// the RPC gets to this point, there will be a single, unambiguous authority
	// present in the header map.
	authority := md.Get(":authority")
	vh := xdsresource.FindBestMatchingVirtualHostServer(authority[0], cw.VirtualHosts())
	if vh == nil {
		return status.Error(codes.Unavailable, "the incoming RPC did not match a configured Virtual Host")
	}

	var rwi *xdsresource.RouteWithInterceptors
	rpcInfo := iresolver.RPCInfo{
		Context: ctx,
		Method:  mn,
	}
	for _, r := range vh.Routes {
		if r.M.Match(rpcInfo) {
			// "NonForwardingAction is expected for all Routes used on server-side; a route with an inappropriate action causes
			// RPCs matching that route to fail with UNAVAILABLE." - A36
			if r.ActionType != xdsresource.RouteActionNonForwardingAction {
				return status.Error(codes.Unavailable, "the incoming RPC matched to a route that was not of action type non forwarding")
			}
			rwi = &r
			break
		}
	}
	if rwi == nil {
		return status.Error(codes.Unavailable, "the incoming RPC did not match a configured Route")
	}
	for _, interceptor := range rwi.Interceptors {
		if err := interceptor.AllowRPC(ctx); err != nil {
			return status.Errorf(codes.PermissionDenied, "Incoming RPC is not allowed: %v", err)
		}
	}
	return nil
}

// xdsUnaryInterceptor is the unary interceptor added to the gRPC server to
// perform any xDS specific functionality on unary RPCs.
func xdsUnaryInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	if envconfig.XDSRBAC {
		if err := routeAndProcess(ctx); err != nil {
			return nil, err
		}
	}
	return handler(ctx, req)
}

// xdsStreamInterceptor is the stream interceptor added to the gRPC server to
// perform any xDS specific functionality on streaming RPCs.
func xdsStreamInterceptor(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if envconfig.XDSRBAC {
		if err := routeAndProcess(ss.Context()); err != nil {
			return err
		}
	}
	return handler(srv, ss)
}
