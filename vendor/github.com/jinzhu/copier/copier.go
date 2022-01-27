package copier

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

// These flags define options for tag handling
const (
	// Denotes that a destination field must be copied to. If copying fails then a panic will ensue.
	tagMust uint8 = 1 << iota

	// Denotes that the program should not panic when the must flag is on and
	// value is not copied. The program will return an error instead.
	tagNoPanic

	// Ignore a destination field from being copied to.
	tagIgnore

	// Denotes that the value as been copied
	hasCopied

	// Some default converter types for a nicer syntax
	String  string  = ""
	Bool    bool    = false
	Int     int     = 0
	Float32 float32 = 0
	Float64 float64 = 0
)

// Option sets copy options
type Option struct {
	// setting this value to true will ignore copying zero values of all the fields, including bools, as well as a
	// struct having all it's fields set to their zero values respectively (see IsZero() in reflect/value.go)
	IgnoreEmpty bool
	DeepCopy    bool
	Converters  []TypeConverter
}

type TypeConverter struct {
	SrcType interface{}
	DstType interface{}
	Fn      func(src interface{}) (interface{}, error)
}

type converterPair struct {
	SrcType reflect.Type
	DstType reflect.Type
}

// Tag Flags
type flags struct {
	BitFlags  map[string]uint8
	SrcNames  tagNameMapping
	DestNames tagNameMapping
}

// Field Tag name mapping
type tagNameMapping struct {
	FieldNameToTag map[string]string
	TagToFieldName map[string]string
}

// Copy copy things
func Copy(toValue interface{}, fromValue interface{}) (err error) {
	return copier(toValue, fromValue, Option{})
}

// CopyWithOption copy with option
func CopyWithOption(toValue interface{}, fromValue interface{}, opt Option) (err error) {
	return copier(toValue, fromValue, opt)
}

