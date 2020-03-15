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
	"reflect"
	"strconv"
	"text/template"
)

var decodeTpl map[string]*template.Template

func init() {
	decodeTpl = make(map[string]*template.Template)

	funcs := map[string]string{
		"handlerNumeric":    handlerNumericTxt,
		"allowTokens":       allowTokensTxt,
		"handleFallback":    handleFallbackTxt,
		"handleString":      handleStringTxt,
		"handleObject":      handleObjectTxt,
		"handleArray":       handleArrayTxt,
		"handleSlice":       handleSliceTxt,
		"handleByteSlice":   handleByteSliceTxt,
		"handleBool":        handleBoolTxt,
		"handlePtr":         handlePtrTxt,
		"header":            headerTxt,
		"ujFunc":            ujFuncTxt,
		"handleUnmarshaler": handleUnmarshalerTxt,
	}

	tplFuncs := template.FuncMap{
		"getAllowTokens":  getAllowTokens,
		"getNumberSize":   getNumberSize,
		"getType":         getType,
		"handleField":     handleField,
		"handleFieldAddr": handleFieldAddr,
		"unquoteField":    unquoteField,
		"getTmpVarFor":    getTmpVarFor,
	}

	for k, v := range funcs {
		decodeTpl[k] = template.Must(template.New(k).Funcs(tplFuncs).Parse(v))
	}
}

type handlerNumeric struct {
	IC        *Inception
	Name      string
	ParseFunc string
	Typ       reflect.Type
	TakeAddr  bool
}

var handlerNumericTxt = `
{
	{{$ic := .IC}}

	if tok == fflib.FFTok_null {
		{{if eq .TakeAddr true}}
		{{.Name}} = nil
		{{end}}
	} else {
		{{if eq .ParseFunc "ParseFloat" }}
		tval, err := fflib.{{ .ParseFunc}}(fs.Output.Bytes(), {{getNumberSize .Typ}})
		{{else}}
		tval, err := fflib.{{ .ParseFunc}}(fs.Output.Bytes(), 10, {{getNumberSize .Typ}})
		{{end}}

		if err != nil {
			return fs.WrapErr(err)
		}
		{{if eq .TakeAddr true}}
		ttypval := {{getType $ic .Name .Typ}}(tval)
		{{.Name}} = &ttypval
		{{else}}
		{{.Name}} = {{getType $ic .Name .Typ}}(tval)
		{{end}}
	}
}
`

type allowTokens struct {
	Name   string
	Tokens []string
}

var allowTokensTxt = `
{
	if {{range $index, $element := .Tokens}}{{if ne $index 0 }}&&{{end}} tok != fflib.{{$element}}{{end}} {
		return fs.WrapErr(fmt.Errorf("cannot unmarshal %s into Go value for {{.Name}}", tok))
	}
}
`

type handleFallback struct {
	Name string
	Typ  reflect.Type
	Kind reflect.Kind
}

var handleFallbackTxt = `
{
	/* Falling back. type={{printf "%v" .Typ}} kind={{printf "%v" .Kind}} */
	tbuf, err := fs.CaptureField(tok)
	if err != nil {
		return fs.WrapErr(err)
	}

	err = json.Unmarshal(tbuf, &{{.Name}})
	if err != nil {
		return fs.WrapErr(err)
	}
}
`

type handleString struct {
	IC       *Inception
	Name     string
	Typ      reflect.Type
	TakeAddr bool
	Quoted   bool
}

var handleStringTxt = `
{
	{{$ic := .IC}}

	{{getAllowTokens .Typ.Name "FFTok_string" "FFTok_null"}}
	if tok == fflib.FFTok_null {
	{{if eq .TakeAddr true}}
		{{.Name}} = nil
	{{end}}
	} else {
	{{if eq .TakeAddr true}}
		var tval {{getType $ic .Name .Typ}}
		outBuf := fs.Output.Bytes()
		{{unquoteField .Quoted}}
		tval = {{getType $ic .Name .Typ}}(string(outBuf))
		{{.Name}} = &tval
	{{else}}
		outBuf := fs.Output.Bytes()
		{{unquoteField .Quoted}}
		{{.Name}} = {{getType $ic .Name .Typ}}(string(outBuf))
	{{end}}
	}
}
`

