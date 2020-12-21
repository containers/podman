package images

import (
	"net/url"
	"reflect"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.

Created 2020-12-18 15:58:27.881232044 -0600 CST m=+0.000242458
*/

// Changed
func (o *PushOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *PushOptions) ToParams() (url.Values, error) {
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

// WithAuthfile
func (o *PushOptions) WithAuthfile(value string) *PushOptions {
	v := &value
	o.Authfile = v
	return o
}

// GetAuthfile
func (o *PushOptions) GetAuthfile() string {
	var authfile string
	if o.Authfile == nil {
		return authfile
	}
	return *o.Authfile
}

// WithCertDir
func (o *PushOptions) WithCertDir(value string) *PushOptions {
	v := &value
	o.CertDir = v
	return o
}

// GetCertDir
func (o *PushOptions) GetCertDir() string {
	var certDir string
	if o.CertDir == nil {
		return certDir
	}
	return *o.CertDir
}

// WithCompress
func (o *PushOptions) WithCompress(value bool) *PushOptions {
	v := &value
	o.Compress = v
	return o
}

// GetCompress
func (o *PushOptions) GetCompress() bool {
	var compress bool
	if o.Compress == nil {
		return compress
	}
	return *o.Compress
}

// WithUsername
func (o *PushOptions) WithUsername(value string) *PushOptions {
	v := &value
	o.Username = v
	return o
}

// GetUsername
func (o *PushOptions) GetUsername() string {
	var username string
	if o.Username == nil {
		return username
	}
	return *o.Username
}

// WithPassword
func (o *PushOptions) WithPassword(value string) *PushOptions {
	v := &value
	o.Password = v
	return o
}

// GetPassword
func (o *PushOptions) GetPassword() string {
	var password string
	if o.Password == nil {
		return password
	}
	return *o.Password
}

// WithDigestFile
func (o *PushOptions) WithDigestFile(value string) *PushOptions {
	v := &value
	o.DigestFile = v
	return o
}

// GetDigestFile
func (o *PushOptions) GetDigestFile() string {
	var digestFile string
	if o.DigestFile == nil {
		return digestFile
	}
	return *o.DigestFile
}

// WithFormat
func (o *PushOptions) WithFormat(value string) *PushOptions {
	v := &value
	o.Format = v
	return o
}

// GetFormat
func (o *PushOptions) GetFormat() string {
	var format string
	if o.Format == nil {
		return format
	}
	return *o.Format
}

// WithQuiet
func (o *PushOptions) WithQuiet(value bool) *PushOptions {
	v := &value
	o.Quiet = v
	return o
}

// GetQuiet
func (o *PushOptions) GetQuiet() bool {
	var quiet bool
	if o.Quiet == nil {
		return quiet
	}
	return *o.Quiet
}

// WithRemoveSignatures
func (o *PushOptions) WithRemoveSignatures(value bool) *PushOptions {
	v := &value
	o.RemoveSignatures = v
	return o
}

// GetRemoveSignatures
func (o *PushOptions) GetRemoveSignatures() bool {
	var removeSignatures bool
	if o.RemoveSignatures == nil {
		return removeSignatures
	}
	return *o.RemoveSignatures
}

// WithSignaturePolicy
func (o *PushOptions) WithSignaturePolicy(value string) *PushOptions {
	v := &value
	o.SignaturePolicy = v
	return o
}

// GetSignaturePolicy
func (o *PushOptions) GetSignaturePolicy() string {
	var signaturePolicy string
	if o.SignaturePolicy == nil {
		return signaturePolicy
	}
	return *o.SignaturePolicy
}

// WithSignBy
func (o *PushOptions) WithSignBy(value string) *PushOptions {
	v := &value
	o.SignBy = v
	return o
}

// GetSignBy
func (o *PushOptions) GetSignBy() string {
	var signBy string
	if o.SignBy == nil {
		return signBy
	}
	return *o.SignBy
}

// WithSkipTLSVerify
func (o *PushOptions) WithSkipTLSVerify(value bool) *PushOptions {
	v := &value
	o.SkipTLSVerify = v
	return o
}

// GetSkipTLSVerify
func (o *PushOptions) GetSkipTLSVerify() bool {
	var skipTLSVerify bool
	if o.SkipTLSVerify == nil {
		return skipTLSVerify
	}
	return *o.SkipTLSVerify
}
