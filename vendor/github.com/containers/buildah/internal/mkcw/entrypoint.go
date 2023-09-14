package mkcw

import _ "embed"

//go:embed "embed/entrypoint.gz"
var entrypointCompressedBytes []byte