type handleObject struct {
	IC       *Inception
	Name     string
	Typ      reflect.Type
	Ptr      reflect.Kind
	TakeAddr bool
}

var handleObjectTxt = `
{
	{{$ic := .IC}}
	{{getAllowTokens .Typ.Name "FFTok_left_bracket" "FFTok_null"}}
	if tok == fflib.FFTok_null {
		{{.Name}} = nil
	} else {

		{{if eq .TakeAddr true}}
			{{if eq .Typ.Elem.Kind .Ptr }}
				{{if eq .Typ.Key.Kind .Ptr }}
				var tval = make(map[*{{getType $ic .Name .Typ.Key.Elem}}]*{{getType $ic .Name .Typ.Elem.Elem}}, 0)
				{{else}}
				var tval = make(map[{{getType $ic .Name .Typ.Key}}]*{{getType $ic .Name .Typ.Elem.Elem}}, 0)
				{{end}}
			{{else}}
				{{if eq .Typ.Key.Kind .Ptr }}
				var tval = make(map[*{{getType $ic .Name .Typ.Key.Elem}}]{{getType $ic .Name .Typ.Elem}}, 0)
				{{else}}
				var tval = make(map[{{getType $ic .Name .Typ.Key}}]{{getType $ic .Name .Typ.Elem}}, 0)
				{{end}}
			{{end}}
		{{else}}
			{{if eq .Typ.Elem.Kind .Ptr }}
				{{if eq .Typ.Key.Kind .Ptr }}
				{{.Name}} = make(map[*{{getType $ic .Name .Typ.Key.Elem}}]*{{getType $ic .Name .Typ.Elem.Elem}}, 0)
				{{else}}
				{{.Name}} = make(map[{{getType $ic .Name .Typ.Key}}]*{{getType $ic .Name .Typ.Elem.Elem}}, 0)
				{{end}}
			{{else}}
				{{if eq .Typ.Key.Kind .Ptr }}
				{{.Name}} = make(map[*{{getType $ic .Name .Typ.Key.Elem}}]{{getType $ic .Name .Typ.Elem}}, 0)
				{{else}}
				{{.Name}} = make(map[{{getType $ic .Name .Typ.Key}}]{{getType $ic .Name .Typ.Elem}}, 0)
				{{end}}
			{{end}}
		{{end}}

		wantVal := true

		for {
		{{$keyPtr := false}}
		{{if eq .Typ.Key.Kind .Ptr }}
			{{$keyPtr := true}}
			var k *{{getType $ic .Name .Typ.Key.Elem}}
		{{else}}
			var k {{getType $ic .Name .Typ.Key}}
		{{end}}

		{{$valPtr := false}}
		{{$tmpVar := getTmpVarFor .Name}}
		{{if eq .Typ.Elem.Kind .Ptr }}
			{{$valPtr := true}}
			var {{$tmpVar}} *{{getType $ic .Name .Typ.Elem.Elem}}
		{{else}}
			var {{$tmpVar}} {{getType $ic .Name .Typ.Elem}}
		{{end}}

			tok = fs.Scan()
			if tok == fflib.FFTok_error {
				goto tokerror
			}
			if tok == fflib.FFTok_right_bracket {
				break
			}

			if tok == fflib.FFTok_comma {
				if wantVal == true {
					// TODO(pquerna): this isn't an ideal error message, this handles
					// things like [,,,] as an array value.
					return fs.WrapErr(fmt.Errorf("wanted value token, but got token: %v", tok))
				}
				continue
			} else {
				wantVal = true
			}

			{{handleField .IC "k" .Typ.Key $keyPtr false}}

			// Expect ':' after key
			tok = fs.Scan()
			if tok != fflib.FFTok_colon {
				return fs.WrapErr(fmt.Errorf("wanted colon token, but got token: %v", tok))
			}

			tok = fs.Scan()
			{{handleField .IC $tmpVar .Typ.Elem $valPtr false}}

			{{if eq .TakeAddr true}}
			tval[k] = {{$tmpVar}}
			{{else}}
			{{.Name}}[k] = {{$tmpVar}}
			{{end}}
			wantVal = false
		}

		{{if eq .TakeAddr true}}
		{{.Name}} = &tval
		{{end}}
	}
}
`