func copier(toValue interface{}, fromValue interface{}, opt Option) (err error) {
	var (
		isSlice    bool
		amount     = 1
		from       = indirect(reflect.ValueOf(fromValue))
		to         = indirect(reflect.ValueOf(toValue))
		converters map[converterPair]TypeConverter
	)

	// save convertes into map for faster lookup
	for i := range opt.Converters {
		if converters == nil {
			converters = make(map[converterPair]TypeConverter)
		}

		pair := converterPair{
			SrcType: reflect.TypeOf(opt.Converters[i].SrcType),
			DstType: reflect.TypeOf(opt.Converters[i].DstType),
		}

		converters[pair] = opt.Converters[i]
	}

	if !to.CanAddr() {
		return ErrInvalidCopyDestination
	}

	// Return is from value is invalid
	if !from.IsValid() {
		return ErrInvalidCopyFrom
	}

	fromType, isPtrFrom := indirectType(from.Type())
	toType, _ := indirectType(to.Type())

	if fromType.Kind() == reflect.Interface {
		fromType = reflect.TypeOf(from.Interface())
	}

	if toType.Kind() == reflect.Interface {
		toType, _ = indirectType(reflect.TypeOf(to.Interface()))
		oldTo := to
		to = reflect.New(reflect.TypeOf(to.Interface())).Elem()
		defer func() {
			oldTo.Set(to)
		}()
	}

	// Just set it if possible to assign for normal types
	if from.Kind() != reflect.Slice && from.Kind() != reflect.Struct && from.Kind() != reflect.Map && (from.Type().AssignableTo(to.Type()) || from.Type().ConvertibleTo(to.Type())) {
		if !isPtrFrom || !opt.DeepCopy {
			to.Set(from.Convert(to.Type()))
		} else {
			fromCopy := reflect.New(from.Type())
			fromCopy.Set(from.Elem())
			to.Set(fromCopy.Convert(to.Type()))
		}
		return
	}

	if from.Kind() != reflect.Slice && fromType.Kind() == reflect.Map && toType.Kind() == reflect.Map {
		if !fromType.Key().ConvertibleTo(toType.Key()) {
			return ErrMapKeyNotMatch
		}

		if to.IsNil() {
			to.Set(reflect.MakeMapWithSize(toType, from.Len()))
		}

		for _, k := range from.MapKeys() {
			toKey := indirect(reflect.New(toType.Key()))
			if !set(toKey, k, opt.DeepCopy, converters) {
				return fmt.Errorf("%w map, old key: %v, new key: %v", ErrNotSupported, k.Type(), toType.Key())
			}

			elemType := toType.Elem()
			if elemType.Kind() != reflect.Slice {
				elemType, _ = indirectType(elemType)
			}
			toValue := indirect(reflect.New(elemType))
			if !set(toValue, from.MapIndex(k), opt.DeepCopy, converters) {
				if err = copier(toValue.Addr().Interface(), from.MapIndex(k).Interface(), opt); err != nil {
					return err
				}
			}

			for {
				if elemType == toType.Elem() {
					to.SetMapIndex(toKey, toValue)
					break
				}
				elemType = reflect.PtrTo(elemType)
				toValue = toValue.Addr()
			}
		}
		return
	}

	if from.Kind() == reflect.Slice && to.Kind() == reflect.Slice && fromType.ConvertibleTo(toType) {
		if to.IsNil() {
			slice := reflect.MakeSlice(reflect.SliceOf(to.Type().Elem()), from.Len(), from.Cap())
			to.Set(slice)
		}

		for i := 0; i < from.Len(); i++ {
			if to.Len() < i+1 {
				to.Set(reflect.Append(to, reflect.New(to.Type().Elem()).Elem()))
			}

			if !set(to.Index(i), from.Index(i), opt.DeepCopy, converters) {
				// ignore error while copy slice element
				err = copier(to.Index(i).Addr().Interface(), from.Index(i).Interface(), opt)
				if err != nil {
					continue
				}
			}
		}
		return
	}

	if fromType.Kind() != reflect.Struct || toType.Kind() != reflect.Struct {
		// skip not supported type
		return
	}

	if from.Kind() == reflect.Slice || to.Kind() == reflect.Slice {
		isSlice = true
		if from.Kind() == reflect.Slice {
			amount = from.Len()
		}
	}

	for i := 0; i < amount; i++ {
		var dest, source reflect.Value

		if isSlice {
			// source
			if from.Kind() == reflect.Slice {
				source = indirect(from.Index(i))
			} else {
				source = indirect(from)
			}
			// dest
			dest = indirect(reflect.New(toType).Elem())
		} else {
			source = indirect(from)
			dest = indirect(to)
		}

		destKind := dest.Kind()
		initDest := false
		if destKind == reflect.Interface {
			initDest = true
			dest = indirect(reflect.New(toType))
		}

		// Get tag options
		flgs, err := getFlags(dest, source, toType, fromType)
		if err != nil {
			return err
		}

		// check source
		if source.IsValid() {
			copyUnexportedStructFields(dest, source)

			// Copy from source field to dest field or method
			fromTypeFields := deepFields(fromType)
			for _, field := range fromTypeFields {
				name := field.Name

				// Get bit flags for field
				fieldFlags, _ := flgs.BitFlags[name]

				// Check if we should ignore copying
				if (fieldFlags & tagIgnore) != 0 {
					continue
				}

				srcFieldName, destFieldName := getFieldName(name, flgs)
				if fromField := source.FieldByName(srcFieldName); fromField.IsValid() && !shouldIgnore(fromField, opt.IgnoreEmpty) {
					// process for nested anonymous field
					destFieldNotSet := false
					if f, ok := dest.Type().FieldByName(destFieldName); ok {
						for idx := range f.Index {
							destField := dest.FieldByIndex(f.Index[:idx+1])

							if destField.Kind() != reflect.Ptr {
								continue
							}

							if !destField.IsNil() {
								continue
							}
							if !destField.CanSet() {
								destFieldNotSet = true
								break
							}

							// destField is a nil pointer that can be set
							newValue := reflect.New(destField.Type().Elem())
							destField.Set(newValue)
						}
					}

					if destFieldNotSet {
						break
					}

					toField := dest.FieldByName(destFieldName)
					if toField.IsValid() {
						if toField.CanSet() {
							if !set(toField, fromField, opt.DeepCopy, converters) {
								if err := copier(toField.Addr().Interface(), fromField.Interface(), opt); err != nil {
									return err
								}
							}
							if fieldFlags != 0 {
								// Note that a copy was made
								flgs.BitFlags[name] = fieldFlags | hasCopied
							}
						}
					} else {
						// try to set to method
						var toMethod reflect.Value
						if dest.CanAddr() {
							toMethod = dest.Addr().MethodByName(destFieldName)
						} else {
							toMethod = dest.MethodByName(destFieldName)
						}

						if toMethod.IsValid() && toMethod.Type().NumIn() == 1 && fromField.Type().AssignableTo(toMethod.Type().In(0)) {
							toMethod.Call([]reflect.Value{fromField})
						}
					}
				}
			}

			// Copy from from method to dest field
			for _, field := range deepFields(toType) {
				name := field.Name
				srcFieldName, destFieldName := getFieldName(name, flgs)

				var fromMethod reflect.Value
				if source.CanAddr() {
					fromMethod = source.Addr().MethodByName(srcFieldName)
				} else {
					fromMethod = source.MethodByName(srcFieldName)
				}

				if fromMethod.IsValid() && fromMethod.Type().NumIn() == 0 && fromMethod.Type().NumOut() == 1 && !shouldIgnore(fromMethod, opt.IgnoreEmpty) {
					if toField := dest.FieldByName(destFieldName); toField.IsValid() && toField.CanSet() {
						values := fromMethod.Call([]reflect.Value{})
						if len(values) >= 1 {
							set(toField, values[0], opt.DeepCopy, converters)
						}
					}
				}
			}
		}

		if isSlice && to.Kind() == reflect.Slice {
			if dest.Addr().Type().AssignableTo(to.Type().Elem()) {
				if to.Len() < i+1 {
					to.Set(reflect.Append(to, dest.Addr()))
				} else {
					if !set(to.Index(i), dest.Addr(), opt.DeepCopy, converters) {
						// ignore error while copy slice element
						err = copier(to.Index(i).Addr().Interface(), dest.Addr().Interface(), opt)
						if err != nil {
							continue
						}
					}
				}
			} else if dest.Type().AssignableTo(to.Type().Elem()) {
				if to.Len() < i+1 {
					to.Set(reflect.Append(to, dest))
				} else {
					if !set(to.Index(i), dest, opt.DeepCopy, converters) {
						// ignore error while copy slice element
						err = copier(to.Index(i).Addr().Interface(), dest.Interface(), opt)
						if err != nil {
							continue
						}
					}
				}
			}
		} else if initDest {
			to.Set(dest)
		}

		err = checkBitFlags(flgs.BitFlags)
	}

	return
}

