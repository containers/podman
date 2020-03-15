/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package ffjsoninception

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pquerna/ffjson/shared"
)

var validValues []string = []string{
	"FFTok_left_brace",
	"FFTok_left_bracket",
	"FFTok_integer",
	"FFTok_double",
	"FFTok_string",
	"FFTok_bool",
	"FFTok_null",
}

func CreateUnmarshalJSON(ic *Inception, si *StructInfo) error {
	out := ""
	ic.OutputImports[`fflib "github.com/pquerna/ffjson/fflib/v1"`] = true
	if len(si.Fields) > 0 {
		ic.OutputImports[`"bytes"`] = true
	}
	ic.OutputImports[`"fmt"`] = true

	out += tplStr(decodeTpl["header"], header{
		IC: ic,
		SI: si,
	})

	out += tplStr(decodeTpl["ujFunc"], ujFunc{
		SI:          si,
		IC:          ic,
		ValidValues: validValues,
		ResetFields: ic.ResetFields,
	})

	ic.OutputFuncs = append(ic.OutputFuncs, out)

	return nil
}

func handleField(ic *Inception, name string, typ reflect.Type, ptr bool, quoted bool) string {
	return handleFieldAddr(ic, name, false, typ, ptr, quoted)
}

func handleFieldAddr(ic *Inception, name string, takeAddr bool, typ reflect.Type, ptr bool, quoted bool) string {
	out := fmt.Sprintf("/* handler: %s type=%v kind=%v quoted=%t*/\n", name, typ, typ.Kind(), quoted)

	umlx := typ.Implements(unmarshalFasterType) || typeInInception(ic, typ, shared.MustDecoder)
	umlx = umlx || reflect.PtrTo(typ).Implements(unmarshalFasterType)

	umlstd := typ.Implements(unmarshalerType) || reflect.PtrTo(typ).Implements(unmarshalerType)

	out += tplStr(decodeTpl["handleUnmarshaler"], handleUnmarshaler{
		IC:                   ic,
		Name:                 name,
		Typ:                  typ,
		Ptr:                  reflect.Ptr,
		TakeAddr:             takeAddr || ptr,
		UnmarshalJSONFFLexer: umlx,
		Unmarshaler:          umlstd,
	})

	if umlx || umlstd {
		return out
	}

	// TODO(pquerna): generic handling of token type mismatching struct type
	switch typ.Kind() {
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:

		allowed := buildTokens(quoted, "FFTok_string", "FFTok_integer", "FFTok_null")
		out += getAllowTokens(typ.Name(), allowed...)

		out += getNumberHandler(ic, name, takeAddr || ptr, typ, "ParseInt")

	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:

		allowed := buildTokens(quoted, "FFTok_string", "FFTok_integer", "FFTok_null")
		out += getAllowTokens(typ.Name(), allowed...)

		out += getNumberHandler(ic, name, takeAddr || ptr, typ, "ParseUint")

	case reflect.Float32,
		reflect.Float64:

		allowed := buildTokens(quoted, "FFTok_string", "FFTok_double", "FFTok_integer", "FFTok_null")
		out += getAllowTokens(typ.Name(), allowed...)

		out += getNumberHandler(ic, name, takeAddr || ptr, typ, "ParseFloat")

	case reflect.Bool:
		ic.OutputImports[`"bytes"`] = true
		ic.OutputImports[`"errors"`] = true

		allowed := buildTokens(quoted, "FFTok_string", "FFTok_bool", "FFTok_null")
		out += getAllowTokens(typ.Name(), allowed...)

		out += tplStr(decodeTpl["handleBool"], handleBool{
			Name:     name,
			Typ:      typ,
			TakeAddr: takeAddr || ptr,
		})

	case reflect.Ptr:
		out += tplStr(decodeTpl["handlePtr"], handlePtr{
			IC:     ic,
			Name:   name,
			Typ:    typ,
			Quoted: quoted,
		})

	case reflect.Array,
		reflect.Slice:
		out += getArrayHandler(ic, name, typ, ptr)

	case reflect.String:
		// Is it a json.Number?
		if typ.PkgPath() == "encoding/json" && typ.Name() == "Number" {
			// Fall back to json package to rely on the valid number check.
			// See: https://github.com/golang/go/blob/f05c3aa24d815cd3869153750c9875e35fc48a6e/src/encoding/json/decode.go#L897
			ic.OutputImports[`"encoding/json"`] = true
			out += tplStr(decodeTpl["handleFallback"], handleFallback{
				Name: name,
				Typ:  typ,
				Kind: typ.Kind(),
			})
		} else {
			out += tplStr(decodeTpl["handleString"], handleString{
				IC:       ic,
				Name:     name,
				Typ:      typ,
				TakeAddr: takeAddr || ptr,
				Quoted:   quoted,
			})
		}
	case reflect.Interface:
		ic.OutputImports[`"encoding/json"`] = true
		out += tplStr(decodeTpl["handleFallback"], handleFallback{
			Name: name,
			Typ:  typ,
			Kind: typ.Kind(),
		})
	case reflect.Map:
		out += tplStr(decodeTpl["handleObject"], handleObject{
			IC:       ic,
			Name:     name,
			Typ:      typ,
			Ptr:      reflect.Ptr,
			TakeAddr: takeAddr || ptr,
		})
	default:
		ic.OutputImports[`"encoding/json"`] = true
		out += tplStr(decodeTpl["handleFallback"], handleFallback{
			Name: name,
			Typ:  typ,
			Kind: typ.Kind(),
		})
	}

	return out
}

