package storage

// NOTE: this is a hack to trick go modules into vendoring the below
//       dependencies.  Those are required during ffjson generation
//       but do NOT end up in the final file.

import (
	_ "github.com/pquerna/ffjson/inception" // nolint:typecheck
	_ "github.com/pquerna/ffjson/shared"    // nolint:typecheck
)
