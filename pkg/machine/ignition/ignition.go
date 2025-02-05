//go:build amd64 || arm64

package ignition

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/systemd/parser"
	"github.com/containers/storage/pkg/fileutils"
	"github.com/sirupsen/logrus"
)

/*
	If this file gets too nuts, we can perhaps use existing go code
	to create ignition files.  At this point, the file is so simple
	that I chose to use structs and not import any code as I was
	concerned (unsubstantiated) about too much bloat coming in.

	https://github.com/openshift/machine-config-operator/blob/master/pkg/server/server.go
*/

const (
	PodmanDockerTmpConfPath = "/etc/tmpfiles.d/podman-docker.conf"
	DefaultIgnitionUserName = "core"
)

// Convenience function to convert int to ptr
func IntToPtr(i int) *int {
	return &i
}

// Convenience function to convert string to ptr
func StrToPtr(s string) *string {
	return &s
}

// Convenience function to convert bool to ptr
func BoolToPtr(b bool) *bool {
	return &b
}

func GetNodeUsr(usrName string) NodeUser {
	return NodeUser{Name: &usrName}
}

func GetNodeGrp(grpName string) NodeGroup {
	return NodeGroup{Name: &grpName}
}

type DynamicIgnition struct {
	Name       string
	Key        string
	TimeZone   string
	UID        int
	VMName     string
	VMType     define.VMType
	WritePath  string
	Cfg        Config
	Rootful    bool
	NetRecover bool
	Rosetta    bool
}

func (ign *DynamicIgnition) Write() error {
	b, err := json.Marshal(ign.Cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ign.WritePath, b, 0644)
}

func (ign *DynamicIgnition) getUsers() []PasswdUser {
	var (
		users []PasswdUser
	)

	isCoreUser := ign.Name == DefaultIgnitionUserName

	// if we are not using the 'core' user, we need to tell ignition to
	// not add it
	if !isCoreUser {
		coreUser := PasswdUser{
			Name:        DefaultIgnitionUserName,
			ShouldExist: BoolToPtr(false),
		}
		users = append(users, coreUser)
	}

	// Adding the user
	user := PasswdUser{
		Name:              ign.Name,
		SSHAuthorizedKeys: []SSHAuthorizedKey{SSHAuthorizedKey(ign.Key)},
		UID:               IntToPtr(ign.UID),
	}

	// If we are not using the core user, we need to make the user part
	// of the following groups
	if !isCoreUser {
		user.Groups = []Group{
			Group("sudo"),
			Group("adm"),
			Group("wheel"),
			Group("systemd-journal")}
	}

	// set root SSH key
	root := PasswdUser{
		Name:              "root",
		SSHAuthorizedKeys: []SSHAuthorizedKey{SSHAuthorizedKey(ign.Key)},
	}
	// add them all in
	users = append(users, user, root)

	return users
}

