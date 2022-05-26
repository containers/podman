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

// Package bootstrap provides the functionality to initialize certain aspects
// of an xDS client by reading a bootstrap file.
package bootstrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/google"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/tls/certprovider"
	"google.golang.org/grpc/internal"
	"google.golang.org/grpc/internal/envconfig"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/xds/internal/xdsclient/xdsresource/version"

	v2corepb "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	v3corepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
)

const (
	// The "server_features" field in the bootstrap file contains a list of
	// features supported by the server. A value of "xds_v3" indicates that the
	// server supports the v3 version of the xDS transport protocol.
	serverFeaturesV3 = "xds_v3"

	// Type name for Google default credentials.
	credsGoogleDefault              = "google_default"
	credsInsecure                   = "insecure"
	gRPCUserAgentName               = "gRPC Go"
	clientFeatureNoOverprovisioning = "envoy.lb.does_not_support_overprovisioning"
)

var gRPCVersion = fmt.Sprintf("%s %s", gRPCUserAgentName, grpc.Version)

// For overriding in unit tests.
var bootstrapFileReadFunc = ioutil.ReadFile

// ServerConfig contains the configuration to connect to a server, including
// URI, creds, and transport API version (e.g. v2 or v3).
type ServerConfig struct {
	// ServerURI is the management server to connect to.
	//
	// The bootstrap file contains an ordered list of xDS servers to contact for
	// this authority. The first one is picked.
	ServerURI string
	// Creds contains the credentials to be used while talking to the xDS
	// server, as a grpc.DialOption.
	Creds grpc.DialOption
	// CredsType is the type of the creds. It will be used to dedup servers.
	CredsType string
	// TransportAPI indicates the API version of xDS transport protocol to use.
	// This describes the xDS gRPC endpoint and version of
	// DiscoveryRequest/Response used on the wire.
	TransportAPI version.TransportAPI
	// NodeProto contains the Node proto to be used in xDS requests. The actual
	// type depends on the transport protocol version used.
	//
	// Note that it's specified in the bootstrap globally for all the servers,
	// but we keep it in each server config so that its type (e.g. *v2pb.Node or
	// *v3pb.Node) is consistent with the transport API version.
	NodeProto proto.Message
}

// String returns the string representation of the ServerConfig.
//
// This string representation will be used as map keys in federation
// (`map[ServerConfig]authority`), so that the xDS ClientConn and stream will be
// shared by authorities with different names but the same server config.
//
// It covers (almost) all the fields so the string can represent the config
// content. It doesn't cover NodeProto because NodeProto isn't used by
// federation.
func (sc *ServerConfig) String() string {
	var ver string
	switch sc.TransportAPI {
	case version.TransportV3:
		ver = "xDSv3"
	case version.TransportV2:
		ver = "xDSv2"
	}
	return strings.Join([]string{sc.ServerURI, sc.CredsType, ver}, "-")
}

// UnmarshalJSON takes the json data (a list of servers) and unmarshals the
// first one in the list.
func (sc *ServerConfig) UnmarshalJSON(data []byte) error {
	var servers []*xdsServer
	if err := json.Unmarshal(data, &servers); err != nil {
		return fmt.Errorf("xds: json.Unmarshal(data) for field xds_servers failed during bootstrap: %v", err)
	}
	if len(servers) < 1 {
		return fmt.Errorf("xds: bootstrap file parsing failed during bootstrap: file doesn't contain any management server to connect to")
	}
	xs := servers[0]
	sc.ServerURI = xs.ServerURI
	for _, cc := range xs.ChannelCreds {
		// We stop at the first credential type that we support.
		sc.CredsType = cc.Type
		if cc.Type == credsGoogleDefault {
			sc.Creds = grpc.WithCredentialsBundle(google.NewDefaultCredentials())
			break
		} else if cc.Type == credsInsecure {
			sc.Creds = grpc.WithTransportCredentials(insecure.NewCredentials())
			break
		}
	}
	for _, f := range xs.ServerFeatures {
		if f == serverFeaturesV3 {
			sc.TransportAPI = version.TransportV3
		}
	}
	return nil
}

