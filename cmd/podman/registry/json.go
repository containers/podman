package registry

import (
	"sync"

	"github.com/bytedance/sonic"
)

var (
	json     sonic.API
	jsonSync sync.Once
)

// JSONLibrary provides a "encoding/json" compatible API
func JSONLibrary() sonic.API {
	jsonSync.Do(func() {
		json = sonic.ConfigStd
	})
	return json
}
