package types

import (
	"errors"
)

var (
	ErrArtifactUnamed        = errors.New("artifact is unnamed")
	ErrArtifactNotExist      = errors.New("artifact does not exist")
	ErrArtifactAlreadyExists = errors.New("artifact already exists")
	ErrArtifactFileExists    = errors.New("file already exists in artifact")
)
