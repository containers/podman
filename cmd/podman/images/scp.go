package images

import (
	"context"
	"fmt"
	"io/ioutil"
	urlP "net/url"
	"os"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/parse"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/system/connection"
	"github.com/containers/podman/v3/libpod/define"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/docker/distribution/reference"
	scpD "github.com/dtylman/scp"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	saveScpDescription = `Securely copy an image from one host to another.`
	imageScpCommand    = &cobra.Command{
		Use:               "scp [options] IMAGE [HOST::]",
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Long:              saveScpDescription,
		Short:             "securely copy images",
		RunE:              scp,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: common.AutocompleteScp,
		Example:           `podman image scp myimage:latest otherhost::`,
	}
)

var (
	scpOpts entities.ImageScpOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageScpCommand,
		Parent:  imageCmd,
	})
	scpFlags(imageScpCommand)
}

func scpFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVarP(&scpOpts.Save.Quiet, "quiet", "q", false, "Suppress the output")
}

func scp(cmd *cobra.Command, args []string) (finalErr error) {
	var (
		// TODO add tag support for images
		err error
	)
	if scpOpts.Save.Quiet { // set quiet for both load and save
		scpOpts.Load.Quiet = true
	}
	f, err := ioutil.TempFile("", "podman") // open temp file for load/save output
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())

	scpOpts.Save.Output = f.Name()
	scpOpts.Load.Input = scpOpts.Save.Output
	if err := parse.ValidateFileName(saveOpts.Output); err != nil {
		return err
	}
	confR, err := config.NewConfig("") // create a hand made config for the remote engine since we might use remote and native at once
	if err != nil {
		return errors.Wrapf(err, "could not make config")
	}
	abiEng, err := registry.NewImageEngine(cmd, args) // abi native engine
	if err != nil {
		return err
	}

	cfg, err := config.ReadCustomConfig() // get ready to set ssh destination if necessary
	if err != nil {
		return err
	}
	serv, err := parseArgs(args, cfg) // parses connection data and "which way" we are loading and saving
	if err != nil {
		return err
	}
	// TODO: Add podman remote support
	confR.Engine = config.EngineConfig{Remote: true, CgroupManager: "cgroupfs", ServiceDestinations: serv} // pass the service dest (either remote or something else) to engine
	switch {
	case scpOpts.FromRemote: // if we want to load FROM the remote
		err = saveToRemote(scpOpts.SourceImageName, scpOpts.Save.Output, "", scpOpts.URI[0], scpOpts.Iden[0])
		if err != nil {
			return err
		}
		if scpOpts.ToRemote { // we want to load remote -> remote
			rep, err := loadToRemote(scpOpts.Save.Output, "", scpOpts.URI[1], scpOpts.Iden[1])
			if err != nil {
				return err
			}
			fmt.Println(rep)
			break
		}
		report, err := abiEng.Load(context.Background(), scpOpts.Load)
		if err != nil {
			return err
		}
		fmt.Println("Loaded image(s): " + strings.Join(report.Names, ","))
	case scpOpts.ToRemote: // remote host load
		scpOpts.Save.Format = "oci-archive"
		abiErr := abiEng.Save(context.Background(), scpOpts.SourceImageName, []string{}, scpOpts.Save) // save the image locally before loading it on remote, local, or ssh
		if abiErr != nil {
			errors.Wrapf(abiErr, "could not save image as specified")
		}
		rep, err := loadToRemote(scpOpts.Save.Output, "", scpOpts.URI[0], scpOpts.Iden[0])
		if err != nil {
			return err
		}
		fmt.Println(rep)
	// TODO: Add podman remote support
	default: // else native load
		if scpOpts.Tag != "" {
			return errors.Wrapf(define.ErrInvalidArg, "Renaming of an image is currently not supported")
		}
		scpOpts.Save.Format = "oci-archive"
		abiErr := abiEng.Save(context.Background(), scpOpts.SourceImageName, []string{}, scpOpts.Save) // save the image locally before loading it on remote, local, or ssh
		if abiErr != nil {
			errors.Wrapf(abiErr, "could not save image as specified")
		}
		rep, err := abiEng.Load(context.Background(), scpOpts.Load)
		if err != nil {
			return err
		}
		fmt.Println("Loaded image(s): " + strings.Join(rep.Names, ","))
	}
	return nil
}

