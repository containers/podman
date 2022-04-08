/*
   Copyright © 2022 The CDI Authors

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

package cdi

import (
	"github.com/container-orchestrated-devices/container-device-interface/schema"
	cdi "github.com/container-orchestrated-devices/container-device-interface/specs-go"
)

const (
	// DefaultExternalSchema is the JSON schema to load if found.
	DefaultExternalSchema = "/etc/cdi/schema/schema.json"
)

// SetSchema sets the Spec JSON validation schema to use.
func SetSchema(name string) error {
	s, err := schema.Load(name)
	if err != nil {
		return err
	}
	schema.Set(s)
	return nil
}

// Validate CDI Spec against JSON Schema.
func validateWithSchema(raw *cdi.Spec) error {
	return schema.ValidateType(raw)
}

func init() {
	SetSchema(DefaultExternalSchema)
}
