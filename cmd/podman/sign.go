package main

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containers/image/signature"
	"github.com/containers/image/transports"
	"github.com/containers/image/transports/alltransports"
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/trust"
	"github.com/containers/libpod/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	signCommand     cliconfig.SignValues
	signDescription = "Create a signature file that can be used later to verify the image."
	_signCommand    = &cobra.Command{
		Use:   "sign [flags] IMAGE [IMAGE...]",
		Short: "Sign an image",
		Long:  signDescription,
		RunE: func(cmd *cobra.Command, args []string) error {
			signCommand.InputArgs = args
			signCommand.GlobalFlags = MainGlobalOpts
			signCommand.Remote = remoteclient
			return signCmd(&signCommand)
		},
		Example: `podman sign --sign-by mykey imageID
  podman sign --sign-by mykey --directory ./mykeydir imageID`,
	}
)

func init() {
	signCommand.Command = _signCommand
	signCommand.SetHelpTemplate(HelpTemplate())
	signCommand.SetUsageTemplate(UsageTemplate())
	flags := signCommand.Flags()
	flags.StringVarP(&signCommand.Directory, "directory", "d", "", "Define an alternate directory to store signatures")
	flags.StringVar(&signCommand.SignBy, "sign-by", "", "Name of the signing key")
	flags.StringVar(&signCommand.CertDir, "cert-dir", "", "`Pathname` of a directory containing TLS certificates and keys")
}

// SignatureStoreDir defines default directory to store signatures
const SignatureStoreDir = "/var/lib/containers/sigstore"

func signCmd(c *cliconfig.SignValues) error {
	args := c.InputArgs
	if len(args) < 1 {
		return errors.Errorf("at least one image name must be specified")
	}
	runtime, err := libpodruntime.GetRuntime(getContext(), &c.PodmanCommand)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.DeferredShutdown(false)

	signby := c.SignBy
	if signby == "" {
		return errors.Errorf("please provide an identity")
	}

	var sigStoreDir string
	if c.Flag("directory").Changed {
		sigStoreDir = c.Directory
		if _, err := os.Stat(sigStoreDir); err != nil {
			return errors.Wrapf(err, "invalid directory %s", sigStoreDir)
		}
	}

	sc := runtime.SystemContext()
	sc.DockerCertPath = c.CertDir

	dockerRegistryOptions := image.DockerRegistryOptions{
		DockerCertPath: c.CertDir,
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return errors.Wrap(err, "error initializing GPG")
	}
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		return errors.Wrap(err, "signing is not supported")
	}

	systemRegistriesDirPath := trust.RegistriesDirPath(sc)
	registryConfigs, err := trust.LoadAndMergeConfig(systemRegistriesDirPath)
	if err != nil {
		return errors.Wrapf(err, "error reading registry configuration")
	}

	for _, signimage := range args {
		srcRef, err := alltransports.ParseImageName(signimage)
		if err != nil {
			return errors.Wrapf(err, "error parsing image name")
		}
		rawSource, err := srcRef.NewImageSource(getContext(), sc)
		if err != nil {
			return errors.Wrapf(err, "error getting image source")
		}
		err = rawSource.Close()
		if err != nil {
			logrus.Errorf("unable to close new image source %q", err)
		}
		manifest, _, err := rawSource.GetManifest(getContext(), nil)
		if err != nil {
			return errors.Wrapf(err, "error getting manifest")
		}
		dockerReference := rawSource.Reference().DockerReference()
		if dockerReference == nil {
			return errors.Errorf("cannot determine canonical Docker reference for destination %s", transports.ImageName(rawSource.Reference()))
		}

		// create the signstore file
		rtc, err := runtime.GetConfig()
		if err != nil {
			return err
		}
		newImage, err := runtime.ImageRuntime().New(getContext(), signimage, rtc.SignaturePolicyPath, "", os.Stderr, &dockerRegistryOptions, image.SigningOptions{SignBy: signby}, nil, util.PullImageMissing)
		if err != nil {
			return errors.Wrapf(err, "error pulling image %s", signimage)
		}

		registryInfo := trust.HaveMatchRegistry(rawSource.Reference().DockerReference().String(), registryConfigs)
		if registryInfo != nil {
			if sigStoreDir == "" {
				sigStoreDir = registryInfo.SigStoreStaging
				if sigStoreDir == "" {
					sigStoreDir = registryInfo.SigStore
				}
			}
			sigStoreDir, err = isValidSigStoreDir(sigStoreDir)
			if err != nil {
				return errors.Wrapf(err, "invalid signature storage %s", sigStoreDir)
			}
		}
		if sigStoreDir == "" {
			sigStoreDir = SignatureStoreDir
		}

		repos, err := newImage.RepoDigests()
		if err != nil {
			return errors.Wrapf(err, "error calculating repo digests for %s", signimage)
		}
		if len(repos) == 0 {
			logrus.Errorf("no repodigests associated with the image %s", signimage)
			continue
		}

		// create signature
		newSig, err := signature.SignDockerManifest(manifest, dockerReference.String(), mech, signby)
		if err != nil {
			return errors.Wrapf(err, "error creating new signature")
		}

		trimmedDigest := strings.TrimPrefix(repos[0], strings.Split(repos[0], "/")[0])
		sigStoreDir = filepath.Join(sigStoreDir, strings.Replace(trimmedDigest, ":", "=", 1))
		if err := os.MkdirAll(sigStoreDir, 0751); err != nil {
			// The directory is allowed to exist
			if !os.IsExist(err) {
				logrus.Errorf("error creating directory %s: %s", sigStoreDir, err)
				continue
			}
		}
		sigFilename, err := getSigFilename(sigStoreDir)
		if err != nil {
			logrus.Errorf("error creating sigstore file: %v", err)
			continue
		}
		err = ioutil.WriteFile(filepath.Join(sigStoreDir, sigFilename), newSig, 0644)
		if err != nil {
			logrus.Errorf("error storing signature for %s", rawSource.Reference().DockerReference().String())
			continue
		}
	}
	return nil
}

func getSigFilename(sigStoreDirPath string) (string, error) {
	sigFileSuffix := 1
	sigFiles, err := ioutil.ReadDir(sigStoreDirPath)
	if err != nil {
		return "", err
	}
	sigFilenames := make(map[string]bool)
	for _, file := range sigFiles {
		sigFilenames[file.Name()] = true
	}
	for {
		sigFilename := "signature-" + strconv.Itoa(sigFileSuffix)
		if _, exists := sigFilenames[sigFilename]; !exists {
			return sigFilename, nil
		}
		sigFileSuffix++
	}
}

func isValidSigStoreDir(sigStoreDir string) (string, error) {
	writeURIs := map[string]bool{"file": true}
	url, err := url.Parse(sigStoreDir)
	if err != nil {
		return sigStoreDir, errors.Wrapf(err, "invalid directory %s", sigStoreDir)
	}
	_, exists := writeURIs[url.Scheme]
	if !exists {
		return sigStoreDir, errors.Errorf("writing to %s is not supported. Use a supported scheme", sigStoreDir)
	}
	sigStoreDir = url.Path
	return sigStoreDir, nil
}
