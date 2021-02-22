package network

import (
	"net"
	"net/url"

	"github.com/containers/podman/v3/pkg/bindings/internal/util"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *CreateOptions) Changed(fieldName string) bool {
	return util.Changed(o, fieldName)
}

// ToParams
func (o *CreateOptions) ToParams() (url.Values, error) {
	return util.ToParams(o)
}

// WithDisableDNS
func (o *CreateOptions) WithDisableDNS(value bool) *CreateOptions {
	v := &value
	o.DisableDNS = v
	return o
}

// GetDisableDNS
func (o *CreateOptions) GetDisableDNS() bool {
	var disableDNS bool
	if o.DisableDNS == nil {
		return disableDNS
	}
	return *o.DisableDNS
}

// WithDriver
func (o *CreateOptions) WithDriver(value string) *CreateOptions {
	v := &value
	o.Driver = v
	return o
}

// GetDriver
func (o *CreateOptions) GetDriver() string {
	var driver string
	if o.Driver == nil {
		return driver
	}
	return *o.Driver
}

// WithGateway
func (o *CreateOptions) WithGateway(value net.IP) *CreateOptions {
	v := &value
	o.Gateway = v
	return o
}

// GetGateway
func (o *CreateOptions) GetGateway() net.IP {
	var gateway net.IP
	if o.Gateway == nil {
		return gateway
	}
	return *o.Gateway
}

// WithInternal
func (o *CreateOptions) WithInternal(value bool) *CreateOptions {
	v := &value
	o.Internal = v
	return o
}

// GetInternal
func (o *CreateOptions) GetInternal() bool {
	var internal bool
	if o.Internal == nil {
		return internal
	}
	return *o.Internal
}

// WithLabels
func (o *CreateOptions) WithLabels(value map[string]string) *CreateOptions {
	v := value
	o.Labels = v
	return o
}

// GetLabels
func (o *CreateOptions) GetLabels() map[string]string {
	var labels map[string]string
	if o.Labels == nil {
		return labels
	}
	return o.Labels
}

// WithMacVLAN
func (o *CreateOptions) WithMacVLAN(value string) *CreateOptions {
	v := &value
	o.MacVLAN = v
	return o
}

// GetMacVLAN
func (o *CreateOptions) GetMacVLAN() string {
	var macVLAN string
	if o.MacVLAN == nil {
		return macVLAN
	}
	return *o.MacVLAN
}

// WithIPRange
func (o *CreateOptions) WithIPRange(value net.IPNet) *CreateOptions {
	v := &value
	o.IPRange = v
	return o
}

// GetIPRange
func (o *CreateOptions) GetIPRange() net.IPNet {
	var iPRange net.IPNet
	if o.IPRange == nil {
		return iPRange
	}
	return *o.IPRange
}

// WithSubnet
func (o *CreateOptions) WithSubnet(value net.IPNet) *CreateOptions {
	v := &value
	o.Subnet = v
	return o
}

// GetSubnet
func (o *CreateOptions) GetSubnet() net.IPNet {
	var subnet net.IPNet
	if o.Subnet == nil {
		return subnet
	}
	return *o.Subnet
}

// WithIPv6
func (o *CreateOptions) WithIPv6(value bool) *CreateOptions {
	v := &value
	o.IPv6 = v
	return o
}

// GetIPv6
func (o *CreateOptions) GetIPv6() bool {
	var iPv6 bool
	if o.IPv6 == nil {
		return iPv6
	}
	return *o.IPv6
}

// WithOptions
func (o *CreateOptions) WithOptions(value map[string]string) *CreateOptions {
	v := value
	o.Options = v
	return o
}

// GetOptions
func (o *CreateOptions) GetOptions() map[string]string {
	var options map[string]string
	if o.Options == nil {
		return options
	}
	return o.Options
}

// WithName
func (o *CreateOptions) WithName(value string) *CreateOptions {
	v := &value
	o.Name = v
	return o
}

// GetName
func (o *CreateOptions) GetName() string {
	var name string
	if o.Name == nil {
		return name
	}
	return *o.Name
}
