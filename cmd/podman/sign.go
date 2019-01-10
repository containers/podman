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
	"github.com/containers/libpod/cmd/podman/libpodruntime"
	"github.com/containers/libpod/libpod/image"
	"github.com/containers/libpod/pkg/trust"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	signFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "sign-by",
			Usage: "Name of the signing key",
		},
		cli.StringFlag{
			Name:  "directory, d",
			Usage: "Define an alternate directory to store signatures",
		},
	}

	signDescription = "Create a signature file that can be used later to verify the image"
	signCommand     = cli.Command{
		Name:         "sign",
		Usage:        "Sign an image",
		Description:  signDescription,
		Flags:        sortFlags(signFlags),
		Action:       signCmd,
		ArgsUsage:    "IMAGE-NAME [IMAGE-NAME ...]",
		OnUsageError: usageErrorHandler,
	}
)

// SignatureStoreDir defines default directory to store signatures
const SignatureStoreDir = "/var/lib/containers/sigstore"

func signCmd(c *cli.Context) error {
	args := c.Args()
	if len(args) < 1 {
		return errors.Errorf("at least one image name must be specified")
	}
	runtime, err := libpodruntime.GetRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "could not create runtime")
	}
	defer runtime.Shutdown(false)

	signby := c.String("sign-by")
	if signby == "" {
		return errors.Errorf("please provide an identity")
	}

	var sigStoreDir string
	if c.IsSet("directory") {
		sigStoreDir = c.String("directory")
		if _, err := os.Stat(sigStoreDir); err != nil {
			return errors.Wrapf(err, "invalid directory %s", sigStoreDir)
		}
	}

	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return errors.Wrap(err, "error initializing GPG")
	}
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		return errors.Wrap(err, "signing is not supported")
	}

	systemRegistriesDirPath := trust.RegistriesDirPath(runtime.SystemContext())
	registryConfigs, err := trust.LoadAndMergeConfig(systemRegistriesDirPath)
	if err != nil {
		return errors.Wrapf(err, "error reading registry configuration")
	}

	for _, signimage := range args {
		srcRef, err := alltransports.ParseImageName(signimage)
		if err != nil {
			return errors.Wrapf(err, "error parsing image name")
		}
		rawSource, err := srcRef.NewImageSource(getContext(), runtime.SystemContext())
		if err != nil {
			return errors.Wrapf(err, "error getting image source")
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
		newImage, err := runtime.ImageRuntime().New(getContext(), signimage, runtime.GetConfig().SignaturePolicyPath, "", os.Stderr, nil, image.SigningOptions{SignBy: signby}, false)
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
