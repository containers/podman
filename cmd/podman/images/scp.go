package images

import (
	"context"
	"fmt"
	"io/ioutil"
	urlP "net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/cmd/podman/common"
	"github.com/containers/podman/v4/cmd/podman/registry"
	"github.com/containers/podman/v4/cmd/podman/system/connection"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/utils"
	scpD "github.com/dtylman/scp"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	saveScpDescription = `Securely copy an image from one host to another.`
	imageScpCommand    = &cobra.Command{
		Use: "scp [options] IMAGE [HOST::]",
		Annotations: map[string]string{
			registry.UnshareNSRequired: "",
			registry.ParentNSRequired:  "",
			registry.EngineMode:        registry.ABIMode,
		},
		Long:              saveScpDescription,
		Short:             "securely copy images",
		RunE:              scp,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: common.AutocompleteScp,
		Example:           `podman image scp myimage:latest otherhost::`,
	}
)

var (
	parentFlags []string
	quiet       bool
	source      entities.ImageScpOptions
	dest        entities.ImageScpOptions
	sshInfo     entities.ImageScpConnections
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
	flags.BoolVarP(&quiet, "quiet", "q", false, "Suppress the output")
}

func scp(cmd *cobra.Command, args []string) (finalErr error) {
	var (
		// TODO add tag support for images
		err error
	)
	for i, val := range os.Args {
		if val == "image" {
			break
		}
		if i == 0 {
			continue
		}
		if strings.Contains(val, "CIRRUS") { // need to skip CIRRUS flags for testing suite purposes
			continue
		}
		parentFlags = append(parentFlags, val)
	}
	podman, err := os.Executable()
	if err != nil {
		return err
	}
	f, err := ioutil.TempFile("", "podman") // open temp file for load/save output
	if err != nil {
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
	locations := []*entities.ImageScpOptions{}
	cliConnections := []string{}
	var flipConnections bool
	for _, arg := range args {
		loc, connect, err := parseImageSCPArg(arg)
		if err != nil {
			return err
		}
		locations = append(locations, loc)
		cliConnections = append(cliConnections, connect...)
	}
	source = *locations[0]
	switch {
	case len(locations) > 1:
		if flipConnections, err = validateSCPArgs(locations); err != nil {
			return err
		}
		if flipConnections { // the order of cliConnections matters, we need to flip both arrays since the args are parsed separately sometimes.
			cliConnections[0], cliConnections[1] = cliConnections[1], cliConnections[0]
			locations[0], locations[1] = locations[1], locations[0]
		}
		dest = *locations[1]
	case len(locations) == 1:
		switch {
		case len(locations[0].Image) == 0:
			return errors.Wrapf(define.ErrInvalidArg, "no source image specified")
		case len(locations[0].Image) > 0 && !locations[0].Remote && len(locations[0].User) == 0: // if we have podman image scp $IMAGE
			return errors.Wrapf(define.ErrInvalidArg, "must specify a destination")
		}
	}

	source.Quiet = quiet
	source.File = f.Name() // after parsing the arguments, set the file for the save/load
	dest.File = source.File
	if err = os.Remove(source.File); err != nil { // remove the file and simply use its name so podman creates the file upon save. avoids umask errors
		return err
	}

	allLocal := true // if we are all localhost, do not validate connections but if we are using one localhost and one non we need to use sshd
	for _, val := range cliConnections {
		if !strings.Contains(val, "@localhost::") {
			allLocal = false
			break
		}
	}
	if allLocal {
		cliConnections = []string{}
	}

	var serv map[string]config.Destination
	serv, err = GetServiceInformation(cliConnections, cfg)
	if err != nil {
		return err
	}

	// TODO: Add podman remote support
	confR.Engine = config.EngineConfig{Remote: true, CgroupManager: "cgroupfs", ServiceDestinations: serv} // pass the service dest (either remote or something else) to engine
	saveCmd, loadCmd := createCommands(podman)
	switch {
	case source.Remote: // if we want to load FROM the remote, dest can either be local or remote in this case
		err = saveToRemote(source.Image, source.File, "", sshInfo.URI[0], sshInfo.Identities[0])
		if err != nil {
			return err
		}
		if dest.Remote { // we want to load remote -> remote, both source and dest are remote
			rep, err := loadToRemote(dest.File, "", sshInfo.URI[1], sshInfo.Identities[1])
			if err != nil {
				return err
			}
			fmt.Println(rep)
			break
		}
		err = execPodman(podman, loadCmd)
		if err != nil {
			return err
		}
	case dest.Remote: // remote host load, implies source is local
		err = execPodman(podman, saveCmd)
		if err != nil {
			return err
		}
		rep, err := loadToRemote(source.File, "", sshInfo.URI[0], sshInfo.Identities[0])
		if err != nil {
			return err
		}
		fmt.Println(rep)
		if err = os.Remove(source.File); err != nil {
			return err
		}
	// TODO: Add podman remote support
	default: // else native load, both source and dest are local and transferring between users
		if source.User == "" { // source user has to be set, destination does not
			source.User = os.Getenv("USER")
			if source.User == "" {
				u, err := user.Current()
				if err != nil {
					return errors.Wrapf(err, "could not obtain user, make sure the environmental variable $USER is set")
				}
				source.User = u.Username
			}
		}
		err := abiEng.Transfer(context.Background(), source, dest, parentFlags)
		if err != nil {
			return err
		}
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
		errOut := strconv.Itoa(int(n)) + " Bytes copied before error"
		return " ", errors.Wrapf(err, errOut)
	}
	var run string
	if tag != "" {
		return "", errors.Wrapf(define.ErrInvalidArg, "Renaming of an image is currently not supported")
	}
	podman := os.Args[0]
	run = podman + " image load --input=" + remoteFile + ";rm " + remoteFile // run ssh image load of the file copied via scp
	out, err := connection.ExecRemoteCommand(dial, run)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(out), "\n"), nil
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
		return err
	}
	n, err := scpD.CopyFrom(dial, remoteFile, localFile)
	if _, conErr := connection.ExecRemoteCommand(dial, "rm "+remoteFile); conErr != nil {
		logrus.Errorf("Removing file on endpoint: %v", conErr)
	}
	if err != nil {
		errOut := strconv.Itoa(int(n)) + " Bytes copied before error"
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
	return strings.TrimSuffix(string(remoteFile), "\n"), nil
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

// GetSerivceInformation takes the parsed list of hosts to connect to and validates the information
func GetServiceInformation(cliConnections []string, cfg *config.Config) (map[string]config.Destination, error) {
	var serv map[string]config.Destination
	var url string
	var iden string
	for i, val := range cliConnections {
		splitEnv := strings.SplitN(val, "::", 2)
		sshInfo.Connections = append(sshInfo.Connections, splitEnv[0])
		if len(splitEnv[1]) != 0 {
			err := validateImageName(splitEnv[1])
			if err != nil {
				return nil, err
			}
			source.Image = splitEnv[1]
			//TODO: actually use the new name given by the user
		}
		conn, found := cfg.Engine.ServiceDestinations[sshInfo.Connections[i]]
		if found {
			url = conn.URI
			iden = conn.Identity
		} else { // no match, warn user and do a manual connection.
			url = "ssh://" + sshInfo.Connections[i]
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
		sshInfo.URI = append(sshInfo.URI, urlT)
		sshInfo.Identities = append(sshInfo.Identities, iden)
	}
	return serv, nil
}

// execPodman executes the podman save/load command given the podman binary
func execPodman(podman string, command []string) error {
	cmd := exec.Command(podman)
	utils.CreateSCPCommand(cmd, command[1:])
	logrus.Debugf("Executing podman command: %q", cmd)
	return cmd.Run()
}

// createCommands forms the podman save and load commands used by SCP
func createCommands(podman string) ([]string, []string) {
	var parentString string
	quiet := ""
	if source.Quiet {
		quiet = "-q "
	}
	if len(parentFlags) > 0 {
		parentString = strings.Join(parentFlags, " ") + " " // if there are parent args, an extra space needs to be added
	} else {
		parentString = strings.Join(parentFlags, " ")
	}
	loadCmd := strings.Split(fmt.Sprintf("%s %sload %s--input %s", podman, parentString, quiet, dest.File), " ")
	saveCmd := strings.Split(fmt.Sprintf("%s %vsave %s--output %s %s", podman, parentString, quiet, source.File, source.Image), " ")
	return saveCmd, loadCmd
}