// GenerateIgnitionConfig
func (ign *DynamicIgnition) GenerateIgnitionConfig() error {
	if len(ign.Name) < 1 {
		ign.Name = DefaultIgnitionUserName
	}
	ignVersion := Ignition{
		Version: "3.2.0",
	}
	ignPassword := Passwd{
		Users: ign.getUsers(),
	}

	ignStorage := Storage{
		Directories: getDirs(ign.Name),
		Files:       getFiles(ign.Name, ign.UID, ign.Rootful, ign.VMType, ign.NetRecover),
		Links:       getLinks(ign.Name),
	}

	// Add or set the time zone for the machine
	if len(ign.TimeZone) > 0 {
		var (
			err error
			tz  string
		)
		// local means the same as the host
		// look up where it is pointing to on the host
		if ign.TimeZone == "local" {
			tz, err = getLocalTimeZone()
			if err != nil {
				return err
			}
		} else {
			tz = ign.TimeZone
		}
		tzLink := Link{
			Node: Node{
				Group:     GetNodeGrp("root"),
				Path:      "/etc/localtime",
				Overwrite: BoolToPtr(false),
				User:      GetNodeUsr("root"),
			},
			LinkEmbedded1: LinkEmbedded1{
				Hard: BoolToPtr(false),
				// We always want this value in unix form (/path/to/something) because this is being
				// set in the machine OS (always Linux).  However, filepath.join on windows will use a "\\"
				// separator; therefore we use ToSlash to convert the path to unix style
				Target: filepath.ToSlash(filepath.Join("/usr/share/zoneinfo", tz)),
			},
		}
		ignStorage.Links = append(ignStorage.Links, tzLink)
	}

	// This service gets environment variables that are provided
	// through qemu fw_cfg and then sets them into systemd/system.conf.d,
	// profile.d and environment.d files
	//
	// Currently, it is used for propagating
	// proxy settings e.g. HTTP_PROXY and others, on a start avoiding
	// a need of re-creating/re-initiating a VM

	envset := parser.NewUnitFile()
	envset.Add("Unit", "Description", "Environment setter from QEMU FW_CFG")

	envset.Add("Service", "Type", "oneshot")
	envset.Add("Service", "RemainAfterExit", "yes")
	envset.Add("Service", "Environment", "FWCFGRAW=/sys/firmware/qemu_fw_cfg/by_name/opt/com.coreos/environment/raw")
	envset.Add("Service", "Environment", "SYSTEMD_CONF=/etc/systemd/system.conf.d/default-env.conf")
	envset.Add("Service", "Environment", "ENVD_CONF=/etc/environment.d/default-env.conf")
	envset.Add("Service", "Environment", "PROFILE_CONF=/etc/profile.d/default-env.sh")
	envset.Add("Service", "ExecStart", `/usr/bin/bash -c '/usr/bin/test -f ${FWCFGRAW} &&\
        echo "[Manager]\n#Got from QEMU FW_CFG\nDefaultEnvironment=$(/usr/bin/base64 -d ${FWCFGRAW} | sed -e "s+|+ +g")\n" > ${SYSTEMD_CONF} ||\
        echo "[Manager]\n#Got nothing from QEMU FW_CFG\n#DefaultEnvironment=\n" > ${SYSTEMD_CONF}'`)
	envset.Add("Service", "ExecStart", `/usr/bin/bash -c '/usr/bin/test -f ${FWCFGRAW} && (\
        echo "#Got from QEMU FW_CFG"> ${ENVD_CONF};\
        IFS="|";\
        for iprxy in $(/usr/bin/base64 -d ${FWCFGRAW}); do\
            echo "$iprxy" >> ${ENVD_CONF}; done ) || \
        echo "#Got nothing from QEMU FW_CFG"> ${ENVD_CONF}'`)
	envset.Add("Service", "ExecStart", `/usr/bin/bash -c '/usr/bin/test -f ${FWCFGRAW} && (\
        echo "#Got from QEMU FW_CFG"> ${PROFILE_CONF};\
        IFS="|";\
        for iprxy in $(/usr/bin/base64 -d ${FWCFGRAW}); do\
            echo "export $iprxy" >> ${PROFILE_CONF}; done ) || \
        echo "#Got nothing from QEMU FW_CFG"> ${PROFILE_CONF}'`)
	envset.Add("Service", "ExecStartPost", "/usr/bin/systemctl daemon-reload")

	envset.Add("Install", "WantedBy", "sysinit.target")
	envsetFile, err := envset.ToString()
	if err != nil {
		return err
	}

	ignSystemd := Systemd{
		Units: []Unit{
			{
				Enabled: BoolToPtr(true),
				Name:    "podman.socket",
			},
			{
				// TODO Need to understand if this could play a role in machine
				// updates given a certain configuration
				// Disable auto-updating of fcos images
				// https://github.com/containers/podman/issues/20122
				Enabled: BoolToPtr(false),
				Name:    "zincati.service",
			},
		},
	}

	// Only qemu has the qemu firmware environment setting
	if ign.VMType == define.QemuVirt {
		qemuUnit := Unit{
			Enabled:  BoolToPtr(true),
			Name:     "envset-fwcfg.service",
			Contents: &envsetFile,
		}
		ignSystemd.Units = append(ignSystemd.Units, qemuUnit)
	}

	// Only AppleHv with Apple Silicon can use Rosetta
	if ign.VMType == define.AppleHvVirt && runtime.GOARCH == "arm64" {
		rosettaUnit := Systemd{
			Units: []Unit{
				{
					Enabled: BoolToPtr(true),
					Name:    "rosetta-activation.service",
				},
			},
		}
		ignSystemd.Units = append(ignSystemd.Units, rosettaUnit.Units...)
	}

	// Only after all checks are done
	// it's ready create the ingConfig
	ign.Cfg = Config{
		Ignition: ignVersion,
		Passwd:   ignPassword,
		Storage:  ignStorage,
		Systemd:  ignSystemd,
	}
	return nil
}

