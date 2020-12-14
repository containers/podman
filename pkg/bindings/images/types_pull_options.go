package images

import (
	"net/url"
	"reflect"
	"strconv"

	"github.com/containers/common/pkg/config"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.

Created 2020-12-16 11:47:09.104988433 -0600 CST m=+0.000274515
*/

// Changed
func (o *PullOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *PullOptions) ToParams() (url.Values, error) {
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
			slice := reflect.MakeSlice(reflect.SliceOf(typ), f.Len(), f.Cap())
			switch typ.Kind() {
			case reflect.String:
				s, ok := slice.Interface().([]string)
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

// WithAllTags
func (o *PullOptions) WithAllTags(value bool) *PullOptions {
	v := &value
	o.AllTags = v
	return o
}

// GetAllTags
func (o *PullOptions) GetAllTags() bool {
	var allTags bool
	if o.AllTags == nil {
		return allTags
	}
	return *o.AllTags
}

// WithAuthfile
func (o *PullOptions) WithAuthfile(value string) *PullOptions {
	v := &value
	o.Authfile = v
	return o
}

// GetAuthfile
func (o *PullOptions) GetAuthfile() string {
	var authfile string
	if o.Authfile == nil {
		return authfile
	}
	return *o.Authfile
}

// WithCertDir
func (o *PullOptions) WithCertDir(value string) *PullOptions {
	v := &value
	o.CertDir = v
	return o
}

// GetCertDir
func (o *PullOptions) GetCertDir() string {
	var certDir string
	if o.CertDir == nil {
		return certDir
	}
	return *o.CertDir
}

// WithUsername
func (o *PullOptions) WithUsername(value string) *PullOptions {
	v := &value
	o.Username = v
	return o
}

// GetUsername
func (o *PullOptions) GetUsername() string {
	var username string
	if o.Username == nil {
		return username
	}
	return *o.Username
}

// WithPassword
func (o *PullOptions) WithPassword(value string) *PullOptions {
	v := &value
	o.Password = v
	return o
}

// GetPassword
func (o *PullOptions) GetPassword() string {
	var password string
	if o.Password == nil {
		return password
	}
	return *o.Password
}

// WithOverrideArch
func (o *PullOptions) WithOverrideArch(value string) *PullOptions {
	v := &value
	o.OverrideArch = v
	return o
}

// GetOverrideArch
func (o *PullOptions) GetOverrideArch() string {
	var overrideArch string
	if o.OverrideArch == nil {
		return overrideArch
	}
	return *o.OverrideArch
}

// WithOverrideOS
func (o *PullOptions) WithOverrideOS(value string) *PullOptions {
	v := &value
	o.OverrideOS = v
	return o
}

// GetOverrideOS
func (o *PullOptions) GetOverrideOS() string {
	var overrideOS string
	if o.OverrideOS == nil {
		return overrideOS
	}
	return *o.OverrideOS
}

// WithOverrideVariant
func (o *PullOptions) WithOverrideVariant(value string) *PullOptions {
	v := &value
	o.OverrideVariant = v
	return o
}

// GetOverrideVariant
func (o *PullOptions) GetOverrideVariant() string {
	var overrideVariant string
	if o.OverrideVariant == nil {
		return overrideVariant
	}
	return *o.OverrideVariant
}

// WithQuiet
func (o *PullOptions) WithQuiet(value bool) *PullOptions {
	v := &value
	o.Quiet = v
	return o
}

// GetQuiet
func (o *PullOptions) GetQuiet() bool {
	var quiet bool
	if o.Quiet == nil {
		return quiet
	}
	return *o.Quiet
}

// WithSignaturePolicy
func (o *PullOptions) WithSignaturePolicy(value string) *PullOptions {
	v := &value
	o.SignaturePolicy = v
	return o
}

// GetSignaturePolicy
func (o *PullOptions) GetSignaturePolicy() string {
	var signaturePolicy string
	if o.SignaturePolicy == nil {
		return signaturePolicy
	}
	return *o.SignaturePolicy
}

// WithSkipTLSVerify
func (o *PullOptions) WithSkipTLSVerify(value bool) *PullOptions {
	v := &value
	o.SkipTLSVerify = v
	return o
}

// GetSkipTLSVerify
func (o *PullOptions) GetSkipTLSVerify() bool {
	var skipTLSVerify bool
	if o.SkipTLSVerify == nil {
		return skipTLSVerify
	}
	return *o.SkipTLSVerify
}

// WithPullPolicy
func (o *PullOptions) WithPullPolicy(value config.PullPolicy) *PullOptions {
	v := &value
	o.PullPolicy = v
	return o
}

// GetPullPolicy
func (o *PullOptions) GetPullPolicy() config.PullPolicy {
	var pullPolicy config.PullPolicy
	if o.PullPolicy == nil {
		return pullPolicy
	}
	return *o.PullPolicy
}
