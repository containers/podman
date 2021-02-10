package play

import (
	"net/url"
	"reflect"
	"strconv"
)

/*
This file is generated automatically by go generate.  Do not edit.
*/

// Changed
func (o *KubeOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *KubeOptions) ToParams() (url.Values, error) {
	params := url.Values{}

	if o == nil {
		return params, nil
	}

	if o.Authfile != nil {
		params.Set("authfile", *o.Authfile)
	}

	if o.CertDir != nil {
		params.Set("certdir", *o.CertDir)
	}

	if o.Username != nil {
		params.Set("username", *o.Username)
	}

	if o.Password != nil {
		params.Set("password", *o.Password)
	}

	if o.Network != nil {
		params.Set("network", *o.Network)
	}

	if o.Quiet != nil {
		params.Set("quiet", strconv.FormatBool(*o.Quiet))
	}

	if o.SignaturePolicy != nil {
		params.Set("signaturepolicy", *o.SignaturePolicy)
	}

	if o.SkipTLSVerify != nil {
		params.Set("skiptlsverify", strconv.FormatBool(*o.SkipTLSVerify))
	}

	if o.SeccompProfileRoot != nil {
		params.Set("seccompprofileroot", *o.SeccompProfileRoot)
	}

	if o.ConfigMaps != nil {
		for _, val := range o.ConfigMaps {
			params.Add("configmaps", val)
		}
	}

	if o.LogDriver != nil {
		params.Set("logdriver", *o.LogDriver)
	}

	if o.Start != nil {
		params.Set("start", strconv.FormatBool(*o.Start))
	}

	return params, nil
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

// WithConfigMaps
func (o *KubeOptions) WithConfigMaps(value []string) *KubeOptions {
	v := value
	o.ConfigMaps = v
	return o
}

// GetConfigMaps
func (o *KubeOptions) GetConfigMaps() []string {
	var configMaps []string
	if o.ConfigMaps == nil {
		return configMaps
	}
	return o.ConfigMaps
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