func getDirs(usrName string) []Directory {
	// Ignition has a bug/feature? where if you make a series of dirs
	// in one swoop, then the leading dirs are creates as root.
	newDirs := []string{
		"/home/" + usrName + "/.config",
		"/home/" + usrName + "/.config/containers",
		"/home/" + usrName + "/.config/systemd",
		"/home/" + usrName + "/.config/systemd/user",
		"/home/" + usrName + "/.config/systemd/user/default.target.wants",
	}
	var (
		dirs = make([]Directory, len(newDirs))
	)
	for i, d := range newDirs {
		newDir := Directory{
			Node: Node{
				Group: GetNodeGrp(usrName),
				Path:  d,
				User:  GetNodeUsr(usrName),
			},
			DirectoryEmbedded1: DirectoryEmbedded1{Mode: IntToPtr(0755)},
		}
		dirs[i] = newDir
	}

	return dirs
}

func getFiles(usrName string, uid int, rootful bool, vmtype define.VMType, _ bool) []File {
	files := make([]File, 0)

	lingerExample := parser.NewUnitFile()
	lingerExample.Add("Unit", "Description", "A systemd user unit demo")
	lingerExample.Add("Unit", "After", "network-online.target")
	lingerExample.Add("Unit", "Wants", "network-online.target podman.socket")
	lingerExample.Add("Service", "ExecStart", "/usr/bin/sleep infinity")
	lingerExampleFile, err := lingerExample.ToString()
	if err != nil {
		logrus.Warn(err.Error())
	}

	containers := `[containers]
netns="bridge"
pids_limit=0
`
	// TODO I think this can be removed but leaving breadcrumb until certain.
	// Set deprecated machine_enabled until podman package on fcos is
	// current enough to no longer require it
	// 	rootContainers := `[engine]
	// machine_enabled=true
	// `
	// Prevent subUID from clashing with actual UID
	subUID := 100000
	subUIDs := 1000000
	if uid >= subUID && uid < (subUID+subUIDs) {
		subUID = uid + 1
	}
	etcSubUID := fmt.Sprintf(`%s:%d:%d`, usrName, subUID, subUIDs)

	// Add a fake systemd service to get the user socket rolling
	files = append(files, File{
		Node: Node{
			Group: GetNodeGrp(usrName),
			Path:  "/home/" + usrName + "/.config/systemd/user/linger-example.service",
			User:  GetNodeUsr(usrName),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: EncodeDataURLPtr(lingerExampleFile),
			},
			Mode: IntToPtr(0744),
		},
	})

	// Set containers.conf up for core user to use networks
	// by default
	files = append(files, File{
		Node: Node{
			Group: GetNodeGrp(usrName),
			Path:  "/home/" + usrName + "/.config/containers/containers.conf",
			User:  GetNodeUsr(usrName),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: EncodeDataURLPtr(containers),
			},
			Mode: IntToPtr(0744),
		},
	})

	// Set up /etc/subuid and /etc/subgid
	for _, sub := range []string{"/etc/subuid", "/etc/subgid"} {
		files = append(files, File{
			Node: Node{
				Group:     GetNodeGrp("root"),
				Path:      sub,
				User:      GetNodeUsr("root"),
				Overwrite: BoolToPtr(true),
			},
			FileEmbedded1: FileEmbedded1{
				Append: nil,
				Contents: Resource{
					Source: EncodeDataURLPtr(etcSubUID),
				},
				Mode: IntToPtr(0744),
			},
		})
	}

	// Set machine marker file to indicate podman what vmtype we are
	// operating under
	files = append(files, File{
		Node: Node{
			Group: GetNodeGrp("root"),
			Path:  "/etc/containers/podman-machine",
			User:  GetNodeUsr("root"),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: EncodeDataURLPtr(fmt.Sprintf("%s\n", vmtype.String())),
			},
			Mode: IntToPtr(0644),
		},
	})

	files = append(files, File{
		Node: Node{
			Path: PodmanDockerTmpConfPath,
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			// Create a symlink from the docker socket to the podman socket.
			Contents: Resource{
				Source: EncodeDataURLPtr(GetPodmanDockerTmpConfig(uid, rootful, true)),
			},
			Mode: IntToPtr(0644),
		},
	})

	// get certs for current user
	userHome, err := os.UserHomeDir()
	if err != nil {
		logrus.Warnf("Unable to copy certs via ignition %s", err.Error())
		return files
	}

	certFiles := getCerts(filepath.Join(userHome, ".config/containers/certs.d"), true)
	files = append(files, certFiles...)

	certFiles = getCerts(filepath.Join(userHome, ".config/docker/certs.d"), true)
	files = append(files, certFiles...)

	sslCertFileName, ok := os.LookupEnv(sslCertFile)
	if ok {
		if err := fileutils.Exists(sslCertFileName); err == nil {
			certFiles = getCerts(sslCertFileName, false)
			files = append(files, certFiles...)
		} else {
			logrus.Warnf("Invalid path in %s: %q", sslCertFile, err)
		}
	}

	sslCertDirName, ok := os.LookupEnv(sslCertDir)
	if ok {
		if err := fileutils.Exists(sslCertDirName); err == nil {
			certFiles = getCerts(sslCertDirName, true)
			files = append(files, certFiles...)
		} else {
			logrus.Warnf("Invalid path in %s: %q", sslCertDir, err)
		}
	}
	if sslCertFileName != "" || sslCertDirName != "" {
		// If we copied certs via env then also make the to set the env in the VM.
		files = append(files, getSSLEnvironmentFiles(sslCertFileName, sslCertDirName)...)
	}

	return files
}