func copyUnexportedStructFields(to, from reflect.Value) {
	if from.Kind() != reflect.Struct || to.Kind() != reflect.Struct || !from.Type().AssignableTo(to.Type()) {
		return
	}

	// create a shallow copy of 'to' to get all fields
	tmp := indirect(reflect.New(to.Type()))
	tmp.Set(from)

	// revert exported fields
	for i := 0; i < to.NumField(); i++ {
		if tmp.Field(i).CanSet() {
			tmp.Field(i).Set(to.Field(i))
		}
	}
	to.Set(tmp)
}

func shouldIgnore(v reflect.Value, ignoreEmpty bool) bool {
	if !ignoreEmpty {
		return false
	}

	return v.IsZero()
}

func deepFields(reflectType reflect.Type) []reflect.StructField {
	if reflectType, _ = indirectType(reflectType); reflectType.Kind() == reflect.Struct {
		fields := make([]reflect.StructField, 0, reflectType.NumField())

		for i := 0; i < reflectType.NumField(); i++ {
			v := reflectType.Field(i)
			// PkgPath is the package path that qualifies a lower case (unexported)
			// field name. It is empty for upper case (exported) field names.
			// See https://golang.org/ref/spec#Uniqueness_of_identifiers
			if v.PkgPath == "" {
				fields = append(fields, v)
				if v.Anonymous {
					// also consider fields of anonymous fields as fields of the root
					fields = append(fields, deepFields(v.Type)...)
				}
			}
		}

		return fields
	}

	return nil
}

