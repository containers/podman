package play

import (
	"net"
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *KubeOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *KubeOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithAuthfile
func (o *KubeOptions) WithAuthfile(value string) *KubeOptions {
	v := &value
	o.Authfile = v
	return o
}

// GetAuthfile
func (o *KubeOptions) GetAuthfile() string {
	var authfile string
	if o.Authfile == nil {
		return authfile
	}
	return *o.Authfile
}

// WithCertDir
func (o *KubeOptions) WithCertDir(value string) *KubeOptions {
	v := &value
	o.CertDir = v
	return o
}

// GetCertDir
func (o *KubeOptions) GetCertDir() string {
	var certDir string
	if o.CertDir == nil {
		return certDir
	}
	return *o.CertDir
}

// WithUsername
func (o *KubeOptions) WithUsername(value string) *KubeOptions {
	v := &value
	o.Username = v
	return o
}

// GetUsername
func (o *KubeOptions) GetUsername() string {
	var username string
	if o.Username == nil {
		return username
	}
	return *o.Username
}

// WithPassword
func (o *KubeOptions) WithPassword(value string) *KubeOptions {
	v := &value
	o.Password = v
	return o
}

// GetPassword
func (o *KubeOptions) GetPassword() string {
	var password string
	if o.Password == nil {
		return password
	}
	return *o.Password
}

// WithNetwork
func (o *KubeOptions) WithNetwork(value string) *KubeOptions {
	v := &value
	o.Network = v
	return o
}

// GetNetwork
func (o *KubeOptions) GetNetwork() string {
	var network string
	if o.Network == nil {
		return network
	}
	return *o.Network
}

// WithQuiet
func (o *KubeOptions) WithQuiet(value bool) *KubeOptions {
	v := &value
	o.Quiet = v
	return o
}

// GetQuiet
func (o *KubeOptions) GetQuiet() bool {
	var quiet bool
	if o.Quiet == nil {
		return quiet
	}
	return *o.Quiet
}

// WithSignaturePolicy
func (o *KubeOptions) WithSignaturePolicy(value string) *KubeOptions {
	v := &value
	o.SignaturePolicy = v
	return o
}

// GetSignaturePolicy
func (o *KubeOptions) GetSignaturePolicy() string {
	var signaturePolicy string
	if o.SignaturePolicy == nil {
		return signaturePolicy
	}
	return *o.SignaturePolicy
}

// WithSkipTLSVerify
func (o *KubeOptions) WithSkipTLSVerify(value bool) *KubeOptions {
	v := &value
	o.SkipTLSVerify = v
	return o
}

// GetSkipTLSVerify
func (o *KubeOptions) GetSkipTLSVerify() bool {
	var skipTLSVerify bool
	if o.SkipTLSVerify == nil {
		return skipTLSVerify
	}
	return *o.SkipTLSVerify
}

// WithSeccompProfileRoot
func (o *KubeOptions) WithSeccompProfileRoot(value string) *KubeOptions {
	v := &value
	o.SeccompProfileRoot = v
	return o
}

// GetSeccompProfileRoot
func (o *KubeOptions) GetSeccompProfileRoot() string {
	var seccompProfileRoot string
	if o.SeccompProfileRoot == nil {
		return seccompProfileRoot
	}
	return *o.SeccompProfileRoot
}

// WithStaticIPs
func (o *KubeOptions) WithStaticIPs(value []net.IP) *KubeOptions {
	v := &value
	o.StaticIPs = v
	return o
}

// GetStaticIPs
func (o *KubeOptions) GetStaticIPs() []net.IP {
	var staticIPs []net.IP
	if o.StaticIPs == nil {
		return staticIPs
	}
	return *o.StaticIPs
}

// WithStaticMACs
func (o *KubeOptions) WithStaticMACs(value []net.HardwareAddr) *KubeOptions {
	v := &value
	o.StaticMACs = v
	return o
}

// GetStaticMACs
func (o *KubeOptions) GetStaticMACs() []net.HardwareAddr {
	var staticMACs []net.HardwareAddr
	if o.StaticMACs == nil {
		return staticMACs
	}
	return *o.StaticMACs
}

// WithConfigMaps
func (o *KubeOptions) WithConfigMaps(value []string) *KubeOptions {
	v := &value
	o.ConfigMaps = v
	return o
}

// GetConfigMaps
func (o *KubeOptions) GetConfigMaps() []string {
	var configMaps []string
	if o.ConfigMaps == nil {
		return configMaps
	}
	return *o.ConfigMaps
}

// WithLogDriver
func (o *KubeOptions) WithLogDriver(value string) *KubeOptions {
	v := &value
	o.LogDriver = v
	return o
}

// GetLogDriver
func (o *KubeOptions) GetLogDriver() string {
	var logDriver string
	if o.LogDriver == nil {
		return logDriver
	}
	return *o.LogDriver
}

// WithStart
func (o *KubeOptions) WithStart(value bool) *KubeOptions {
	v := &value
	o.Start = v
	return o
}

// GetStart
func (o *KubeOptions) GetStart() bool {
	var start bool
	if o.Start == nil {
		return start
	}
	return *o.Start
}
