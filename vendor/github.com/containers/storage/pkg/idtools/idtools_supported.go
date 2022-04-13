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
#include <stdio.h>
const char *Prog = "storage";
FILE *shadow_logfd = NULL;

struct subid_range get_range(struct subid_range *ranges, int i)
{
	shadow_logfd = stderr;
	return ranges[i];
}

#if !defined(SUBID_ABI_MAJOR) || (SUBID_ABI_MAJOR < 4)
# define subid_get_uid_ranges get_subuid_ranges
# define subid_get_gid_ranges get_subgid_ranges
#endif

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
		nRanges = C.subid_get_uid_ranges(cUsername, &cRanges)
	} else {
		nRanges = C.subid_get_gid_ranges(cUsername, &cRanges)
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
