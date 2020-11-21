package params

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

func toString(s interface{}, field string) string {
	f := reflect.ValueOf(s).Elem().FieldByName(field)
	if reflect.Ptr == f.Kind() {
		f = f.Elem()
	}

	switch f.Kind() {
	case reflect.String:
		return f.String()
	case reflect.Bool:
		return strconv.FormatBool(f.Bool())
	case reflect.Map:
		if buf, err := json.Marshal(f.Interface()); err == nil {
			return string(buf)
		}
	case reflect.Slice:
		typ := reflect.TypeOf(f.Interface()).Elem()
		slice := reflect.MakeSlice(reflect.SliceOf(typ), f.Len(), f.Cap())
		reflect.Copy(slice, f)
		switch typ.Kind() {
		case reflect.String:
			s, ok := slice.Interface().([]string)
			if !ok {
				panic("Failed to cast slice")
			}
			return strings.Join(s, ",")
		}
	}
	return ""
}
