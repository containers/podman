//+build linux,cgo

package libpod

//#include <sys/un.h>
// extern int unix_path_length(){struct sockaddr_un addr; return sizeof(addr.sun_path) - 1;}
import "C"

func unixPathLength() int {
	return int(C.unix_path_length())
}