func indirect(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

func indirectType(reflectType reflect.Type) (_ reflect.Type, isPtr bool) {
	for reflectType.Kind() == reflect.Ptr || reflectType.Kind() == reflect.Slice {
		reflectType = reflectType.Elem()
		isPtr = true
	}
	return reflectType, isPtr
}

func set(to, from reflect.Value, deepCopy bool, converters map[converterPair]TypeConverter) bool {
	if from.IsValid() {
		if ok, err := lookupAndCopyWithConverter(to, from, converters); err != nil {
			return false
		} else if ok {
			return true
		}

		if to.Kind() == reflect.Ptr {
			// set `to` to nil if from is nil
			if from.Kind() == reflect.Ptr && from.IsNil() {
				to.Set(reflect.Zero(to.Type()))
				return true
			} else if to.IsNil() {
				// `from`         -> `to`
				// sql.NullString -> *string
				if fromValuer, ok := driverValuer(from); ok {
					v, err := fromValuer.Value()
					if err != nil {
						return false
					}
					// if `from` is not valid do nothing with `to`
					if v == nil {
						return true
					}
				}
				// allocate new `to` variable with default value (eg. *string -> new(string))
				to.Set(reflect.New(to.Type().Elem()))
			}
			// depointer `to`
			to = to.Elem()
		}

		if deepCopy {
			toKind := to.Kind()
			if toKind == reflect.Interface && to.IsNil() {
				if reflect.TypeOf(from.Interface()) != nil {
					to.Set(reflect.New(reflect.TypeOf(from.Interface())).Elem())
					toKind = reflect.TypeOf(to.Interface()).Kind()
				}
			}
			if from.Kind() == reflect.Ptr && from.IsNil() {
				return true
			}
			if toKind == reflect.Struct || toKind == reflect.Map || toKind == reflect.Slice {
				return false
			}
		}

		if from.Type().ConvertibleTo(to.Type()) {
			to.Set(from.Convert(to.Type()))
		} else if toScanner, ok := to.Addr().Interface().(sql.Scanner); ok {
			// `from`  -> `to`
			// *string -> sql.NullString
			if from.Kind() == reflect.Ptr {
				// if `from` is nil do nothing with `to`
				if from.IsNil() {
					return true
				}
				// depointer `from`
				from = indirect(from)
			}
			// `from` -> `to`
			// string -> sql.NullString
			// set `to` by invoking method Scan(`from`)
			err := toScanner.Scan(from.Interface())
			if err != nil {
				return false
			}
		} else if fromValuer, ok := driverValuer(from); ok {
			// `from`         -> `to`
			// sql.NullString -> string
			v, err := fromValuer.Value()
			if err != nil {
				return false
			}
			// if `from` is not valid do nothing with `to`
			if v == nil {
				return true
			}
			rv := reflect.ValueOf(v)
			if rv.Type().AssignableTo(to.Type()) {
				to.Set(rv)
			}
		} else if from.Kind() == reflect.Ptr {
			return set(to, from.Elem(), deepCopy, converters)
		} else {
			return false
		}
	}

	return true
}

// lookupAndCopyWithConverter looks up the type pair, on success the TypeConverter Fn func is called to copy src to dst field.
func lookupAndCopyWithConverter(to, from reflect.Value, converters map[converterPair]TypeConverter) (copied bool, err error) {
	pair := converterPair{
		SrcType: from.Type(),
		DstType: to.Type(),
	}

	if cnv, ok := converters[pair]; ok {
		result, err := cnv.Fn(from.Interface())

		if err != nil {
			return false, err
		}

		if result != nil {
			to.Set(reflect.ValueOf(result))
		} else {
			// in case we've got a nil value to copy
			to.Set(reflect.Zero(to.Type()))
		}

		return true, nil
	}

	return false, nil
}

// parseTags Parses struct tags and returns uint8 bit flags.
func parseTags(tag string) (flg uint8, name string, err error) {
	for _, t := range strings.Split(tag, ",") {
		switch t {
		case "-":
			flg = tagIgnore
			return
		case "must":
			flg = flg | tagMust
		case "nopanic":
			flg = flg | tagNoPanic
		default:
			if unicode.IsUpper([]rune(t)[0]) {
				name = strings.TrimSpace(t)
			} else {
				err = errors.New("copier field name tag must be start upper case")
			}
		}
	}
	return
}

// getTagFlags Parses struct tags for bit flags, field name.
func getFlags(dest, src reflect.Value, toType, fromType reflect.Type) (flags, error) {
	flgs := flags{
		BitFlags: map[string]uint8{},
		SrcNames: tagNameMapping{
			FieldNameToTag: map[string]string{},
			TagToFieldName: map[string]string{},
		},
		DestNames: tagNameMapping{
			FieldNameToTag: map[string]string{},
			TagToFieldName: map[string]string{},
		},
	}
	var toTypeFields, fromTypeFields []reflect.StructField
	if dest.IsValid() {
		toTypeFields = deepFields(toType)
	}
	if src.IsValid() {
		fromTypeFields = deepFields(fromType)
	}

	// Get a list dest of tags
	for _, field := range toTypeFields {
		tags := field.Tag.Get("copier")
		if tags != "" {
			var name string
			var err error
			if flgs.BitFlags[field.Name], name, err = parseTags(tags); err != nil {
				return flags{}, err
			} else if name != "" {
				flgs.DestNames.FieldNameToTag[field.Name] = name
				flgs.DestNames.TagToFieldName[name] = field.Name
			}
		}
	}

	// Get a list source of tags
	for _, field := range fromTypeFields {
		tags := field.Tag.Get("copier")
		if tags != "" {
			var name string
			var err error
			if _, name, err = parseTags(tags); err != nil {
				return flags{}, err
			} else if name != "" {
				flgs.SrcNames.FieldNameToTag[field.Name] = name
				flgs.SrcNames.TagToFieldName[name] = field.Name
			}
		}
	}
	return flgs, nil
}

// checkBitFlags Checks flags for error or panic conditions.
func checkBitFlags(flagsList map[string]uint8) (err error) {
	// Check flag conditions were met
	for name, flgs := range flagsList {
		if flgs&hasCopied == 0 {
			switch {
			case flgs&tagMust != 0 && flgs&tagNoPanic != 0:
				err = fmt.Errorf("field %s has must tag but was not copied", name)
				return
			case flgs&(tagMust) != 0:
				panic(fmt.Sprintf("Field %s has must tag but was not copied", name))
			}
		}
	}
	return
}

func getFieldName(fieldName string, flgs flags) (srcFieldName string, destFieldName string) {
	// get dest field name
	if srcTagName, ok := flgs.SrcNames.FieldNameToTag[fieldName]; ok {
		destFieldName = srcTagName
		if destTagName, ok := flgs.DestNames.TagToFieldName[srcTagName]; ok {
			destFieldName = destTagName
		}
	} else {
		if destTagName, ok := flgs.DestNames.TagToFieldName[fieldName]; ok {
			destFieldName = destTagName
		}
	}
	if destFieldName == "" {
		destFieldName = fieldName
	}

	// get source field name
	if destTagName, ok := flgs.DestNames.FieldNameToTag[fieldName]; ok {
		srcFieldName = destTagName
		if srcField, ok := flgs.SrcNames.TagToFieldName[destTagName]; ok {
			srcFieldName = srcField
		}
	} else {
		if srcField, ok := flgs.SrcNames.TagToFieldName[fieldName]; ok {
			srcFieldName = srcField
		}
	}

	if srcFieldName == "" {
		srcFieldName = fieldName
	}
	return
}

func driverValuer(v reflect.Value) (i driver.Valuer, ok bool) {

	if !v.CanAddr() {
		i, ok = v.Interface().(driver.Valuer)
		return
	}

	i, ok = v.Addr().Interface().(driver.Valuer)
	return
}