type handleArray struct {
	IC              *Inception
	Name            string
	Typ             reflect.Type
	Ptr             reflect.Kind
	UseReflectToSet bool
	IsPtr           bool
}

var handleArrayTxt = `
{
	{{$ic := .IC}}
	{{getAllowTokens .Typ.Name "FFTok_left_brace" "FFTok_null"}}
	{{if eq .Typ.Elem.Kind .Ptr}}
		{{.Name}} = [{{.Typ.Len}}]*{{getType $ic .Name .Typ.Elem.Elem}}{}
	{{else}}
		{{.Name}} = [{{.Typ.Len}}]{{getType $ic .Name .Typ.Elem}}{}
	{{end}}
	if tok != fflib.FFTok_null {
		wantVal := true

		idx := 0
		for {
			{{$ptr := false}}
			{{$tmpVar := getTmpVarFor .Name}}
			{{if eq .Typ.Elem.Kind .Ptr }}
				{{$ptr := true}}
				var {{$tmpVar}} *{{getType $ic .Name .Typ.Elem.Elem}}
			{{else}}
				var {{$tmpVar}} {{getType $ic .Name .Typ.Elem}}
			{{end}}

			tok = fs.Scan()
			if tok == fflib.FFTok_error {
				goto tokerror
			}
			if tok == fflib.FFTok_right_brace {
				break
			}

			if tok == fflib.FFTok_comma {
				if wantVal == true {
					// TODO(pquerna): this isn't an ideal error message, this handles
					// things like [,,,] as an array value.
					return fs.WrapErr(fmt.Errorf("wanted value token, but got token: %v", tok))
				}
				continue
			} else {
				wantVal = true
			}

			{{handleField .IC $tmpVar .Typ.Elem $ptr false}}

			// Standard json.Unmarshal ignores elements out of array bounds,
			// that what we do as well.
			if idx < {{.Typ.Len}} {
				{{.Name}}[idx] = {{$tmpVar}}
				idx++
			}

			wantVal = false
		}
	}
}
`

var handleSliceTxt = `
{
	{{$ic := .IC}}
	{{getAllowTokens .Typ.Name "FFTok_left_brace" "FFTok_null"}}
	if tok == fflib.FFTok_null {
		{{.Name}} = nil
	} else {
		{{if eq .Typ.Elem.Kind .Ptr }}
			{{if eq .IsPtr true}}
				{{.Name}} = &[]*{{getType $ic .Name .Typ.Elem.Elem}}{}
			{{else}}
				{{.Name}} = []*{{getType $ic .Name .Typ.Elem.Elem}}{}
			{{end}}
		{{else}}
			{{if eq .IsPtr true}}
				{{.Name}} = &[]{{getType $ic .Name .Typ.Elem}}{}
			{{else}}
				{{.Name}} = []{{getType $ic .Name .Typ.Elem}}{}
			{{end}}
		{{end}}

		wantVal := true

		for {
			{{$ptr := false}}
			{{$tmpVar := getTmpVarFor .Name}}
			{{if eq .Typ.Elem.Kind .Ptr }}
				{{$ptr := true}}
				var {{$tmpVar}} *{{getType $ic .Name .Typ.Elem.Elem}}
			{{else}}
				var {{$tmpVar}} {{getType $ic .Name .Typ.Elem}}
			{{end}}

			tok = fs.Scan()
			if tok == fflib.FFTok_error {
				goto tokerror
			}
			if tok == fflib.FFTok_right_brace {
				break
			}

			if tok == fflib.FFTok_comma {
				if wantVal == true {
					// TODO(pquerna): this isn't an ideal error message, this handles
					// things like [,,,] as an array value.
					return fs.WrapErr(fmt.Errorf("wanted value token, but got token: %v", tok))
				}
				continue
			} else {
				wantVal = true
			}

			{{handleField .IC $tmpVar .Typ.Elem $ptr false}}
			{{if eq .IsPtr true}}
				*{{.Name}} = append(*{{.Name}}, {{$tmpVar}})
			{{else}}
				{{.Name}} = append({{.Name}}, {{$tmpVar}})
			{{end}}
			wantVal = false
		}
	}
}
`

