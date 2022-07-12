package utils

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"

	scpD "github.com/dtylman/scp"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/terminal"
	"github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func ExecuteTransfer(src, dst string, parentFlags []string, quiet bool) (*entities.ImageLoadReport, *entities.ImageScpOptions, *entities.ImageScpOptions, []string, error) {
	source := entities.ImageScpOptions{}
	dest := entities.ImageScpOptions{}
	sshInfo := entities.ImageScpConnections{}
	report := entities.ImageLoadReport{Names: []string{}}

	podman, err := os.Executable()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	f, err := ioutil.TempFile("", "podman") // open temp file for load/save output
	if err != nil {
		return nil, nil, nil, nil, err
	}

	confR, err := config.NewConfig("") // create a hand made config for the remote engine since we might use remote and native at once
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("could not make config: %w", err)
	}

	cfg, err := config.ReadCustomConfig() // get ready to set ssh destination if necessary
	if err != nil {
		return nil, nil, nil, nil, err
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
	if err = os.Remove(source.File); err != nil { // remove the file and simply use its name so podman creates the file upon save. avoids umask errors
		return nil, nil, nil, nil, err
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
	serv, err = GetServiceInformation(&sshInfo, cliConnections, cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	confR.Engine = config.EngineConfig{Remote: true, CgroupManager: "cgroupfs", ServiceDestinations: serv} // pass the service dest (either remote or something else) to engine
	saveCmd, loadCmd := CreateCommands(source, dest, parentFlags, podman)

	switch {
	case source.Remote: // if we want to load FROM the remote, dest can either be local or remote in this case
		err = SaveToRemote(source.Image, source.File, "", sshInfo.URI[0], sshInfo.Identities[0])
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if dest.Remote { // we want to load remote -> remote, both source and dest are remote
			rep, id, err := LoadToRemote(dest, dest.File, "", sshInfo.URI[1], sshInfo.Identities[1])
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
		rep, id, err := LoadToRemote(dest, source.File, "", sshInfo.URI[0], sshInfo.Identities[0])
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
func LoadToRemote(dest entities.ImageScpOptions, localFile string, tag string, url *url.URL, iden string) (string, string, error) {
	dial, remoteFile, err := CreateConnection(url, iden)
	if err != nil {
		return "", "", err
	}
	defer dial.Close()

	n, err := scpD.CopyTo(dial, localFile, remoteFile)
	if err != nil {
		errOut := strconv.Itoa(int(n)) + " Bytes copied before error"
		return " ", "", fmt.Errorf("%v: %w", errOut, err)
	}
	var run string
	if tag != "" {
		return "", "", fmt.Errorf("renaming of an image is currently not supported: %w", define.ErrInvalidArg)
	}
	podman := os.Args[0]
	run = podman + " image load --input=" + remoteFile + ";rm " + remoteFile // run ssh image load of the file copied via scp
	out, err := ExecRemoteCommand(dial, run)
	if err != nil {
		return "", "", err
	}
	rep := strings.TrimSuffix(string(out), "\n")
	outArr := strings.Split(rep, " ")
	id := outArr[len(outArr)-1]
	if len(dest.Tag) > 0 { // tag the remote image using the output ID
		run = podman + " tag " + id + " " + dest.Tag
		_, err = ExecRemoteCommand(dial, run)
		if err != nil {
			return "", "", err
		}
	}
	return rep, id, nil
}

// saveToRemote takes image information and remote connection information. it connects to the specified client
// and saves the specified image on the remote machine and then copies it to the specified local location
// returns an error if one occurs.
func SaveToRemote(image, localFile string, tag string, uri *url.URL, iden string) error {
	dial, remoteFile, err := CreateConnection(uri, iden)

	if err != nil {
		return err
	}
	defer dial.Close()

	if tag != "" {
		return fmt.Errorf("renaming of an image is currently not supported: %w", define.ErrInvalidArg)
	}
	podman := os.Args[0]
	run := podman + " image save " + image + " --format=oci-archive --output=" + remoteFile // run ssh image load of the file copied via scp. Files are reverse in this case...
	_, err = ExecRemoteCommand(dial, run)
	if err != nil {
		return err
	}
	n, err := scpD.CopyFrom(dial, remoteFile, localFile)
	if _, conErr := ExecRemoteCommand(dial, "rm "+remoteFile); conErr != nil {
		logrus.Errorf("Removing file on endpoint: %v", conErr)
	}
	if err != nil {
		errOut := strconv.Itoa(int(n)) + " Bytes copied before error"
		return fmt.Errorf("%v: %w", errOut, err)
	}
	return nil
}

// makeRemoteFile creates the necessary remote file on the host to
// save or load the image to. returns a string with the file name or an error
func MakeRemoteFile(dial *ssh.Client) (string, error) {
	run := "mktemp"
	remoteFile, err := ExecRemoteCommand(dial, run)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(remoteFile), "\n"), nil
}

// createConnections takes a boolean determining which ssh client to dial
// and returns the dials client, its newly opened remote file, and an error if applicable.
func CreateConnection(url *url.URL, iden string) (*ssh.Client, string, error) {
	cfg, err := ValidateAndConfigure(url, iden)
	if err != nil {
		return nil, "", err
	}
	dialAdd, err := ssh.Dial("tcp", url.Host, cfg) // dial the client
	if err != nil {
		return nil, "", fmt.Errorf("failed to connect: %w", err)
	}
	file, err := MakeRemoteFile(dialAdd)
	if err != nil {
		return nil, "", err
	}

	return dialAdd, file, nil
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

// validateImagePortion is a helper function to validate the image name in an SCP argument
func ValidateImagePortion(location entities.ImageScpOptions, arg string) (entities.ImageScpOptions, error) {
	if RemoteArgLength(arg, 1) > 0 {
		err := ValidateImageName(strings.Split(arg, "::")[1])
		if err != nil {
			return location, err
		}
		location.Image = strings.Split(arg, "::")[1] // this will get checked/set again once we validate connections
	}
	return location, nil
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

// validateImageName makes sure that the image given is valid and no injections are occurring
// we simply use this for error checking, bot setting the image
func ValidateImageName(input string) error {
	// ParseNormalizedNamed transforms a shortname image into its
	// full name reference so busybox => docker.io/library/busybox
	// we want to keep our shortnames, so only return an error if
	// we cannot parse what the user has given us
	_, err := reference.ParseNormalizedNamed(input)
	return err
}

// remoteArgLength is a helper function to simplify the extracting of host argument data
// returns an int which contains the length of a specified index in a host::image string
func RemoteArgLength(input string, side int) int {
	if strings.Contains(input, "::") {
		return len((strings.Split(input, "::"))[side])
	}
	return -1
}

// ExecRemoteCommand takes a ssh client connection and a command to run and executes the
// command on the specified client. The function returns the Stdout from the client or the Stderr
func ExecRemoteCommand(dial *ssh.Client, run string) ([]byte, error) {
	sess, err := dial.NewSession() // new ssh client session
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	var buffer bytes.Buffer
	var bufferErr bytes.Buffer
	sess.Stdout = &buffer                 // output from client funneled into buffer
	sess.Stderr = &bufferErr              // err form client funneled into buffer
	if err := sess.Run(run); err != nil { // run the command on the ssh client
		return nil, fmt.Errorf("%v: %w", bufferErr.String(), err)
	}
	return buffer.Bytes(), nil
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

// ValidateAndConfigure will take a ssh url and an identity key (rsa and the like) and ensure the information given is valid
// iden iden can be blank to mean no identity key
// once the function validates the information it creates and returns an ssh.ClientConfig.
func ValidateAndConfigure(uri *url.URL, iden string) (*ssh.ClientConfig, error) {
	var signers []ssh.Signer
	passwd, passwdSet := uri.User.Password()
	if iden != "" { // iden might be blank if coming from image scp or if no validation is needed
		value := iden
		s, err := terminal.PublicKey(value, []byte(passwd))
		if err != nil {
			return nil, fmt.Errorf("failed to read identity %q: %w", value, err)
		}
		signers = append(signers, s)
		logrus.Debugf("SSH Ident Key %q %s %s", value, ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
	}
	if sock, found := os.LookupEnv("SSH_AUTH_SOCK"); found { // validate ssh information, specifically the unix file socket used by the ssh agent.
		logrus.Debugf("Found SSH_AUTH_SOCK %q, ssh-agent signer enabled", sock)

		c, err := net.Dial("unix", sock)
		if err != nil {
			return nil, err
		}
		agentSigners, err := agent.NewClient(c).Signers()
		if err != nil {
			return nil, err
		}

		signers = append(signers, agentSigners...)

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			for _, s := range agentSigners {
				logrus.Debugf("SSH Agent Key %s %s", ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
			}
		}
	}
	var authMethods []ssh.AuthMethod // now we validate and check for the authorization methods, most notaibly public key authorization
	if len(signers) > 0 {
		var dedup = make(map[string]ssh.Signer)
		for _, s := range signers {
			fp := ssh.FingerprintSHA256(s.PublicKey())
			if _, found := dedup[fp]; found {
				logrus.Debugf("Dedup SSH Key %s %s", ssh.FingerprintSHA256(s.PublicKey()), s.PublicKey().Type())
			}
			dedup[fp] = s
		}

		var uniq []ssh.Signer
		for _, s := range dedup {
			uniq = append(uniq, s)
		}
		authMethods = append(authMethods, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			return uniq, nil
		}))
	}
	if passwdSet { // if password authentication is given and valid, add to the list
		authMethods = append(authMethods, ssh.Password(passwd))
	}
	if len(authMethods) == 0 {
		authMethods = append(authMethods, ssh.PasswordCallback(func() (string, error) {
			pass, err := terminal.ReadPassword(fmt.Sprintf("%s's login password:", uri.User.Username()))
			return string(pass), err
		}))
	}
	tick, err := time.ParseDuration("40s")
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            uri.User.Username(),
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         tick,
	}
	return cfg, nil
}
