package common

import (
	"fmt"

	"github.com/containers/common/pkg/ssh"
	"github.com/containers/image/v5/pkg/cli"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

// PrepareSigningPassphrase updates pushOpts.SignPassphrase and SignSigstorePrivateKeyPassphrase based on a --sign-passphrase-file value signPassphraseFile,
// and validates pushOpts.Sign* consistency.
// It may interactively prompt for a passphrase if one is required and wasn’t provided otherwise.
func PrepareSigningPassphrase(pushOpts *entities.ImagePushOptions, signPassphraseFile string) error {
	// c/common/libimage.Image does allow creating both simple signing and sigstore signatures simultaneously,
	// with independent passphrases, but that would make the CLI probably too confusing.
	// For now, use the passphrase with either, but only one of them.
	if signPassphraseFile != "" && pushOpts.SignBy != "" && pushOpts.SignBySigstorePrivateKeyFile != "" {
		return fmt.Errorf("only one of --sign-by and sign-by-sigstore-private-key can be used with --sign-passphrase-file")
	}

	var passphrase string
	if signPassphraseFile != "" {
		p, err := cli.ReadPassphraseFile(signPassphraseFile)
		if err != nil {
			return err
		}
		passphrase = p
	} else if pushOpts.SignBySigstorePrivateKeyFile != "" {
		p := ssh.ReadPassphrase()
		passphrase = string(p)
	} // pushOpts.SignBy triggers a GPG-agent passphrase prompt, possibly using a more secure channel, so we usually shouldn’t prompt ourselves if no passphrase was explicitly provided.
	pushOpts.SignPassphrase = passphrase
	pushOpts.SignSigstorePrivateKeyPassphrase = []byte(passphrase)
	return nil
}
