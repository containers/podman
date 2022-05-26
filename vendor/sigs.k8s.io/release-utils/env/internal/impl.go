/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import "os"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//go:generate /usr/bin/env bash -c "cat ../../scripts/boilerplate/boilerplate.generatego.txt internalfakes/fake_impl.go > internalfakes/_fake_impl.go && mv internalfakes/_fake_impl.go internalfakes/fake_impl.go"

//counterfeiter:generate . impl
type impl interface {
	LookupEnv(key string) (string, bool)
}

type defImpl struct{}

var Impl impl = &defImpl{}

func (defImpl) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}
