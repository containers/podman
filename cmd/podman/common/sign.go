package common

import (
	"fmt"
	"os"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/common/pkg/ssh"
	"github.com/containers/image/v5/pkg/cli"
	"github.com/containers/image/v5/pkg/cli/sigstore"
	"github.com/containers/image/v5/signature/signer"
	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/spf13/cobra"
)

// SigningCLIOnlyOptions contains signing-related CLI options.
// Some other options are defined in entities.ImagePushOptions.
type SigningCLIOnlyOptions struct {
	signPassphraseFile      string
	signBySigstoreParamFile string
}

func DefineSigningFlags(cmd *cobra.Command, cliOpts *SigningCLIOnlyOptions, pushOpts *entities.ImagePushOptions) {
	flags := cmd.Flags()

	signByFlagName := "sign-by"
	flags.StringVar(&pushOpts.SignBy, signByFlagName, "", "Add a signature at the destination using the specified key")
	_ = cmd.RegisterFlagCompletionFunc(signByFlagName, completion.AutocompleteNone)

	signBySigstoreFlagName := "sign-by-sigstore"
	flags.StringVar(&cliOpts.signBySigstoreParamFile, signBySigstoreFlagName, "", "Sign the image using a sigstore parameter file at `PATH`")
	_ = cmd.RegisterFlagCompletionFunc(signBySigstoreFlagName, completion.AutocompleteDefault)

	signBySigstorePrivateKeyFlagName := "sign-by-sigstore-private-key"
	flags.StringVar(&pushOpts.SignBySigstorePrivateKeyFile, signBySigstorePrivateKeyFlagName, "", "Sign the image using a sigstore private key at `PATH`")
	_ = cmd.RegisterFlagCompletionFunc(signBySigstorePrivateKeyFlagName, completion.AutocompleteDefault)

	signPassphraseFileFlagName := "sign-passphrase-file"
	flags.StringVar(&cliOpts.signPassphraseFile, signPassphraseFileFlagName, "", "Read a passphrase for signing an image from `PATH`")
	_ = cmd.RegisterFlagCompletionFunc(signPassphraseFileFlagName, completion.AutocompleteDefault)

	if registry.IsRemote() {
		_ = flags.MarkHidden(signByFlagName)
		_ = flags.MarkHidden(signBySigstoreFlagName)
		_ = flags.MarkHidden(signBySigstorePrivateKeyFlagName)
		_ = flags.MarkHidden(signPassphraseFileFlagName)
	}
}

// PrepareSigning updates pushOpts.Signers, pushOpts.SignPassphrase and SignSigstorePrivateKeyPassphrase based on cliOpts,
// and validates pushOpts.Sign* consistency.
// It may interactively prompt for a passphrase if one is required and wasn’t provided otherwise;
// or it may interactively trigger an OIDC authentication, using standard input/output, or even open a web browser.
// Returns a cleanup callback on success, which must be called when done.
func PrepareSigning(pushOpts *entities.ImagePushOptions, cliOpts *SigningCLIOnlyOptions) (func(), error) {
	// c/common/libimage.Image does allow creating both simple signing and sigstore signatures simultaneously,
	// with independent passphrases, but that would make the CLI probably too confusing.
	// For now, use the passphrase with either, but only one of them.
	if cliOpts.signPassphraseFile != "" && pushOpts.SignBy != "" && pushOpts.SignBySigstorePrivateKeyFile != "" {
		return nil, fmt.Errorf("only one of --sign-by and sign-by-sigstore-private-key can be used with --sign-passphrase-file")
	}

	var passphrase string
	if cliOpts.signPassphraseFile != "" {
		p, err := cli.ReadPassphraseFile(cliOpts.signPassphraseFile)
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
	if cliOpts.signBySigstoreParamFile != "" {
		signer, err := sigstore.NewSignerFromParameterFile(cliOpts.signBySigstoreParamFile, &sigstore.Options{
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
