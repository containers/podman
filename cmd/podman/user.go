package main

// #include <sys/types.h>
// #include <grp.h>
// #include <pwd.h>
// #include <stdlib.h>
// #include <stdio.h>
// #include <string.h>
// typedef FILE * pFILE;
import "C"

import (
	"fmt"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

func fopenContainerFile(rootdir, filename string) (C.pFILE, error) {
	var st, lst syscall.Stat_t

	ctrfile := filepath.Join(rootdir, filename)
	cctrfile := C.CString(ctrfile)
	defer C.free(unsafe.Pointer(cctrfile))
	mode := C.CString("r")
	defer C.free(unsafe.Pointer(mode))
	f, err := C.fopen(cctrfile, mode)
	if f == nil || err != nil {
		return nil, errors.Wrapf(err, "error opening %q", ctrfile)
	}
	if err = syscall.Fstat(int(C.fileno(f)), &st); err != nil {
		return nil, errors.Wrapf(err, "fstat(%q)", ctrfile)
	}
	if err = syscall.Lstat(ctrfile, &lst); err != nil {
		return nil, errors.Wrapf(err, "lstat(%q)", ctrfile)
	}
	if st.Dev != lst.Dev || st.Ino != lst.Ino {
		return nil, errors.Errorf("%q is not a regular file", ctrfile)
	}
	return f, nil
}

var (
	lookupUser, lookupGroup sync.Mutex
)

func lookupUserInContainer(rootdir, username string) (uint64, uint64, error) {
	name := C.CString(username)
	defer C.free(unsafe.Pointer(name))

	f, err := fopenContainerFile(rootdir, "/etc/passwd")
	if err != nil {
		return 0, 0, err
	}
	defer C.fclose(f)

	lookupUser.Lock()
	defer lookupUser.Unlock()

	pwd := C.fgetpwent(f)
	for pwd != nil {
		if C.strcmp(pwd.pw_name, name) != 0 {
			pwd = C.fgetpwent(f)
			continue
		}
		return uint64(pwd.pw_uid), uint64(pwd.pw_gid), nil
	}

	return 0, 0, user.UnknownUserError(fmt.Sprintf("error looking up user %q", username))
}

func lookupGroupForUIDInContainer(rootdir string, userid uint64) (string, uint64, error) {
	f, err := fopenContainerFile(rootdir, "/etc/passwd")
	if err != nil {
		return "", 0, err
	}
	defer C.fclose(f)

	lookupUser.Lock()
	defer lookupUser.Unlock()

	pwd := C.fgetpwent(f)
	for pwd != nil {
		if uint64(pwd.pw_uid) != userid {
			pwd = C.fgetpwent(f)
			continue
		}
		return C.GoString(pwd.pw_name), uint64(pwd.pw_gid), nil
	}

	return "", 0, user.UnknownUserError(fmt.Sprintf("error looking up user with UID %d", userid))
}

func lookupGroupInContainer(rootdir, groupname string) (uint64, error) {
	name := C.CString(groupname)
	defer C.free(unsafe.Pointer(name))

	f, err := fopenContainerFile(rootdir, "/etc/group")
	if err != nil {
		return 0, err
	}
	defer C.fclose(f)

	lookupGroup.Lock()
	defer lookupGroup.Unlock()

	grp := C.fgetgrent(f)
	for grp != nil {
		if C.strcmp(grp.gr_name, name) != 0 {
			grp = C.fgetgrent(f)
			continue
		}
		return uint64(grp.gr_gid), nil
	}

	return 0, user.UnknownGroupError(fmt.Sprintf("error looking up group %q", groupname))
}
