//go:build !go1.21
// +build !go1.21

/*
 * Copyright 2021 ByteDance Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package resolver

import (
    _ `encoding/json`
    `reflect`
    _ `unsafe`
)

type StdField struct {
    name        string
    nameBytes   []byte
    equalFold   func()
    nameNonEsc  string
    nameEscHTML string
    tag         bool
    index       []int
    typ         reflect.Type
    omitEmpty   bool
    quoted      bool
    encoder     func()
}

type StdStructFields struct {
    list      []StdField
    nameIndex map[string]int
}

//go:noescape
//go:linkname typeFields encoding/json.typeFields
func typeFields(_ reflect.Type) StdStructFields

func handleOmitZero(f StdField, fv *FieldMeta) {}
