package common

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/ssh"
	"github.com/containers/image/v5/pkg/cli"
	"github.com/containers/image/v5/pkg/cli/sigstore"
	"github.com/containers/image/v5/signature/signer"
	"github.com/containers/podman/v4/pkg/domain/entities"
)

// PrepareSigning updates pushOpts.Signers, pushOpts.SignPassphrase and SignSigstorePrivateKeyPassphrase based on a --sign-passphrase-file
// value signPassphraseFile and a --sign-by-sigsstore value signBySigstoreParamFile, and validates pushOpts.Sign* consistency.
// It may interactively prompt for a passphrase if one is required and wasn’t provided otherwise;
// or it may interactively trigger an OIDC authentication, using standard input/output, or even open a web browser.
// Returns a cleanup callback on success, which must be called when done.
func PrepareSigning(pushOpts *entities.ImagePushOptions,
	signPassphraseFile, signBySigstoreParamFile string) (func(), error) {
	// c/common/libimage.Image does allow creating both simple signing and sigstore signatures simultaneously,
	// with independent passphrases, but that would make the CLI probably too confusing.
	// For now, use the passphrase with either, but only one of them.
	if signPassphraseFile != "" && pushOpts.SignBy != "" && pushOpts.SignBySigstorePrivateKeyFile != "" {
		return nil, fmt.Errorf("only one of --sign-by and sign-by-sigstore-private-key can be used with --sign-passphrase-file")
	}

	var passphrase string
	if signPassphraseFile != "" {
		p, err := cli.ReadPassphraseFile(signPassphraseFile)
		if err != nil {
			return nil, err
		}
		passphrase = p
	} else if pushOpts.SignBySigstorePrivateKeyFile != "" {
		p := ssh.ReadPassphrase()
		passphrase = string(p)
	} // pushOpts.SignBy triggers a GPG-agent passphrase prompt, possibly using a more secure channel, so we usually shouldn’t prompt ourselves if no passphrase was explicitly provided.
	pushOpts.SignPassphrase = passphrase
	pushOpts.SignSigstorePrivateKeyPassphrase = []byte(passphrase)
	cleanup := signingCleanup{}
	if signBySigstoreParamFile != "" {
		signer, err := sigstore.NewSignerFromParameterFile(signBySigstoreParamFile, &sigstore.Options{
			PrivateKeyPassphrasePrompt: cli.ReadPassphraseFile,
			Stdin:                      os.Stdin,
			Stdout:                     os.Stdout,
		})
		if err != nil {
			return nil, err
		}
		pushOpts.Signers = append(pushOpts.Signers, signer)
		cleanup.signers = append(cleanup.signers, signer)
	}
	return cleanup.cleanup, nil
}

// signingCleanup carries state for cleanup after PrepareSigning
type signingCleanup struct {
	signers []*signer.Signer
}

func (c *signingCleanup) cleanup() {
	for _, s := range c.signers {
		s.Close()
	}
}