// loadToRemote takes image and remote connection information. it connects to the specified client
// and copies the saved image dir over to the remote host and then loads it onto the machine
// returns a string containing output or an error
func loadToRemote(localFile string, tag string, url *urlP.URL, iden string) (string, error) {
	dial, remoteFile, err := createConnection(url, iden)
	if err != nil {
		return "", err
	}
	defer dial.Close()

	n, err := scpD.CopyTo(dial, localFile, remoteFile)
	if err != nil {
		errOut := (strconv.Itoa(int(n)) + " Bytes copied before error")
		return " ", errors.Wrapf(err, errOut)
	}
	run := ""
	if tag != "" {
		return "", errors.Wrapf(define.ErrInvalidArg, "Renaming of an image is currently not supported")
	}
	podman := os.Args[0]
	run = podman + " image load --input=" + remoteFile + ";rm " + remoteFile // run ssh image load of the file copied via scp
	out, err := connection.ExecRemoteCommand(dial, run)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(out, "\n"), nil
}

// saveToRemote takes image information and remote connection information. it connects to the specified client
// and saves the specified image on the remote machine and then copies it to the specified local location
// returns an error if one occurs.
func saveToRemote(image, localFile string, tag string, uri *urlP.URL, iden string) error {
	dial, remoteFile, err := createConnection(uri, iden)

	if err != nil {
		return err
	}
	defer dial.Close()

	if tag != "" {
		return errors.Wrapf(define.ErrInvalidArg, "Renaming of an image is currently not supported")
	}
	podman := os.Args[0]
	run := podman + " image save " + image + " --format=oci-archive --output=" + remoteFile // run ssh image load of the file copied via scp. Files are reverse in this case...
	_, err = connection.ExecRemoteCommand(dial, run)
	if err != nil {
		return nil
	}
	n, err := scpD.CopyFrom(dial, remoteFile, localFile)
	connection.ExecRemoteCommand(dial, "rm "+remoteFile)
	if err != nil {
		errOut := (strconv.Itoa(int(n)) + " Bytes copied before error")
		return errors.Wrapf(err, errOut)
	}
	return nil
}

// makeRemoteFile creates the necessary remote file on the host to
// save or load the image to. returns a string with the file name or an error
func makeRemoteFile(dial *ssh.Client) (string, error) {
	run := "mktemp"
	remoteFile, err := connection.ExecRemoteCommand(dial, run)
	if err != nil {
		return "", err
	}
	remoteFile = strings.TrimSuffix(remoteFile, "\n")
	if err != nil {
		return "", err
	}
	return remoteFile, nil
}

// createConnections takes a boolean determining which ssh client to dial
// and returns the dials client, its newly opened remote file, and an error if applicable.
func createConnection(url *urlP.URL, iden string) (*ssh.Client, string, error) {
	cfg, err := connection.ValidateAndConfigure(url, iden)
	if err != nil {
		return nil, "", err
	}
	dialAdd, err := ssh.Dial("tcp", url.Host, cfg) // dial the client
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to connect")
	}
	file, err := makeRemoteFile(dialAdd)
	if err != nil {
		return nil, "", err
	}

	return dialAdd, file, nil
}

// validateImageName makes sure that the image given is valid and no injections are occurring
// we simply use this for error checking, bot setting the image
func validateImageName(input string) error {
	// ParseNormalizedNamed transforms a shortname image into its
	// full name reference so busybox => docker.io/library/busybox
	// we want to keep our shortnames, so only return an error if
	// we cannot parse what th euser has given us
	_, err := reference.ParseNormalizedNamed(input)
	return err
}

// remoteArgLength is a helper function to simplify the extracting of host argument data
// returns an int which contains the length of a specified index in a host::image string
func remoteArgLength(input string, side int) int {
	return len((strings.Split(input, "::"))[side])
}

