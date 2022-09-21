package utils

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/ssh"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/sirupsen/logrus"
)

func ExecuteTransfer(src, dst string, parentFlags []string, quiet bool, sshMode ssh.EngineMode) (*entities.ImageLoadReport, *entities.ImageScpOptions, *entities.ImageScpOptions, []string, error) {
	source := entities.ImageScpOptions{}
	dest := entities.ImageScpOptions{}
	sshInfo := entities.ImageScpConnections{}
	report := entities.ImageLoadReport{Names: []string{}}

	podman, err := os.Executable()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	f, err := os.CreateTemp("", "podman") // open temp file for load/save output
	if err != nil {
		return nil, nil, nil, nil, err
	}

	confR, err := config.NewConfig("") // create a hand made config for the remote engine since we might use remote and native at once
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not make config: %w", err)
	}

	locations := []*entities.ImageScpOptions{}
	cliConnections := []string{}
	args := []string{src}
	if len(dst) > 0 {
		args = append(args, dst)
	}
	for _, arg := range args {
		loc, connect, err := ParseImageSCPArg(arg)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		locations = append(locations, loc)
		cliConnections = append(cliConnections, connect...)
	}
	source = *locations[0]
	switch {
	case len(locations) > 1:
		if err = ValidateSCPArgs(locations); err != nil {
			return nil, nil, nil, nil, err
		}
		dest = *locations[1]
	case len(locations) == 1:
		switch {
		case len(locations[0].Image) == 0:
			return nil, nil, nil, nil, fmt.Errorf("no source image specified: %w", define.ErrInvalidArg)
		case len(locations[0].Image) > 0 && !locations[0].Remote && len(locations[0].User) == 0: // if we have podman image scp $IMAGE
			return nil, nil, nil, nil, fmt.Errorf("must specify a destination: %w", define.ErrInvalidArg)
		}
	}

	source.Quiet = quiet
	source.File = f.Name() // after parsing the arguments, set the file for the save/load
	dest.File = source.File
	defer os.Remove(source.File)

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

	cfg, err := config.ReadCustomConfig() // get ready to set ssh destination if necessary
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var serv map[string]config.Destination
	serv, err = GetServiceInformation(&sshInfo, cliConnections, cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	confR.Engine = config.EngineConfig{Remote: true, CgroupManager: "cgroupfs", ServiceDestinations: serv} // pass the service dest (either remote or something else) to engine
	saveCmd, loadCmd := CreateCommands(source, dest, parentFlags, podman)

	switch {
	case source.Remote: // if we want to load FROM the remote, dest can either be local or remote in this case
		err = SaveToRemote(source.Image, source.File, "", sshInfo.URI[0], sshInfo.Identities[0], sshMode)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if dest.Remote { // we want to load remote -> remote, both source and dest are remote
			rep, id, err := LoadToRemote(dest, dest.File, "", sshInfo.URI[1], sshInfo.Identities[1], sshMode)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if len(rep) > 0 {
				fmt.Println(rep)
			}
			if len(id) > 0 {
				report.Names = append(report.Names, id)
			}
			break
		}
		id, err := ExecPodman(dest, podman, loadCmd)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if len(id) > 0 {
			report.Names = append(report.Names, id)
		}
	case dest.Remote: // remote host load, implies source is local
		_, err = ExecPodman(dest, podman, saveCmd)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		rep, id, err := LoadToRemote(dest, source.File, "", sshInfo.URI[0], sshInfo.Identities[0], sshMode)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if len(rep) > 0 {
			fmt.Println(rep)
		}
		if len(id) > 0 {
			report.Names = append(report.Names, id)
		}
		if err = os.Remove(source.File); err != nil {
			return nil, nil, nil, nil, err
		}
	default: // else native load, both source and dest are local and transferring between users
		if source.User == "" { // source user has to be set, destination does not
			source.User = os.Getenv("USER")
			if source.User == "" {
				u, err := user.Current()
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("could not obtain user, make sure the environmental variable $USER is set: %w", err)
				}
				source.User = u.Username
			}
		}
		return nil, &source, &dest, parentFlags, nil // transfer needs to be done in ABI due to cross issues
	}

	return &report, nil, nil, nil, nil
}

// CreateSCPCommand takes an existing command, appends the given arguments and returns a configured podman command for image scp
func CreateSCPCommand(cmd *exec.Cmd, command []string) *exec.Cmd {
	cmd.Args = append(cmd.Args, command...)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd
}

// ScpTag is a helper function for native podman to tag an image after a local load from image SCP
func ScpTag(cmd *exec.Cmd, podman string, dest entities.ImageScpOptions) error {
	cmd.Stdout = nil
	out, err := cmd.Output() // this function captures the output temporarily in order to execute the next command
	if err != nil {
		return err
	}
	image := ExtractImage(out)
	if cmd.Args[0] == "sudo" { // transferRootless will need the sudo since we are loading to sudo from a user acct
		cmd = exec.Command("sudo", podman, "tag", image, dest.Tag)
	} else {
		cmd = exec.Command(podman, "tag", image, dest.Tag)
	}
	cmd.Stdout = os.Stdout
	return cmd.Run()
}

