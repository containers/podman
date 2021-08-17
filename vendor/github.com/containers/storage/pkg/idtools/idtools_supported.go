// +build linux,cgo,libsubid

package idtools

import (
	"unsafe"

	"github.com/pkg/errors"
)

/*
#cgo LDFLAGS: -l subid
#include <shadow/subid.h>
#include <stdlib.h>
const char *Prog = "storage";
struct subid_range get_range(struct subid_range *ranges, int i)
{
    return ranges[i];
}
*/
import "C"

func readSubid(username string, isUser bool) (ranges, error) {
	var ret ranges
	if username == "ALL" {
		return nil, errors.New("username ALL not supported")
	}

	cUsername := C.CString(username)
	defer C.free(unsafe.Pointer(cUsername))

	var nRanges C.int
	var cRanges *C.struct_subid_range
	if isUser {
		nRanges = C.get_subuid_ranges(cUsername, &cRanges)
	} else {
		nRanges = C.get_subgid_ranges(cUsername, &cRanges)
	}
	if nRanges < 0 {
		return nil, errors.New("cannot read subids")
	}
	defer C.free(unsafe.Pointer(cRanges))

	for i := 0; i < int(nRanges); i++ {
		r := C.get_range(cRanges, C.int(i))
		newRange := subIDRange{
			Start:  int(r.start),
			Length: int(r.count),
		}
		ret = append(ret, newRange)
	}
	return ret, nil
}

func readSubuid(username string) (ranges, error) {
	return readSubid(username, true)
}

func readSubgid(username string) (ranges, error) {
	return readSubid(username, false)
}