var handleByteSliceTxt = `
{
	{{getAllowTokens .Typ.Name "FFTok_string" "FFTok_null"}}
	if tok == fflib.FFTok_null {
		{{.Name}} = nil
	} else {
		b := make([]byte, base64.StdEncoding.DecodedLen(fs.Output.Len()))
		n, err := base64.StdEncoding.Decode(b, fs.Output.Bytes())
		if err != nil {
			return fs.WrapErr(err)
		}
		{{if eq .UseReflectToSet true}}
			v := reflect.ValueOf(&{{.Name}}).Elem()
			v.SetBytes(b[0:n])
		{{else}}
			{{.Name}} = append([]byte(), b[0:n]...)
		{{end}}
	}
}
`

type handleBool struct {
	Name     string
	Typ      reflect.Type
	TakeAddr bool
}

var handleBoolTxt = `
{
	if tok == fflib.FFTok_null {
		{{if eq .TakeAddr true}}
		{{.Name}} = nil
		{{end}}
	} else {
		tmpb := fs.Output.Bytes()

		{{if eq .TakeAddr true}}
		var tval bool
		{{end}}

		if bytes.Compare([]byte{'t', 'r', 'u', 'e'}, tmpb) == 0 {
		{{if eq .TakeAddr true}}
			tval = true
		{{else}}
			{{.Name}} = true
		{{end}}
		} else if bytes.Compare([]byte{'f', 'a', 'l', 's', 'e'}, tmpb) == 0 {
		{{if eq .TakeAddr true}}
			tval = false
		{{else}}
			{{.Name}} = false
		{{end}}
		} else {
			err = errors.New("unexpected bytes for true/false value")
			return fs.WrapErr(err)
		}

		{{if eq .TakeAddr true}}
		{{.Name}} = &tval
		{{end}}
	}
}
`

type handlePtr struct {
	IC     *Inception
	Name   string
	Typ    reflect.Type
	Quoted bool
}

var handlePtrTxt = `
{
	{{$ic := .IC}}

	if tok == fflib.FFTok_null {
		{{.Name}} = nil
	} else {
		if {{.Name}} == nil {
			{{.Name}} = new({{getType $ic .Typ.Elem.Name .Typ.Elem}})
		}

		{{handleFieldAddr .IC .Name true .Typ.Elem false .Quoted}}
	}
}
`

type header struct {
	IC *Inception
	SI *StructInfo
}

var headerTxt = `
const (
	ffjt{{.SI.Name}}base = iota
	ffjt{{.SI.Name}}nosuchkey
	{{with $si := .SI}}
		{{range $index, $field := $si.Fields}}
			{{if ne $field.JsonName "-"}}
		ffjt{{$si.Name}}{{$field.Name}}
			{{end}}
		{{end}}
	{{end}}
)

{{with $si := .SI}}
	{{range $index, $field := $si.Fields}}
		{{if ne $field.JsonName "-"}}
var ffjKey{{$si.Name}}{{$field.Name}} = []byte({{$field.JsonName}})
		{{end}}
	{{end}}
{{end}}

`

type ujFunc struct {
	IC          *Inception
	SI          *StructInfo
	ValidValues []string
	ResetFields bool
}

