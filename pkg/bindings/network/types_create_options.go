package network

import (
	"net"
	"net/url"
	"reflect"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.

Created 2020-12-18 15:58:33.37307678 -0600 CST m=+0.000176739
*/

// Changed
func (o *CreateOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *CreateOptions) ToParams() (url.Values, error) {
	params := url.Values{}
	if o == nil {
		return params, nil
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	s := reflect.ValueOf(o)
	if reflect.Ptr == s.Kind() {
		s = s.Elem()
	}
	sType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		fieldName := sType.Field(i).Name
		if !o.Changed(fieldName) {
			continue
		}
		f := s.Field(i)
		if reflect.Ptr == f.Kind() {
			f = f.Elem()
		}
		switch f.Kind() {
		case reflect.Bool:
			params.Set(fieldName, strconv.FormatBool(f.Bool()))
		case reflect.String:
			params.Set(fieldName, f.String())
		case reflect.Int, reflect.Int64:
			// f.Int() is always an int64
			params.Set(fieldName, strconv.FormatInt(f.Int(), 10))
		case reflect.Uint, reflect.Uint64:
			// f.Uint() is always an uint64
			params.Set(fieldName, strconv.FormatUint(f.Uint(), 10))
		case reflect.Slice:
			typ := reflect.TypeOf(f.Interface()).Elem()
			switch typ.Kind() {
			case reflect.String:
				sl := f.Slice(0, f.Len())
				s, ok := sl.Interface().([]string)
				if !ok {
					return nil, errors.New("failed to convert to string slice")
				}
				for _, val := range s {
					params.Add(fieldName, val)
				}
			default:
				return nil, errors.Errorf("unknown slice type %s", f.Kind().String())
			}
		case reflect.Map:
			lowerCaseKeys := make(map[string][]string)
			iter := f.MapRange()
			for iter.Next() {
				lowerCaseKeys[iter.Key().Interface().(string)] = iter.Value().Interface().([]string)

			}
			s, err := json.MarshalToString(lowerCaseKeys)
			if err != nil {
				return nil, err
			}

			params.Set(fieldName, s)
		}
	}
	return params, nil
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
