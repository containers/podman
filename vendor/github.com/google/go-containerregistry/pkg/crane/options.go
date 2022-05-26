// Copyright 2019 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crane

import (
	"context"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Options hold the options that crane uses when calling other packages.
type Options struct {
	Name     []name.Option
	Remote   []remote.Option
	Platform *v1.Platform
}

// GetOptions exposes the underlying []remote.Option, []name.Option, and
// platform, based on the passed Option. Generally, you shouldn't need to use
// this unless you've painted yourself into a dependency corner as we have
// with the crane and gcrane cli packages.
func GetOptions(opts ...Option) Options {
	return makeOptions(opts...)
}

func makeOptions(opts ...Option) Options {
	opt := Options{
		Remote: []remote.Option{
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
		},
	}
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

// Option is a functional option for crane.
type Option func(*Options)

// WithTransport is a functional option for overriding the default transport
// for remote operations.
func WithTransport(t http.RoundTripper) Option {
	return func(o *Options) {
		o.Remote = append(o.Remote, remote.WithTransport(t))
	}
}

// Insecure is an Option that allows image references to be fetched without TLS.
func Insecure(o *Options) {
	o.Name = append(o.Name, name.Insecure)
}

// WithPlatform is an Option to specify the platform.
func WithPlatform(platform *v1.Platform) Option {
	return func(o *Options) {
		if platform != nil {
			o.Remote = append(o.Remote, remote.WithPlatform(*platform))
		}
		o.Platform = platform
	}
}

// WithAuthFromKeychain is a functional option for overriding the default
// authenticator for remote operations, using an authn.Keychain to find
// credentials.
//
// By default, crane will use authn.DefaultKeychain.
func WithAuthFromKeychain(keys authn.Keychain) Option {
	return func(o *Options) {
		// Replace the default keychain at position 0.
		o.Remote[0] = remote.WithAuthFromKeychain(keys)
	}
}

// WithAuth is a functional option for overriding the default authenticator
// for remote operations.
//
// By default, crane will use authn.DefaultKeychain.
func WithAuth(auth authn.Authenticator) Option {
	return func(o *Options) {
		// Replace the default keychain at position 0.
		o.Remote[0] = remote.WithAuth(auth)
	}
}

// WithUserAgent adds the given string to the User-Agent header for any HTTP
// requests.
func WithUserAgent(ua string) Option {
	return func(o *Options) {
		o.Remote = append(o.Remote, remote.WithUserAgent(ua))
	}
}

// WithContext is a functional option for setting the context.
func WithContext(ctx context.Context) Option {
	return func(o *Options) {
		o.Remote = append(o.Remote, remote.WithContext(ctx))
	}
}
