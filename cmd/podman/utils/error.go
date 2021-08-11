package utils

import (
	"fmt"
	"os"
)

type OutputErrors []error

func (o OutputErrors) PrintErrors() (lastError error) {
	if len(o) == 0 {
		return
	}
	lastError = o[len(o)-1]
	for e := 0; e < len(o)-1; e++ {
		fmt.Fprintf(os.Stderr, "Error: %s\n", o[e])
	}
	return
}
