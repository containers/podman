package params

import (
	"reflect"
)

type ImagesListOptions struct {
	All     *bool
	Digests *bool
	Ids     []string
	Filters map[string][]string
}

func (o *ImagesListOptions) WithAll(value bool) *ImagesListOptions {
	v := &value
	o.All = v
	return o
}

func (o *ImagesListOptions) WithDigests(value bool) *ImagesListOptions {
	v := &value
	o.Digests = v
	return o
}

func (o *ImagesListOptions) WithFilters(value map[string][]string) *ImagesListOptions {
	o.Filters = value
	return o
}

func (o *ImagesListOptions) ToString(field string) string {
	return toString(o, field)
}

func (o *ImagesListOptions) Changed(field string) bool {
	return !reflect.ValueOf(o).Elem().FieldByName(field).IsNil()
}
