package osxkeychain

/*
#cgo CFLAGS: -x objective-c -mmacosx-version-min=10.10
#cgo LDFLAGS: -framework Security -framework Foundation -mmacosx-version-min=10.10

#include "osxkeychain_darwin.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unsafe"

	"github.com/docker/docker-credential-helpers/credentials"
)

// errCredentialsNotFound is the specific error message returned by OS X
// when the credentials are not in the keychain.
const errCredentialsNotFound = "The specified item could not be found in the keychain."

// Osxkeychain handles secrets using the OS X Keychain as store.
type Osxkeychain struct{}

// Add adds new credentials to the keychain.
func (h Osxkeychain) Add(creds *credentials.Credentials) error {
	h.Delete(creds.ServerURL)

	s, err := splitServer(creds.ServerURL)
	if err != nil {
		return err
	}
	defer freeServer(s)

	label := C.CString(credentials.CredsLabel)
	defer C.free(unsafe.Pointer(label))
	username := C.CString(creds.Username)
	defer C.free(unsafe.Pointer(username))
	secret := C.CString(creds.Secret)
	defer C.free(unsafe.Pointer(secret))

	errMsg := C.keychain_add(s, label, username, secret)
	if errMsg != nil {
		defer C.free(unsafe.Pointer(errMsg))
		return errors.New(C.GoString(errMsg))
	}

	return nil
}

// Delete removes credentials from the keychain.
func (h Osxkeychain) Delete(serverURL string) error {
	s, err := splitServer(serverURL)
	if err != nil {
		return err
	}
	defer freeServer(s)

	errMsg := C.keychain_delete(s)
	if errMsg != nil {
		defer C.free(unsafe.Pointer(errMsg))
		return errors.New(C.GoString(errMsg))
	}

	return nil
}

// Get returns the username and secret to use for a given registry server URL.
func (h Osxkeychain) Get(serverURL string) (string, string, error) {
	s, err := splitServer(serverURL)
	if err != nil {
		return "", "", err
	}
	defer freeServer(s)

	var usernameLen C.uint
	var username *C.char
	var secretLen C.uint
	var secret *C.char
	defer C.free(unsafe.Pointer(username))
	defer C.free(unsafe.Pointer(secret))

	errMsg := C.keychain_get(s, &usernameLen, &username, &secretLen, &secret)
	if errMsg != nil {
		defer C.free(unsafe.Pointer(errMsg))
		goMsg := C.GoString(errMsg)
		if goMsg == errCredentialsNotFound {
			return "", "", credentials.NewErrCredentialsNotFound()
		}

		return "", "", errors.New(goMsg)
	}

	user := C.GoStringN(username, C.int(usernameLen))
	pass := C.GoStringN(secret, C.int(secretLen))
	return user, pass, nil
}

// List returns the stored URLs and corresponding usernames.
func (h Osxkeychain) List() (map[string]string, error) {
	credsLabelC := C.CString(credentials.CredsLabel)
	defer C.free(unsafe.Pointer(credsLabelC))

	var pathsC **C.char
	defer C.free(unsafe.Pointer(pathsC))
	var acctsC **C.char
	defer C.free(unsafe.Pointer(acctsC))
	var listLenC C.uint
	errMsg := C.keychain_list(credsLabelC, &pathsC, &acctsC, &listLenC)
	if errMsg != nil {
		defer C.free(unsafe.Pointer(errMsg))
		goMsg := C.GoString(errMsg)
		return nil, errors.New(goMsg)
	}

	defer C.freeListData(&pathsC, listLenC)
	defer C.freeListData(&acctsC, listLenC)

	var listLen int
	listLen = int(listLenC)
	pathTmp := (*[1 << 30]*C.char)(unsafe.Pointer(pathsC))[:listLen:listLen]
	acctTmp := (*[1 << 30]*C.char)(unsafe.Pointer(acctsC))[:listLen:listLen]
	//taking the array of c strings into go while ignoring all the stuff irrelevant to credentials-helper
	resp := make(map[string]string)
	for i := 0; i < listLen; i++ {
		if C.GoString(pathTmp[i]) == "0" {
			continue
		}
		resp[C.GoString(pathTmp[i])] = C.GoString(acctTmp[i])
	}
	return resp, nil
}

func splitServer(serverURL string) (*C.struct_Server, error) {
	u, err := parseURL(serverURL)
	if err != nil {
		return nil, err
	}

	proto := C.kSecProtocolTypeHTTPS
	if u.Scheme == "http" {
		proto = C.kSecProtocolTypeHTTP
	}
	var port int
	p := getPort(u)
	if p != "" {
		port, err = strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
	}

	return &C.struct_Server{
		proto: C.SecProtocolType(proto),
		host:  C.CString(getHostname(u)),
		port:  C.uint(port),
		path:  C.CString(u.Path),
	}, nil
}

func freeServer(s *C.struct_Server) {
	C.free(unsafe.Pointer(s.host))
	C.free(unsafe.Pointer(s.path))
}

// parseURL parses and validates a given serverURL to an url.URL, and
// returns an error if validation failed. Querystring parameters are
// omitted in the resulting URL, because they are not used in the helper.
//
// If serverURL does not have a valid scheme, `//` is used as scheme
// before parsing. This prevents the hostname being used as path,
// and the credentials being stored without host.
func parseURL(serverURL string) (*url.URL, error) {
	// Check if serverURL has a scheme, otherwise add `//` as scheme.
	if !strings.Contains(serverURL, "://") && !strings.HasPrefix(serverURL, "//") {
		serverURL = "//" + serverURL
	}

	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "" && u.Scheme != "https" && u.Scheme != "http" {
		return nil, errors.New("unsupported scheme: " + u.Scheme)
	}
	if getHostname(u) == "" {
		return nil, errors.New("no hostname in URL")
	}

	u.RawQuery = ""
	return u, nil
}
