package containers

import (
	"net/url"
	"reflect"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"
)

/*
This file is generated automatically by go generate.  Do not edit.

Created 2020-12-18 13:33:18.420656951 -0600 CST m=+0.000259662
*/

// Changed
func (o *CommitOptions) Changed(fieldName string) bool {
	r := reflect.ValueOf(o)
	value := reflect.Indirect(r).FieldByName(fieldName)
	return !value.IsNil()
}

// ToParams
func (o *CommitOptions) ToParams() (url.Values, error) {
	params := url.Values{}
	if o == nil {
		return params, nil
	}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	s := reflect.ValueOf(o)
	if reflect.Ptr == s.Kind() {
		s = s.Elem()
	}
	sType := s.Type()
	for i := 0; i < s.NumField(); i++ {
		fieldName := sType.Field(i).Name
		if !o.Changed(fieldName) {
			continue
		}
		f := s.Field(i)
		if reflect.Ptr == f.Kind() {
			f = f.Elem()
		}
		switch f.Kind() {
		case reflect.Bool:
			params.Set(fieldName, strconv.FormatBool(f.Bool()))
		case reflect.String:
			params.Set(fieldName, f.String())
		case reflect.Int, reflect.Int64:
			// f.Int() is always an int64
			params.Set(fieldName, strconv.FormatInt(f.Int(), 10))
		case reflect.Uint, reflect.Uint64:
			// f.Uint() is always an uint64
			params.Set(fieldName, strconv.FormatUint(f.Uint(), 10))
		case reflect.Slice:
			typ := reflect.TypeOf(f.Interface()).Elem()
			switch typ.Kind() {
			case reflect.String:
				sl := f.Slice(0, f.Len())
				s, ok := sl.Interface().([]string)
				if !ok {
					return nil, errors.New("failed to convert to string slice")
				}
				for _, val := range s {
					params.Add(fieldName, val)
				}
			default:
				return nil, errors.Errorf("unknown slice type %s", f.Kind().String())
			}
		case reflect.Map:
			lowerCaseKeys := make(map[string][]string)
			iter := f.MapRange()
			for iter.Next() {
				lowerCaseKeys[iter.Key().Interface().(string)] = iter.Value().Interface().([]string)

			}
			s, err := json.MarshalToString(lowerCaseKeys)
			if err != nil {
				return nil, err
			}

			params.Set(fieldName, s)
		}
	}
	return params, nil
}

// WithAuthor
func (o *CommitOptions) WithAuthor(value string) *CommitOptions {
	v := &value
	o.Author = v
	return o
}

// GetAuthor
func (o *CommitOptions) GetAuthor() string {
	var author string
	if o.Author == nil {
		return author
	}
	return *o.Author
}

// WithChanges
func (o *CommitOptions) WithChanges(value []string) *CommitOptions {
	v := value
	o.Changes = v
	return o
}

// GetChanges
func (o *CommitOptions) GetChanges() []string {
	var changes []string
	if o.Changes == nil {
		return changes
	}
	return o.Changes
}

// WithComment
func (o *CommitOptions) WithComment(value string) *CommitOptions {
	v := &value
	o.Comment = v
	return o
}

// GetComment
func (o *CommitOptions) GetComment() string {
	var comment string
	if o.Comment == nil {
		return comment
	}
	return *o.Comment
}

// WithFormat
func (o *CommitOptions) WithFormat(value string) *CommitOptions {
	v := &value
	o.Format = v
	return o
}

// GetFormat
func (o *CommitOptions) GetFormat() string {
	var format string
	if o.Format == nil {
		return format
	}
	return *o.Format
}

// WithPause
func (o *CommitOptions) WithPause(value bool) *CommitOptions {
	v := &value
	o.Pause = v
	return o
}

// GetPause
func (o *CommitOptions) GetPause() bool {
	var pause bool
	if o.Pause == nil {
		return pause
	}
	return *o.Pause
}

// WithRepo
func (o *CommitOptions) WithRepo(value string) *CommitOptions {
	v := &value
	o.Repo = v
	return o
}

// GetRepo
func (o *CommitOptions) GetRepo() string {
	var repo string
	if o.Repo == nil {
		return repo
	}
	return *o.Repo
}

// WithTag
func (o *CommitOptions) WithTag(value string) *CommitOptions {
	v := &value
	o.Tag = v
	return o
}

// GetTag
func (o *CommitOptions) GetTag() string {
	var tag string
	if o.Tag == nil {
		return tag
	}
	return *o.Tag
}