func getCerts(certsDir string, isDir bool) []File {
	var (
		files []File
	)

	if isDir {
		err := filepath.WalkDir(certsDir, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				certPath, err := filepath.Rel(certsDir, path)
				if err != nil {
					logrus.Warnf("%s", err)
					return nil
				}

				file, err := prepareCertFile(filepath.Join(certsDir, certPath), certPath)
				if err == nil {
					files = append(files, file)
				}
			}

			return nil
		})
		if err != nil {
			if !os.IsNotExist(err) {
				logrus.Warnf("Unable to copy certs via ignition, error while reading certs from %s:  %s", certsDir, err.Error())
			}
		}
	} else {
		fileName := filepath.Base(certsDir)
		file, err := prepareCertFile(certsDir, fileName)
		if err == nil {
			files = append(files, file)
		}
	}

	return files
}

func prepareCertFile(fpath string, name string) (File, error) {
	b, err := os.ReadFile(fpath)
	if err != nil {
		logrus.Warnf("Unable to read cert file %v", err)
		return File{}, err
	}

	// Note path is required here as we always create a path for the linux VM
	// even when the client run on windows so we cannot use filepath.
	targetPath := path.Join(define.UserCertsTargetPath, name)

	logrus.Debugf("Copying cert file from '%s' to '%s'.", fpath, targetPath)

	file := File{
		Node: Node{
			Group: GetNodeGrp("root"),
			Path:  targetPath,
			User:  GetNodeUsr("root"),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: EncodeDataURLPtr(string(b)),
			},
			Mode: IntToPtr(0644),
		},
	}
	return file, nil
}

const (
	systemdSSLConf = "/etc/systemd/system.conf.d/podman-machine-ssl.conf"
	envdSSLConf    = "/etc/environment.d/podman-machine-ssl.conf"
	profileSSLConf = "/etc/profile.d/podman-machine-ssl.sh"
	sslCertFile    = "SSL_CERT_FILE"
	sslCertDir     = "SSL_CERT_DIR"
)