var ujFuncTxt = `
{{$si := .SI}}
{{$ic := .IC}}

// UnmarshalJSON umarshall json - template of ffjson
func (j *{{.SI.Name}}) UnmarshalJSON(input []byte) error {
    fs := fflib.NewFFLexer(input)
    return j.UnmarshalJSONFFLexer(fs, fflib.FFParse_map_start)
}

// UnmarshalJSONFFLexer fast json unmarshall - template ffjson
func (j *{{.SI.Name}}) UnmarshalJSONFFLexer(fs *fflib.FFLexer, state fflib.FFParseState) error {
	var err error
	currentKey := ffjt{{.SI.Name}}base
	_ = currentKey
	tok := fflib.FFTok_init
	wantedTok := fflib.FFTok_init

				{{if eq .ResetFields true}}
				{{range $index, $field := $si.Fields}}
				var ffjSet{{$si.Name}}{{$field.Name}} = false
 				{{end}}
				{{end}}

mainparse:
	for {
		tok = fs.Scan()
		//	println(fmt.Sprintf("debug: tok: %v  state: %v", tok, state))
		if tok == fflib.FFTok_error {
			goto tokerror
		}

		switch state {

		case fflib.FFParse_map_start:
			if tok != fflib.FFTok_left_bracket {
				wantedTok = fflib.FFTok_left_bracket
				goto wrongtokenerror
			}
			state = fflib.FFParse_want_key
			continue

		case fflib.FFParse_after_value:
			if tok == fflib.FFTok_comma {
				state = fflib.FFParse_want_key
			} else if tok == fflib.FFTok_right_bracket {
				goto done
			} else {
				wantedTok = fflib.FFTok_comma
				goto wrongtokenerror
			}

		case fflib.FFParse_want_key:
			// json {} ended. goto exit. woo.
			if tok == fflib.FFTok_right_bracket {
				goto done
			}
			if tok != fflib.FFTok_string {
				wantedTok = fflib.FFTok_string
				goto wrongtokenerror
			}

			kn := fs.Output.Bytes()
			if len(kn) <= 0 {
				// "" case. hrm.
				currentKey = ffjt{{.SI.Name}}nosuchkey
				state = fflib.FFParse_want_colon
				goto mainparse
			} else {
				switch kn[0] {
				{{range $byte, $fields := $si.FieldsByFirstByte}}
				case '{{$byte}}':
					{{range $index, $field := $fields}}
						{{if ne $index 0 }}} else if {{else}}if {{end}} bytes.Equal(ffjKey{{$si.Name}}{{$field.Name}}, kn) {
						currentKey = ffjt{{$si.Name}}{{$field.Name}}
						state = fflib.FFParse_want_colon
						goto mainparse
					{{end}} }
				{{end}}
				}
				{{range $index, $field := $si.ReverseFields}}
				if {{$field.FoldFuncName}}(ffjKey{{$si.Name}}{{$field.Name}}, kn) {
					currentKey = ffjt{{$si.Name}}{{$field.Name}}
					state = fflib.FFParse_want_colon
					goto mainparse
				}
				{{end}}
				currentKey = ffjt{{.SI.Name}}nosuchkey
				state = fflib.FFParse_want_colon
				goto mainparse
			}

		case fflib.FFParse_want_colon:
			if tok != fflib.FFTok_colon {
				wantedTok = fflib.FFTok_colon
				goto wrongtokenerror
			}
			state = fflib.FFParse_want_value
			continue
		case fflib.FFParse_want_value:

			if {{range $index, $v := .ValidValues}}{{if ne $index 0 }}||{{end}}tok == fflib.{{$v}}{{end}} {
				switch currentKey {
				{{range $index, $field := $si.Fields}}
				case ffjt{{$si.Name}}{{$field.Name}}:
					goto handle_{{$field.Name}}
				{{end}}
				case ffjt{{$si.Name}}nosuchkey:
					err = fs.SkipField(tok)
					if err != nil {
						return fs.WrapErr(err)
					}
					state = fflib.FFParse_after_value
					goto mainparse
				}
			} else {
				goto wantedvalue
			}
		}
	}
{{range $index, $field := $si.Fields}}
handle_{{$field.Name}}:
	{{with $fieldName := $field.Name | printf "j.%s"}}
		{{handleField $ic $fieldName $field.Typ $field.Pointer $field.ForceString}}
		{{if eq $.ResetFields true}}
		ffjSet{{$si.Name}}{{$field.Name}} = true
		{{end}}
		state = fflib.FFParse_after_value
		goto mainparse
	{{end}}
{{end}}

wantedvalue:
	return fs.WrapErr(fmt.Errorf("wanted value token, but got token: %v", tok))
wrongtokenerror:
	return fs.WrapErr(fmt.Errorf("ffjson: wanted token: %v, but got token: %v output=%s", wantedTok, tok, fs.Output.String()))
tokerror:
	if fs.BigError != nil {
		return fs.WrapErr(fs.BigError)
	}
	err = fs.Error.ToError()
	if err != nil {
		return fs.WrapErr(err)
	}
	panic("ffjson-generated: unreachable, please report bug.")
done:
{{if eq .ResetFields true}}
{{range $index, $field := $si.Fields}}
	if !ffjSet{{$si.Name}}{{$field.Name}} {
	{{with $fieldName := $field.Name | printf "j.%s"}}
	{{if eq $field.Pointer true}}
		{{$fieldName}} = nil
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Interface), 10) + `}}
		{{$fieldName}} = nil
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Slice), 10) + `}}
		{{$fieldName}} = nil
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Array), 10) + `}}
		{{$fieldName}} = [{{$field.Typ.Len}}]{{getType $ic $fieldName $field.Typ.Elem}}{}
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Map), 10) + `}}
		{{$fieldName}} = nil
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Bool), 10) + `}}
		{{$fieldName}} = false
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.String), 10) + `}}
		{{$fieldName}} = ""
	{{else if eq $field.Typ.Kind ` + strconv.FormatUint(uint64(reflect.Struct), 10) + `}}
		{{$fieldName}} = {{getType $ic $fieldName $field.Typ}}{}
	{{else}}
		{{$fieldName}} = {{getType $ic $fieldName $field.Typ}}(0)
	{{end}}
	{{end}}
	}
{{end}}
{{end}}
	return nil
}
`

