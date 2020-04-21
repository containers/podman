package registry

import (
	"sync"

	jsoniter "github.com/json-iterator/go"
)

var (
	json     jsoniter.API
	jsonSync sync.Once
)

// JsonLibrary provides a "encoding/json" compatible API
func JsonLibrary() jsoniter.API {
	jsonSync.Do(func() {
		json = jsoniter.ConfigCompatibleWithStandardLibrary
	})
	return json
}