func getSSLEnvironmentFiles(sslFileName, sslDirName string) []File {
	systemdFileContent := "[Manager]\n"
	envdFileContent := ""
	profileFileContent := ""
	if sslFileName != "" {
		// certs are written to UserCertsTargetPath see prepareCertFile()
		// Note the mix of path/filepath is intentional and required, we want to get the name of
		// a path on the client (i.e. windows) but then join to linux path that will be used inside the VM.
		env := fmt.Sprintf("%s=%q\n", sslCertFile, path.Join(define.UserCertsTargetPath, filepath.Base(sslFileName)))
		systemdFileContent += "DefaultEnvironment=" + env
		envdFileContent += env
		profileFileContent += "export " + env
	}
	if sslDirName != "" {
		// certs are written to UserCertsTargetPath see prepareCertFile()
		env := fmt.Sprintf("%s=%q\n", sslCertDir, define.UserCertsTargetPath)
		systemdFileContent += "DefaultEnvironment=" + env
		envdFileContent += env
		profileFileContent += "export " + env
	}
	return []File{
		getSSLFile(systemdSSLConf, systemdFileContent),
		getSSLFile(envdSSLConf, envdFileContent),
		getSSLFile(profileSSLConf, profileFileContent),
	}
}

func getSSLFile(path, content string) File {
	return File{
		Node: Node{
			Group: GetNodeGrp("root"),
			Path:  path,
			User:  GetNodeUsr("root"),
		},
		FileEmbedded1: FileEmbedded1{
			Contents: Resource{
				Source: EncodeDataURLPtr(content),
			},
			Mode: IntToPtr(0644),
		},
	}
}

func getLinks(usrName string) []Link {
	return []Link{{
		Node: Node{
			Group: GetNodeGrp(usrName),
			Path:  "/home/" + usrName + "/.config/systemd/user/default.target.wants/linger-example.service",
			User:  GetNodeUsr(usrName),
		},
		LinkEmbedded1: LinkEmbedded1{
			Hard:   BoolToPtr(false),
			Target: "/home/" + usrName + "/.config/systemd/user/linger-example.service",
		},
	}, {
		Node: Node{
			Group:     GetNodeGrp("root"),
			Path:      "/usr/local/bin/docker",
			Overwrite: BoolToPtr(true),
			User:      GetNodeUsr("root"),
		},
		LinkEmbedded1: LinkEmbedded1{
			Hard:   BoolToPtr(false),
			Target: "/usr/bin/podman",
		},
	}}
}

func EncodeDataURLPtr(contents string) *string {
	return StrToPtr(fmt.Sprintf("data:,%s", url.PathEscape(contents)))
}

func GetPodmanDockerTmpConfig(uid int, rootful bool, newline bool) string {
	// Derived from https://github.com/containers/podman/blob/main/contrib/systemd/system/podman-docker.conf
	podmanSock := "/run/podman/podman.sock"
	if !rootful {
		podmanSock = fmt.Sprintf("/run/user/%d/podman/podman.sock", uid)
	}
	suffix := ""
	if newline {
		suffix = "\n"
	}

	return fmt.Sprintf("L+  /run/docker.sock   -    -    -     -   %s%s", podmanSock, suffix)
}

// SetIgnitionFile creates a new Machine File for the machine's ignition file
// and assigns the handle to `loc`
func SetIgnitionFile(loc *define.VMFile, vmtype define.VMType, vmName, vmConfigDir string) error {
	ignitionFile, err := define.NewMachineFile(filepath.Join(vmConfigDir, vmName+".ign"), nil)
	if err != nil {
		return err
	}

	*loc = *ignitionFile
	return nil
}

type IgnitionBuilder struct {
	dynamicIgnition DynamicIgnition
	units           []Unit
}

// NewIgnitionBuilder generates a new IgnitionBuilder type using the
// base `DynamicIgnition` object
func NewIgnitionBuilder(dynamicIgnition DynamicIgnition) IgnitionBuilder {
	return IgnitionBuilder{
		dynamicIgnition,
		[]Unit{},
	}
}

// GenerateIgnitionConfig generates the ignition config
func (i *IgnitionBuilder) GenerateIgnitionConfig() error {
	return i.dynamicIgnition.GenerateIgnitionConfig()
}

