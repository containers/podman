package utils

import (
	"net/url"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// DeepCopy does a deep copy of a structure
// Error checking of parameters delegated to json engine
var DeepCopy = func(dst interface{}, src interface{}) error {
	payload, err := json.Marshal(src)
	if err != nil {
		return err
	}

	err = json.Unmarshal(payload, dst)
	if err != nil {
		return err
	}
	return nil
}

func ToLibpodFilters(f url.Values) (filters []string) {
	for k, v := range f {
		filters = append(filters, k+"="+v[0])
	}
	return
}

func ToURLValues(f []string) (filters url.Values) {
	filters = make(url.Values)
	for _, v := range f {
		t := strings.SplitN(v, "=", 2)
		filters.Add(t[0], t[1])
	}
	return
}
