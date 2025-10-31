// Copyright 2015 go-swagger maintainers
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

package generate

import (
	"log"

	"github.com/go-swagger/go-swagger/generator"
)

// Support generates the supporting files
type Support struct {
	WithShared
	WithModels
	WithOperations

	clientOptions
	serverOptions
	schemeOptions
	mediaOptions

	Name string `long:"name" short:"A" description:"the name of the application, defaults to a mangled value of info.title"`
}

func (s *Support) apply(opts *generator.GenOpts) {
	s.Shared.apply(opts)
	s.Models.apply(opts)
	s.Operations.apply(opts)
	s.clientOptions.apply(opts)
	s.serverOptions.apply(opts)
	s.schemeOptions.apply(opts)
	s.mediaOptions.apply(opts)
}

func (s *Support) generate(opts *generator.GenOpts) error {
	return generator.GenerateSupport(s.Name, s.Models.Models, s.Operations.Operations, opts)
}

func (s Support) log(_ string) {
	log.Println(`Generation completed!

For this generation to compile you need to have some packages in go.mod:

  * github.com/go-openapi/runtime
  * github.com/go-openapi/strfmt
  * github.com/jessevdk/go-flags

You can get these now with: go mod tidy`)
}

// Execute generates the supporting files file
func (s *Support) Execute(_ []string) error {
	return createSwagger(s)
}
