package utils

import "fmt"

type OutputErrors []error

func (o OutputErrors) PrintErrors() (lastError error) {
	if len(o) == 0 {
		return
	}
	lastError = o[len(o)-1]
	for e := 0; e < len(o)-1; e++ {
		fmt.Println(o[e])
	}
	return
}
