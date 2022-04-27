package libpod

import (
	jsoniter "github.com/json-iterator/go"
)

// pull frozen jsoniter into package
var json = jsoniter.ConfigCompatibleWithStandardLibrary
