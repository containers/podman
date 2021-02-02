package util

import (
	"reflect"
	"strconv"
)

func IsSimpleType(f reflect.Value) bool {
	switch f.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64, reflect.String:
		return true
	}
	return false
}

func SimpleTypeToParam(f reflect.Value) string {
	switch f.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(f.Bool())
	case reflect.Int, reflect.Int64:
		// f.Int() is always an int64
		return strconv.FormatInt(f.Int(), 10)
	case reflect.Uint, reflect.Uint64:
		// f.Uint() is always an uint64
		return strconv.FormatUint(f.Uint(), 10)
	case reflect.String:
		return f.String()
	}
	panic("the input parameter is not a simple type")
}