// Authority contains configuration for an Authority for an xDS control plane
// server. See the Authorities field in the Config struct for how it's used.
type Authority struct {
	// ClientListenerResourceNameTemplate is template for the name of the
	// Listener resource to subscribe to for a gRPC client channel.  Used only
	// when the channel is created using an "xds:" URI with this authority name.
	//
	// The token "%s", if present in this string, will be replaced
	// with %-encoded service authority (i.e., the path part of the target
	// URI used to create the gRPC channel).
	//
	// Must start with "xdstp://<authority_name>/".  If it does not,
	// that is considered a bootstrap file parsing error.
	//
	// If not present in the bootstrap file, defaults to
	// "xdstp://<authority_name>/envoy.config.listener.v3.Listener/%s".
	ClientListenerResourceNameTemplate string
	// XDSServer contains the management server and config to connect to for
	// this authority.
	XDSServer *ServerConfig
}

// UnmarshalJSON implement json unmarshaller.
func (a *Authority) UnmarshalJSON(data []byte) error {
	var jsonData map[string]json.RawMessage
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("xds: failed to parse authority: %v", err)
	}

	for k, v := range jsonData {
		switch k {
		case "xds_servers":
			if err := json.Unmarshal(v, &a.XDSServer); err != nil {
				return fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), k, err)
			}
		case "client_listener_resource_name_template":
			if err := json.Unmarshal(v, &a.ClientListenerResourceNameTemplate); err != nil {
				return fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), k, err)
			}
		}
	}
	return nil
}

// Config provides the xDS client with several key bits of information that it
// requires in its interaction with the management server. The Config is
// initialized from the bootstrap file.
type Config struct {
	// XDSServer is the management server to connect to.
	//
	// The bootstrap file contains a list of servers (with name+creds), but we
	// pick the first one.
	XDSServer *ServerConfig
	// CertProviderConfigs contains a mapping from certificate provider plugin
	// instance names to parsed buildable configs.
	CertProviderConfigs map[string]*certprovider.BuildableConfig
	// ServerListenerResourceNameTemplate is a template for the name of the
	// Listener resource to subscribe to for a gRPC server.
	//
	// If starts with "xdstp:", will be interpreted as a new-style name,
	// in which case the authority of the URI will be used to select the
	// relevant configuration in the "authorities" map.
	//
	// The token "%s", if present in this string, will be replaced with the IP
	// and port on which the server is listening.  (e.g., "0.0.0.0:8080",
	// "[::]:8080"). For example, a value of "example/resource/%s" could become
	// "example/resource/0.0.0.0:8080". If the template starts with "xdstp:",
	// the replaced string will be %-encoded.
	//
	// There is no default; if unset, xDS-based server creation fails.
	ServerListenerResourceNameTemplate string
	// A template for the name of the Listener resource to subscribe to
	// for a gRPC client channel.  Used only when the channel is created
	// with an "xds:" URI with no authority.
	//
	// If starts with "xdstp:", will be interpreted as a new-style name,
	// in which case the authority of the URI will be used to select the
	// relevant configuration in the "authorities" map.
	//
	// The token "%s", if present in this string, will be replaced with
	// the service authority (i.e., the path part of the target URI
	// used to create the gRPC channel).  If the template starts with
	// "xdstp:", the replaced string will be %-encoded.
	//
	// Defaults to "%s".
	ClientDefaultListenerResourceNameTemplate string

	// Authorities is a map of authority name to corresponding configuration.
	//
	// This is used in the following cases:
	// - A gRPC client channel is created using an "xds:" URI that includes
	//   an authority.
	// - A gRPC client channel is created using an "xds:" URI with no
	//   authority, but the "client_default_listener_resource_name_template"
	//   field above turns it into an "xdstp:" URI.
	// - A gRPC server is created and the
	//   "server_listener_resource_name_template" field is an "xdstp:" URI.
	//
	// In any of those cases, it is an error if the specified authority is
	// not present in this map.
	Authorities map[string]*Authority
}

type channelCreds struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

type xdsServer struct {
	ServerURI      string         `json:"server_uri"`
	ChannelCreds   []channelCreds `json:"channel_creds"`
	ServerFeatures []string       `json:"server_features"`
}

func bootstrapConfigFromEnvVariable() ([]byte, error) {
	fName := envconfig.XDSBootstrapFileName
	fContent := envconfig.XDSBootstrapFileContent

	// Bootstrap file name has higher priority than bootstrap content.
	if fName != "" {
		// If file name is set
		// - If file not found (or other errors), fail
		// - Otherwise, use the content.
		//
		// Note that even if the content is invalid, we don't failover to the
		// file content env variable.
		logger.Debugf("xds: using bootstrap file with name %q", fName)
		return bootstrapFileReadFunc(fName)
	}

	if fContent != "" {
		return []byte(fContent), nil
	}

	return nil, fmt.Errorf("none of the bootstrap environment variables (%q or %q) defined",
		envconfig.XDSBootstrapFileNameEnv, envconfig.XDSBootstrapFileContentEnv)
}