// WithUnit adds systemd units to the internal `DynamicIgnition` config
func (i *IgnitionBuilder) WithUnit(units ...Unit) {
	i.dynamicIgnition.Cfg.Systemd.Units = append(i.dynamicIgnition.Cfg.Systemd.Units, units...)
}

// WithFile adds storage files to the internal `DynamicIgnition` config
func (i *IgnitionBuilder) WithFile(files ...File) {
	i.dynamicIgnition.Cfg.Storage.Files = append(i.dynamicIgnition.Cfg.Storage.Files, files...)
}

// BuildWithIgnitionFile copies the provided ignition file into the internal
// `DynamicIgnition` write path
func (i *IgnitionBuilder) BuildWithIgnitionFile(ignPath string) error {
	inputIgnition, err := os.ReadFile(ignPath)
	if err != nil {
		return err
	}

	return os.WriteFile(i.dynamicIgnition.WritePath, inputIgnition, 0644)
}

// Build writes the internal `DynamicIgnition` config to its write path
func (i *IgnitionBuilder) Build() error {
	logrus.Debugf("writing ignition file to %q", i.dynamicIgnition.WritePath)
	return i.dynamicIgnition.Write()
}

func GetNetRecoveryFile() string {
	return `#!/bin/bash
# Verify network health, and bounce the network device if host connectivity
# is lost. This is a temporary workaround for a known rare qemu/virtio issue
# that affects some systems

sleep 120 # allow time for network setup on initial boot
while true; do
  sleep 30
  curl -s -o /dev/null --max-time 30 http://192.168.127.1/health
  if [ "$?" != "0" ]; then
    echo "bouncing nic due to loss of connectivity with host"
    ifconfig enp0s1 down; ifconfig enp0s1 up
  fi
done
`
}

func (i *IgnitionBuilder) AddPlaybook(contents string, destPath string, username string) error {
	// create the ignition file object
	f := File{
		Node: Node{
			Group: GetNodeGrp(username),
			Path:  destPath,
			User:  GetNodeUsr(username),
		},
		FileEmbedded1: FileEmbedded1{
			Append: nil,
			Contents: Resource{
				Source: EncodeDataURLPtr(contents),
			},
			Mode: IntToPtr(0744),
		},
	}

	// call ignitionBuilder.WithFile
	// add the config file to the ignition object
	i.WithFile(f)

	unit := parser.NewUnitFile()
	unit.Add("Unit", "After", "ready.service")
	unit.Add("Unit", "ConditionFirstBoot", "yes")
	unit.Add("Service", "Type", "oneshot")
	unit.Add("Service", "User", username)
	unit.Add("Service", "Group", username)
	unit.Add("Service", "ExecStart", fmt.Sprintf("ansible-playbook %s", destPath))
	unit.Add("Install", "WantedBy", "default.target")
	unitContents, err := unit.ToString()
	if err != nil {
		return err
	}

	// create a systemd service
	playbookUnit := Unit{
		Enabled:  BoolToPtr(true),
		Name:     "playbook.service",
		Contents: &unitContents,
	}
	i.WithUnit(playbookUnit)

	return nil
}

func GetNetRecoveryUnitFile() *parser.UnitFile {
	recoveryUnit := parser.NewUnitFile()
	recoveryUnit.Add("Unit", "Description", "Verifies health of network and recovers if necessary")
	recoveryUnit.Add("Unit", "After", "sshd.socket sshd.service")
	recoveryUnit.Add("Service", "ExecStart", "/usr/local/bin/net-health-recovery.sh")
	recoveryUnit.Add("Service", "StandardOutput", "journal")
	recoveryUnit.Add("Service", "StandardError", "journal")
	recoveryUnit.Add("Service", "StandardInput", "null")
	recoveryUnit.Add("Install", "WantedBy", "default.target")

	return recoveryUnit
}

func DefaultReadyUnitFile() parser.UnitFile {
	u := parser.NewUnitFile()
	u.Add("Unit", "After", "sshd.socket sshd.service")
	u.Add("Unit", "OnFailure", "emergency.target")
	u.Add("Unit", "OnFailureJobMode", "isolate")
	u.Add("Service", "Type", "oneshot")
	u.Add("Service", "RemainAfterExit", "yes")
	u.Add("Install", "RequiredBy", "default.target")
	return *u
}