type handleUnmarshaler struct {
	IC                   *Inception
	Name                 string
	Typ                  reflect.Type
	Ptr                  reflect.Kind
	TakeAddr             bool
	UnmarshalJSONFFLexer bool
	Unmarshaler          bool
}

var handleUnmarshalerTxt = `
	{{$ic := .IC}}

	{{if eq .UnmarshalJSONFFLexer true}}
	{
		if tok == fflib.FFTok_null {
				{{if eq .Typ.Kind .Ptr }}
					{{.Name}} = nil
				{{end}}
				{{if eq .TakeAddr true }}
					{{.Name}} = nil
				{{end}}
		} else {
			{{if eq .Typ.Kind .Ptr }}
				if {{.Name}} == nil {
					{{.Name}} = new({{getType $ic .Typ.Elem.Name .Typ.Elem}})
				}
			{{end}}
			{{if eq .TakeAddr true }}
				if {{.Name}} == nil {
					{{.Name}} = new({{getType $ic .Typ.Name .Typ}})
				}
			{{end}}
			err = {{.Name}}.UnmarshalJSONFFLexer(fs, fflib.FFParse_want_key)
			if err != nil {
				return err
			}
		}
		state = fflib.FFParse_after_value
	}
	{{else}}
	{{if eq .Unmarshaler true}}
	{
		if tok == fflib.FFTok_null {
			{{if eq .TakeAddr true }}
				{{.Name}} = nil
			{{end}}
		} else {

			tbuf, err := fs.CaptureField(tok)
			if err != nil {
				return fs.WrapErr(err)
			}

			{{if eq .TakeAddr true }}
			if {{.Name}} == nil {
				{{.Name}} = new({{getType $ic .Typ.Name .Typ}})
			}
			{{end}}
			err = {{.Name}}.UnmarshalJSON(tbuf)
			if err != nil {
				return fs.WrapErr(err)
			}
		}
		state = fflib.FFParse_after_value
	}
	{{end}}
	{{end}}
`