// NewConfig returns a new instance of Config initialized by reading the
// bootstrap file found at ${GRPC_XDS_BOOTSTRAP}.
//
// Currently, we support exactly one type of credential, which is
// "google_default", where we use the host's default certs for transport
// credentials and a Google oauth token for call credentials.
//
// This function tries to process as much of the bootstrap file as possible (in
// the presence of the errors) and may return a Config object with certain
// fields left unspecified, in which case the caller should use some sane
// defaults.
func NewConfig() (*Config, error) {
	// Examples of the bootstrap json can be found in the generator tests
	// https://github.com/GoogleCloudPlatform/traffic-director-grpc-bootstrap/blob/master/main_test.go.
	data, err := bootstrapConfigFromEnvVariable()
	if err != nil {
		return nil, fmt.Errorf("xds: Failed to read bootstrap config: %v", err)
	}
	logger.Debugf("Bootstrap content: %s", data)
	return NewConfigFromContents(data)
}

// NewConfigFromContents returns a new Config using the specified bootstrap
// file contents instead of reading the environment variable.  This is only
// suitable for testing purposes.
func NewConfigFromContents(data []byte) (*Config, error) {
	config := &Config{}

	var jsonData map[string]json.RawMessage
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return nil, fmt.Errorf("xds: Failed to parse bootstrap config: %v", err)
	}

	var node *v3corepb.Node
	m := jsonpb.Unmarshaler{AllowUnknownFields: true}
	for k, v := range jsonData {
		switch k {
		case "node":
			// We unconditionally convert the JSON into a v3.Node proto. The v3
			// proto does not contain the deprecated field "build_version" from
			// the v2 proto. We do not expect the bootstrap file to contain the
			// "build_version" field. In any case, the unmarshal will succeed
			// because we have set the `AllowUnknownFields` option on the
			// unmarshaler.
			node = &v3corepb.Node{}
			if err := m.Unmarshal(bytes.NewReader(v), node); err != nil {
				return nil, fmt.Errorf("xds: jsonpb.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), k, err)
			}
		case "xds_servers":
			if err := json.Unmarshal(v, &config.XDSServer); err != nil {
				return nil, fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), k, err)
			}
		case "certificate_providers":
			var providerInstances map[string]json.RawMessage
			if err := json.Unmarshal(v, &providerInstances); err != nil {
				return nil, fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), k, err)
			}
			configs := make(map[string]*certprovider.BuildableConfig)
			getBuilder := internal.GetCertificateProviderBuilder.(func(string) certprovider.Builder)
			for instance, data := range providerInstances {
				var nameAndConfig struct {
					PluginName string          `json:"plugin_name"`
					Config     json.RawMessage `json:"config"`
				}
				if err := json.Unmarshal(data, &nameAndConfig); err != nil {
					return nil, fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), instance, err)
				}

				name := nameAndConfig.PluginName
				parser := getBuilder(nameAndConfig.PluginName)
				if parser == nil {
					// We ignore plugins that we do not know about.
					continue
				}
				bc, err := parser.ParseConfig(nameAndConfig.Config)
				if err != nil {
					return nil, fmt.Errorf("xds: Config parsing for plugin %q failed: %v", name, err)
				}
				configs[instance] = bc
			}
			config.CertProviderConfigs = configs
		case "server_listener_resource_name_template":
			if err := json.Unmarshal(v, &config.ServerListenerResourceNameTemplate); err != nil {
				return nil, fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), k, err)
			}
		case "client_default_listener_resource_name_template":
			if !envconfig.XDSFederation {
				logger.Warningf("xds: bootstrap field %v is not support when Federation is disabled", k)
				continue
			}
			if err := json.Unmarshal(v, &config.ClientDefaultListenerResourceNameTemplate); err != nil {
				return nil, fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), k, err)
			}
		case "authorities":
			if !envconfig.XDSFederation {
				logger.Warningf("xds: bootstrap field %v is not support when Federation is disabled", k)
				continue
			}
			if err := json.Unmarshal(v, &config.Authorities); err != nil {
				return nil, fmt.Errorf("xds: json.Unmarshal(%v) for field %q failed during bootstrap: %v", string(v), k, err)
			}
		default:
			logger.Warningf("Bootstrap content has unknown field: %s", k)
		}
		// Do not fail the xDS bootstrap when an unknown field is seen. This can
		// happen when an older version client reads a newer version bootstrap
		// file with new fields.
	}

	if config.ClientDefaultListenerResourceNameTemplate == "" {
		// Default value of the default client listener name template is "%s".
		config.ClientDefaultListenerResourceNameTemplate = "%s"
	}
	if config.XDSServer == nil {
		return nil, fmt.Errorf("xds: Required field %q not found in bootstrap %s", "xds_servers", jsonData["xds_servers"])
	}
	if config.XDSServer.ServerURI == "" {
		return nil, fmt.Errorf("xds: Required field %q not found in bootstrap %s", "xds_servers.server_uri", jsonData["xds_servers"])
	}
	if config.XDSServer.Creds == nil {
		return nil, fmt.Errorf("xds: Required field %q doesn't contain valid value in bootstrap %s", "xds_servers.channel_creds", jsonData["xds_servers"])
	}
	// Post-process the authorities' client listener resource template field:
	// - if set, it must start with "xdstp://<authority_name>/"
	// - if not set, it defaults to "xdstp://<authority_name>/envoy.config.listener.v3.Listener/%s"
	for name, authority := range config.Authorities {
		prefix := fmt.Sprintf("xdstp://%s", name)
		if authority.ClientListenerResourceNameTemplate == "" {
			authority.ClientListenerResourceNameTemplate = prefix + "/envoy.config.listener.v3.Listener/%s"
			continue
		}
		if !strings.HasPrefix(authority.ClientListenerResourceNameTemplate, prefix) {
			return nil, fmt.Errorf("xds: field ClientListenerResourceNameTemplate %q of authority %q doesn't start with prefix %q", authority.ClientListenerResourceNameTemplate, name, prefix)
		}
	}

	if err := config.updateNodeProto(node); err != nil {
		return nil, err
	}
	logger.Infof("Bootstrap config for creating xds-client: %v", pretty.ToJSON(config))
	return config, nil
}