// ExtractImage pulls out the last line of output from save/load (image id)
func ExtractImage(out []byte) string {
	fmt.Println(strings.TrimSuffix(string(out), "\n"))         // print output
	stringOut := string(out)                                   // get all output
	arrOut := strings.Split(stringOut, " ")                    // split it into an array
	return strings.ReplaceAll(arrOut[len(arrOut)-1], "\n", "") // replace the trailing \n
}

// LoginUser starts the user process on the host so that image scp can use systemd-run
func LoginUser(user string) (*exec.Cmd, error) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		return nil, err
	}
	machinectl, err := exec.LookPath("machinectl")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(machinectl, "shell", "-q", user+"@.host", sleep, "inf")
	err = cmd.Start()
	return cmd, err
}

// loadToRemote takes image and remote connection information. it connects to the specified client
// and copies the saved image dir over to the remote host and then loads it onto the machine
// returns a string containing output or an error
func LoadToRemote(dest entities.ImageScpOptions, localFile string, tag string, url *url.URL, iden string, sshEngine ssh.EngineMode) (string, string, error) {
	port, err := strconv.Atoi(url.Port())
	if err != nil {
		return "", "", err
	}

	remoteFile, err := ssh.Exec(&ssh.ConnectionExecOptions{Host: url.String(), Port: port, User: url.User, Args: []string{"mktemp"}}, sshEngine)
	if err != nil {
		return "", "", err
	}

	opts := ssh.ConnectionScpOptions{User: url.User, Identity: iden, Port: port, Source: localFile, Destination: "ssh://" + url.User.String() + "@" + url.Hostname() + ":" + remoteFile}
	scpRep, err := ssh.Scp(&opts, sshEngine)
	if err != nil {
		return "", "", err
	}
	out, err := ssh.Exec(&ssh.ConnectionExecOptions{Host: url.String(), Port: port, User: url.User, Args: []string{"podman", "image", "load", "--input=" + scpRep + ";", "rm", scpRep}}, sshEngine)
	if err != nil {
		return "", "", err
	}
	if tag != "" {
		return "", "", fmt.Errorf("renaming of an image is currently not supported: %w", define.ErrInvalidArg)
	}
	rep := strings.TrimSuffix(out, "\n")
	outArr := strings.Split(rep, " ")
	id := outArr[len(outArr)-1]
	if len(dest.Tag) > 0 { // tag the remote image using the output ID
		_, err := ssh.Exec(&ssh.ConnectionExecOptions{Host: url.Hostname(), Port: port, User: url.User, Args: []string{"podman", "image", "tag", id, dest.Tag}}, sshEngine)
		if err != nil {
			return "", "", err
		}
		if err != nil {
			return "", "", err
		}
	}
	return rep, id, nil
}

// saveToRemote takes image information and remote connection information. it connects to the specified client
// and saves the specified image on the remote machine and then copies it to the specified local location
// returns an error if one occurs.
func SaveToRemote(image, localFile string, tag string, uri *url.URL, iden string, sshEngine ssh.EngineMode) error {
	if tag != "" {
		return fmt.Errorf("renaming of an image is currently not supported: %w", define.ErrInvalidArg)
	}

	port, err := strconv.Atoi(uri.Port())
	if err != nil {
		return err
	}

	remoteFile, err := ssh.Exec(&ssh.ConnectionExecOptions{Host: uri.String(), Port: port, User: uri.User, Args: []string{"mktemp"}}, sshEngine)
	if err != nil {
		return err
	}

	_, err = ssh.Exec(&ssh.ConnectionExecOptions{Host: uri.String(), Port: port, User: uri.User, Args: []string{"podman", "image", "save", image, "--format", "oci-archive", "--output", remoteFile}}, sshEngine)
	if err != nil {
		return err
	}

	opts := ssh.ConnectionScpOptions{User: uri.User, Identity: iden, Port: port, Source: "ssh://" + uri.User.String() + "@" + uri.Hostname() + ":" + remoteFile, Destination: localFile}
	scpRep, err := ssh.Scp(&opts, sshEngine)
	if err != nil {
		return err
	}
	_, err = ssh.Exec(&ssh.ConnectionExecOptions{Host: uri.String(), Port: port, User: uri.User, Args: []string{"rm", scpRep}}, sshEngine)
	if err != nil {
		logrus.Errorf("Removing file on endpoint: %v", err)
	}

	return nil
}

// execPodman executes the podman save/load command given the podman binary
func ExecPodman(dest entities.ImageScpOptions, podman string, command []string) (string, error) {
	cmd := exec.Command(podman)
	CreateSCPCommand(cmd, command[1:])
	logrus.Debugf("Executing podman command: %q", cmd)
	if strings.Contains(strings.Join(command, " "), "load") { // need to tag
		if len(dest.Tag) > 0 {
			return "", ScpTag(cmd, podman, dest)
		}
		cmd.Stdout = nil
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		img := ExtractImage(out)
		return img, nil
	}
	return "", cmd.Run()
}