// parseArgs returns the valid connection data based off of the information provided by the user
// args is an array of the command arguments and cfg is tooling configuration used to get service destinations
// returned is serv and an error if applicable. serv is a map of service destinations with the connection name as the index
// this connection name is intended to be used as EngineConfig.ServiceDestinations
// this function modifies the global scpOpt entities: FromRemote, ToRemote, Connections, and SourceImageName
func parseArgs(args []string, cfg *config.Config) (map[string]config.Destination, error) {
	serv := map[string]config.Destination{}
	cliConnections := []string{}
	switch len(args) {
	case 1:
		if strings.Contains(args[0], "::") {
			scpOpts.FromRemote = true
			cliConnections = append(cliConnections, args[0])
		} else {
			err := validateImageName(args[0])
			if err != nil {
				return nil, err
			}
			scpOpts.SourceImageName = args[0]
		}
	case 2:
		if strings.Contains(args[0], "::") {
			if !(strings.Contains(args[1], "::")) && remoteArgLength(args[0], 1) == 0 { // if an image is specified, this mean we are loading to our client
				cliConnections = append(cliConnections, args[0])
				scpOpts.ToRemote = true
				scpOpts.SourceImageName = args[1]
			} else if strings.Contains(args[1], "::") { // both remote clients
				scpOpts.FromRemote = true
				scpOpts.ToRemote = true
				if remoteArgLength(args[0], 1) == 0 { // is save->load w/ one image name
					cliConnections = append(cliConnections, args[0])
					cliConnections = append(cliConnections, args[1])
				} else if remoteArgLength(args[0], 1) > 0 && remoteArgLength(args[1], 1) > 0 {
					//in the future, this function could, instead of rejecting renames, also set a DestImageName field
					return nil, errors.Wrapf(define.ErrInvalidArg, "cannot specify an image rename")
				} else { // else its a load save (order of args)
					cliConnections = append(cliConnections, args[1])
					cliConnections = append(cliConnections, args[0])
				}
			} else {
				//in the future, this function could, instead of rejecting renames, also set a DestImageName field
				return nil, errors.Wrapf(define.ErrInvalidArg, "cannot specify an image rename")
			}
		} else if strings.Contains(args[1], "::") { // if we are given image host::
			if remoteArgLength(args[1], 1) > 0 {
				//in the future, this function could, instead of rejecting renames, also set a DestImageName field
				return nil, errors.Wrapf(define.ErrInvalidArg, "cannot specify an image rename")
			}
			err := validateImageName(args[0])
			if err != nil {
				return nil, err
			}
			scpOpts.SourceImageName = args[0]
			scpOpts.ToRemote = true
			cliConnections = append(cliConnections, args[1])
		} else {
			//in the future, this function could, instead of rejecting renames, also set a DestImageName field
			return nil, errors.Wrapf(define.ErrInvalidArg, "cannot specify an image rename")
		}
	}
	var url string
	var iden string
	for i, val := range cliConnections {
		splitEnv := strings.SplitN(val, "::", 2)
		scpOpts.Connections = append(scpOpts.Connections, splitEnv[0])
		if len(splitEnv[1]) != 0 {
			err := validateImageName(splitEnv[1])
			if err != nil {
				return nil, err
			}
			scpOpts.SourceImageName = splitEnv[1]
			//TODO: actually use the new name given by the user
		}
		conn, found := cfg.Engine.ServiceDestinations[scpOpts.Connections[i]]
		if found {
			url = conn.URI
			iden = conn.Identity
		} else { // no match, warn user and do a manual connection.
			url = "ssh://" + scpOpts.Connections[i]
			iden = ""
			logrus.Warnf("Unknown connection name given. Please use system connection add to specify the default remote socket location")
		}
		urlT, err := urlP.Parse(url) // create an actual url to pass to exec command
		if err != nil {
			return nil, err
		}
		if urlT.User.Username() == "" {
			if urlT.User, err = connection.GetUserInfo(urlT); err != nil {
				return nil, err
			}
		}
		scpOpts.URI = append(scpOpts.URI, urlT)
		scpOpts.Iden = append(scpOpts.Iden, iden)
	}
	return serv, nil
}
