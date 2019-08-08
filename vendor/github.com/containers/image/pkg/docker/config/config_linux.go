package config

import (
	"fmt"
	"strings"

	"github.com/containers/image/pkg/keyctl"
	"github.com/pkg/errors"
)

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
	return fmt.Sprintf("container-registry-login:%s", registry)
}