// createCommands forms the podman save and load commands used by SCP
func CreateCommands(source entities.ImageScpOptions, dest entities.ImageScpOptions, parentFlags []string, podman string) ([]string, []string) {
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

// parseImageSCPArg returns the valid connection, and source/destination data based off of the information provided by the user
// arg is a string containing one of the cli arguments returned is a filled out source/destination options structs as well as a connections array and an error if applicable
func ParseImageSCPArg(arg string) (*entities.ImageScpOptions, []string, error) {
	location := entities.ImageScpOptions{}
	var err error
	cliConnections := []string{}

	switch {
	case strings.Contains(arg, "@localhost::"): // image transfer between users
		location.User = strings.Split(arg, "@")[0]
		location, err = ValidateImagePortion(location, arg)
		if err != nil {
			return nil, nil, err
		}
		cliConnections = append(cliConnections, arg)
	case strings.Contains(arg, "::"):
		location, err = ValidateImagePortion(location, arg)
		if err != nil {
			return nil, nil, err
		}
		location.Remote = true
		cliConnections = append(cliConnections, arg)
	default:
		location.Image = arg
	}
	return &location, cliConnections, nil
}

func ValidateImagePortion(location entities.ImageScpOptions, arg string) (entities.ImageScpOptions, error) {
	if RemoteArgLength(arg, 1) > 0 {
		before := strings.Split(arg, "::")[1]
		name := ValidateImageName(before)
		if before != name {
			location.Image = name
		} else {
			location.Image = before
		} // this will get checked/set again once we validate connections
	}
	return location, nil
}

// validateImageName makes sure that the image given is valid and no injections are occurring
// we simply use this for error checking, bot setting the image
func ValidateImageName(input string) string {
	// ParseNormalizedNamed transforms a shortname image into its
	// full name reference so busybox => docker.io/library/busybox
	// we want to keep our shortnames, so only return an error if
	// we cannot parse what the user has given us
	if ref, err := alltransports.ParseImageName(input); err == nil {
		return ref.Transport().Name()
	}
	return input
}

// validateSCPArgs takes the array of source and destination options and checks for common errors
func ValidateSCPArgs(locations []*entities.ImageScpOptions) error {
	if len(locations) > 2 {
		return fmt.Errorf("cannot specify more than two arguments: %w", define.ErrInvalidArg)
	}
	switch {
	case len(locations[0].Image) > 0 && len(locations[1].Image) > 0:
		locations[1].Tag = locations[1].Image
		locations[1].Image = ""
	case len(locations[0].Image) == 0 && len(locations[1].Image) == 0:
		return fmt.Errorf("a source image must be specified: %w", define.ErrInvalidArg)
	}
	return nil
}

// remoteArgLength is a helper function to simplify the extracting of host argument data
// returns an int which contains the length of a specified index in a host::image string
func RemoteArgLength(input string, side int) int {
	if strings.Contains(input, "::") {
		return len((strings.Split(input, "::"))[side])
	}
	return -1
}

// GetSerivceInformation takes the parsed list of hosts to connect to and validates the information
func GetServiceInformation(sshInfo *entities.ImageScpConnections, cliConnections []string, cfg *config.Config) (map[string]config.Destination, error) {
	var serv map[string]config.Destination
	var urlS string
	var iden string
	for i, val := range cliConnections {
		splitEnv := strings.SplitN(val, "::", 2)
		sshInfo.Connections = append(sshInfo.Connections, splitEnv[0])
		conn, found := cfg.Engine.ServiceDestinations[sshInfo.Connections[i]]
		if found {
			urlS = conn.URI
			iden = conn.Identity
		} else { // no match, warn user and do a manual connection.
			urlS = "ssh://" + sshInfo.Connections[i]
			iden = ""
			logrus.Warnf("Unknown connection name given. Please use system connection add to specify the default remote socket location")
		}
		urlFinal, err := url.Parse(urlS) // create an actual url to pass to exec command
		if err != nil {
			return nil, err
		}
		if urlFinal.User.Username() == "" {
			if urlFinal.User, err = GetUserInfo(urlFinal); err != nil {
				return nil, err
			}
		}
		sshInfo.URI = append(sshInfo.URI, urlFinal)
		sshInfo.Identities = append(sshInfo.Identities, iden)
	}
	return serv, nil
}

func GetUserInfo(uri *url.URL) (*url.Userinfo, error) {
	var (
		usr *user.User
		err error
	)
	if u, found := os.LookupEnv("_CONTAINERS_ROOTLESS_UID"); found {
		usr, err = user.LookupId(u)
		if err != nil {
			return nil, fmt.Errorf("failed to lookup rootless user: %w", err)
		}
	} else {
		usr, err = user.Current()
		if err != nil {
			return nil, fmt.Errorf("failed to obtain current user: %w", err)
		}
	}

	pw, set := uri.User.Password()
	if set {
		return url.UserPassword(usr.Username, pw), nil
	}
	return url.User(usr.Username), nil
}
