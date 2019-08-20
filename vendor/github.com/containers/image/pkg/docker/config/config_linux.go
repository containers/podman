package config

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/containers/image/pkg/keyctl"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const keyDescribePrefix = "container-registry-login:"

func getAuthFromKernelKeyring(registry string) (string, string, error) {
	userkeyring, err := keyctl.UserKeyring()
	if err != nil {
		return "", "", err
	}
	key, err := userkeyring.Search(genDescription(registry))
	if err != nil {
		return "", "", err
	}
	authData, err := key.Get()
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(authData), "\x00", 2)
	if len(parts) != 2 {
		return "", "", nil
	}
	return parts[0], parts[1], nil
}

func deleteAuthFromKernelKeyring(registry string) error {
	userkeyring, err := keyctl.UserKeyring()

	if err != nil {
		return err
	}
	key, err := userkeyring.Search(genDescription(registry))
	if err != nil {
		return err
	}
	return key.Unlink()
}

func removeAllAuthFromKernelKeyring() error {
	keyIDs, err := readUserKeyring()
	if err != nil {
		return err
	}

	for _, kID := range keyIDs {
		keyAttr, err := unix.KeyctlString(unix.KEYCTL_DESCRIBE, int(kID))
		if err != nil {
			return err
		}
		// split string "type;uid;gid;perm;description"
		keyAttrs := strings.SplitN(keyAttr, ";", 5)
		if len(keyAttrs) < 5 {
			return errors.Errorf("Key attributes of %d are not avaliable", kID)
		}
		keyDescribe := keyAttrs[4]
		if strings.HasPrefix(keyDescribe, keyDescribePrefix) {
			_, err := unix.KeyctlInt(unix.KEYCTL_UNLINK, int(kID), int(unix.KEY_SPEC_USER_KEYRING), 0, 0)
			if err != nil {
				return errors.Wrapf(err, "error unlinking key %d", kID)
			}
			logrus.Debugf("unlink key %d:%s", kID, keyAttr)
		}
	}
	return nil
}

func setAuthToKernelKeyring(registry, username, password string) error {
	keyring, err := keyctl.SessionKeyring()
	if err != nil {
		return err
	}
	id, err := keyring.Add(genDescription(registry), []byte(fmt.Sprintf("%s\x00%s", username, password)))
	if err != nil {
		return err
	}

	// sets all permission(view,read,write,search,link,set attribute) for current user
	// it enables the user to search the key after it linked to user keyring and unlinked from session keyring
	err = keyctl.SetPerm(id, keyctl.PermUserAll)
	if err != nil {
		return err
	}
	// link the key to userKeyring
	userKeyring, err := keyctl.UserKeyring()
	if err != nil {
		return errors.Wrapf(err, "error getting user keyring")
	}
	err = keyctl.Link(userKeyring, id)
	if err != nil {
		return errors.Wrapf(err, "error linking the key to user keyring")
	}
	// unlink the key from session keyring
	err = keyctl.Unlink(keyring, id)
	if err != nil {
		return errors.Wrapf(err, "error unlinking the key from session keyring")
	}
	return nil
}

func genDescription(registry string) string {
	return fmt.Sprintf("%s%s", keyDescribePrefix, registry)
}

// readUserKeyring reads user keyring and returns slice of key id(key_serial_t) representing the IDs of all the keys that are linked to it
func readUserKeyring() ([]int32, error) {
	var (
		b        []byte
		err      error
		sizeRead int
	)

	krSize := 4
	size := krSize
	b = make([]byte, size)
	sizeRead = size + 1
	for sizeRead > size {
		r1, err := unix.KeyctlBuffer(unix.KEYCTL_READ, unix.KEY_SPEC_USER_KEYRING, b, size)
		if err != nil {
			return nil, err
		}

		if sizeRead = int(r1); sizeRead > size {
			b = make([]byte, sizeRead)
			size = sizeRead
			sizeRead = size + 1
		} else {
			krSize = sizeRead
		}
	}

	keyIDs := getKeyIDsFromByte(b[:krSize])
	return keyIDs, err
}

func getKeyIDsFromByte(byteKeyIDs []byte) []int32 {
	idSize := 4
	var keyIDs []int32
	for idx := 0; idx+idSize <= len(byteKeyIDs); idx = idx + idSize {
		tempID := *(*int32)(unsafe.Pointer(&byteKeyIDs[idx]))
		keyIDs = append(keyIDs, tempID)
	}
	return keyIDs
}
