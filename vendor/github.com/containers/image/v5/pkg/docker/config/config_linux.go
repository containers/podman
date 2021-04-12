package config

import (
	"fmt"
	"strings"

	"github.com/containers/image/v5/internal/pkg/keyctl"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// NOTE: none of the functions here are currently used.  If we ever want to
// reenable keyring support, we should introduce a similar built-in credential
// helpers as for `sysregistriesv2.AuthenticationFileHelper`.

const keyDescribePrefix = "container-registry-login:" // nolint

func getAuthFromKernelKeyring(registry string) (string, string, error) { // nolint
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

func deleteAuthFromKernelKeyring(registry string) error { // nolint
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

func removeAllAuthFromKernelKeyring() error { // nolint
	keys, err := keyctl.ReadUserKeyring()
	if err != nil {
		return err
	}

	userkeyring, err := keyctl.UserKeyring()
	if err != nil {
		return err
	}

	for _, k := range keys {
		keyAttr, err := k.Describe()
		if err != nil {
			return err
		}
		// split string "type;uid;gid;perm;description"
		keyAttrs := strings.SplitN(keyAttr, ";", 5)
		if len(keyAttrs) < 5 {
			return errors.Errorf("Key attributes of %d are not available", k.ID())
		}
		keyDescribe := keyAttrs[4]
		if strings.HasPrefix(keyDescribe, keyDescribePrefix) {
			err := keyctl.Unlink(userkeyring, k)
			if err != nil {
				return errors.Wrapf(err, "error unlinking key %d", k.ID())
			}
			logrus.Debugf("unlinked key %d:%s", k.ID(), keyAttr)
		}
	}
	return nil
}

func setAuthToKernelKeyring(registry, username, password string) error { // nolint
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

func genDescription(registry string) string { // nolint
	return fmt.Sprintf("%s%s", keyDescribePrefix, registry)
}