func getArrayHandler(ic *Inception, name string, typ reflect.Type, ptr bool) string {
	if typ.Kind() == reflect.Slice && typ.Elem().Kind() == reflect.Uint8 {
		ic.OutputImports[`"encoding/base64"`] = true
		useReflectToSet := false
		if typ.Elem().Name() != "byte" {
			ic.OutputImports[`"reflect"`] = true
			useReflectToSet = true
		}

		return tplStr(decodeTpl["handleByteSlice"], handleArray{
			IC:              ic,
			Name:            name,
			Typ:             typ,
			Ptr:             reflect.Ptr,
			UseReflectToSet: useReflectToSet,
		})
	}

	if typ.Elem().Kind() == reflect.Struct && typ.Elem().Name() != "" {
		goto sliceOrArray
	}

	if (typ.Elem().Kind() == reflect.Struct || typ.Elem().Kind() == reflect.Map) ||
		typ.Elem().Kind() == reflect.Array || typ.Elem().Kind() == reflect.Slice &&
		typ.Elem().Name() == "" {
		ic.OutputImports[`"encoding/json"`] = true

		return tplStr(decodeTpl["handleFallback"], handleFallback{
			Name: name,
			Typ:  typ,
			Kind: typ.Kind(),
		})
	}

sliceOrArray:

	if typ.Kind() == reflect.Array {
		return tplStr(decodeTpl["handleArray"], handleArray{
			IC:    ic,
			Name:  name,
			Typ:   typ,
			IsPtr: ptr,
			Ptr:   reflect.Ptr,
		})
	}

	return tplStr(decodeTpl["handleSlice"], handleArray{
		IC:    ic,
		Name:  name,
		Typ:   typ,
		IsPtr: ptr,
		Ptr:   reflect.Ptr,
	})
}

func getAllowTokens(name string, tokens ...string) string {
	return tplStr(decodeTpl["allowTokens"], allowTokens{
		Name:   name,
		Tokens: tokens,
	})
}

func getNumberHandler(ic *Inception, name string, takeAddr bool, typ reflect.Type, parsefunc string) string {
	return tplStr(decodeTpl["handlerNumeric"], handlerNumeric{
		IC:        ic,
		Name:      name,
		ParseFunc: parsefunc,
		TakeAddr:  takeAddr,
		Typ:       typ,
	})
}

func getNumberSize(typ reflect.Type) string {
	return fmt.Sprintf("%d", typ.Bits())
}

func getType(ic *Inception, name string, typ reflect.Type) string {
	s := typ.Name()

	if typ.PkgPath() != "" && typ.PkgPath() != ic.PackagePath {
		path := removeVendor(typ.PkgPath())
		ic.OutputImports[`"`+path+`"`] = true
		s = typ.String()
	}

	if s == "" {
		return typ.String()
	}

	return s
}

// removeVendor removes everything before and including a '/vendor/'
// substring in the package path.
// This is needed becuase that full path can't be used in the
// import statement.
func removeVendor(path string) string {
	i := strings.Index(path, "/vendor/")
	if i == -1 {
		return path
	}
	return path[i+8:]
}

func buildTokens(containsOptional bool, optional string, required ...string) []string {
	if containsOptional {
		return append(required, optional)
	}

	return required
}

func unquoteField(quoted bool) string {
	// The outer quote of a string is already stripped out by
	// the lexer. We need to check if the inner string is also
	// quoted. If so, we will decode it as json string. If decoding
	// fails, we will use the original string
	if quoted {
		return `
		unquoted, ok := fflib.UnquoteBytes(outBuf)
		if ok {
			outBuf = unquoted
		}
		`
	}
	return ""
}

func getTmpVarFor(name string) string {
	return "tmp" + strings.Replace(strings.Title(name), ".", "", -1)
}