// updateNodeProto updates the node proto read from the bootstrap file.
//
// The input node is a v3.Node protobuf message corresponding to the JSON
// contents found in the bootstrap file. This method performs some post
// processing on it:
// 1. If the node is nil, we create an empty one here. That way, callers of this
// function can always expect that the NodeProto field is non-nil.
// 2. Some additional fields which are not expected to be set in the bootstrap
// file are populated here.
// 3. For each server config (both top level and in each authority), we set its
// node field to the v3.Node, or a v2.Node with the same content, depending on
// the server's transprot API version.
func (c *Config) updateNodeProto(node *v3corepb.Node) error {
	v3 := node
	if v3 == nil {
		v3 = &v3corepb.Node{}
	}
	v3.UserAgentName = gRPCUserAgentName
	v3.UserAgentVersionType = &v3corepb.Node_UserAgentVersion{UserAgentVersion: grpc.Version}
	v3.ClientFeatures = append(v3.ClientFeatures, clientFeatureNoOverprovisioning)

	v2 := &v2corepb.Node{}
	v3bytes, err := proto.Marshal(v3)
	if err != nil {
		return fmt.Errorf("xds: proto.Marshal(%v): %v", v3, err)
	}
	if err := proto.Unmarshal(v3bytes, v2); err != nil {
		return fmt.Errorf("xds: proto.Unmarshal(%v): %v", v3bytes, err)
	}
	// BuildVersion is deprecated, and is replaced by user_agent_name and
	// user_agent_version. But the management servers are still using the old
	// field, so we will keep both set.
	v2.BuildVersion = gRPCVersion
	v2.UserAgentVersionType = &v2corepb.Node_UserAgentVersion{UserAgentVersion: grpc.Version}

	switch c.XDSServer.TransportAPI {
	case version.TransportV2:
		c.XDSServer.NodeProto = v2
	case version.TransportV3:
		c.XDSServer.NodeProto = v3
	}

	for _, a := range c.Authorities {
		if a.XDSServer == nil {
			continue
		}
		switch a.XDSServer.TransportAPI {
		case version.TransportV2:
			a.XDSServer.NodeProto = v2
		case version.TransportV3:
			a.XDSServer.NodeProto = v3
		}
	}

	return nil
}
