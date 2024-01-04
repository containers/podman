//go:build windows

package wsl

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containers/common/pkg/config"
	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/define"
	"github.com/containers/podman/v4/pkg/machine/vmconfigs"
	"github.com/containers/podman/v4/pkg/machine/wsl/wutil"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/podman/v4/utils"
	"github.com/containers/storage/pkg/homedir"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	// vmtype refers to qemu (vs libvirt, krun, etc)
	vmtype = machine.WSLVirt
)

const (
	ErrorSuccessRebootInitiated = 1641
	ErrorSuccessRebootRequired  = 3010
	currentMachineVersion       = 3
)

const containersConf = `[containers]

[engine]
cgroup_manager = "cgroupfs"
`

const registriesConf = `unqualified-search-registries=["docker.io"]
`

const appendPort = `grep -q Port\ %d /etc/ssh/sshd_config || echo Port %d >> /etc/ssh/sshd_config`

const changePort = `sed -E -i 's/^Port[[:space:]]+[0-9]+/Port %d/' /etc/ssh/sshd_config`

const configServices = `ln -fs /usr/lib/systemd/system/sshd.service /etc/systemd/system/multi-user.target.wants/sshd.service
ln -fs /usr/lib/systemd/system/podman.socket /etc/systemd/system/sockets.target.wants/podman.socket
rm -f /etc/systemd/system/getty.target.wants/console-getty.service
rm -f /etc/systemd/system/getty.target.wants/getty@tty1.service
rm -f /etc/systemd/system/multi-user.target.wants/systemd-resolved.service
rm -f /etc/systemd/system/sysinit.target.wants//systemd-resolved.service
rm -f /etc/systemd/system/dbus-org.freedesktop.resolve1.service
ln -fs /dev/null /etc/systemd/system/console-getty.service
ln -fs /dev/null /etc/systemd/system/systemd-oomd.socket
mkdir -p /etc/systemd/system/systemd-sysusers.service.d/
echo CREATE_MAIL_SPOOL=no >> /etc/default/useradd
adduser -m [USER] -G wheel
mkdir -p /home/[USER]/.config/systemd/[USER]/
chown [USER]:[USER] /home/[USER]/.config
`

const sudoers = `%wheel        ALL=(ALL)       NOPASSWD: ALL
`

const bootstrap = `#!/bin/bash
ps -ef | grep -v grep | grep -q systemd && exit 0
nohup unshare --kill-child --fork --pid --mount --mount-proc --propagation shared /lib/systemd/systemd >/dev/null 2>&1 &
sleep 0.1
`

const wslmotd = `
You will be automatically entered into a nested process namespace where
systemd is running. If you need to access the parent namespace, hit ctrl-d
or type exit. This also means to log out you need to exit twice.

`

const sysdpid = "SYSDPID=`ps -eo cmd,pid | grep -m 1 ^/lib/systemd/systemd | awk '{print $2}'`"

const profile = sysdpid + `
if [ ! -z "$SYSDPID" ] && [ "$SYSDPID" != "1" ]; then
    cat /etc/wslmotd
	/usr/local/bin/enterns
fi
`

const enterns = "#!/bin/bash\n" + sysdpid + `
if [ ! -z "$SYSDPID" ] && [ "$SYSDPID" != "1" ]; then
        NSENTER=("nsenter" "-m" "-p" "-t" "$SYSDPID" "--wd=$PWD")

        if [ "$UID" != "0" ]; then
                NSENTER=("sudo" "${NSENTER[@]}")
                if [ "$#" != "0" ]; then
                        NSENTER+=("sudo" "-u" "$USER")
                else
                        NSENTER+=("su" "-l" "$USER")
                fi
        fi
        "${NSENTER[@]}" "$@"
fi`

const waitTerm = sysdpid + `
if [ ! -z "$SYSDPID" ]; then
	timeout 60 tail -f /dev/null --pid $SYSDPID
fi
`

const wslConf = `[user]
default=[USER]
`

const wslConfUserNet = `
[network]
generateResolvConf = false
`

const resolvConfUserNet = `
nameserver 192.168.127.1
`

// WSL kernel does not have sg and crypto_user modules
const overrideSysusers = `[Service]
LoadCredential=
`

const lingerService = `[Unit]
Description=A systemd user unit demo
After=network-online.target
Wants=network-online.target podman.socket
[Service]
ExecStart=/usr/bin/sleep infinity
`

const lingerSetup = `mkdir -p /home/[USER]/.config/systemd/user/default.target.wants
ln -fs /home/[USER]/.config/systemd/user/linger-example.service \
       /home/[USER]/.config/systemd/user/default.target.wants/linger-example.service
`

const bindMountSystemService = `
[Unit]
Description=Bind mount for system podman sockets
After=podman.socket

[Service]
RemainAfterExit=true
Type=oneshot
# Ensure user services can register sockets as well
ExecStartPre=mkdir -p -m 777 /mnt/wsl/podman-sockets
ExecStartPre=mkdir -p -m 777 /mnt/wsl/podman-sockets/%[1]s
ExecStartPre=touch /mnt/wsl/podman-sockets/%[1]s/podman-root.sock
ExecStart=mount --bind %%t/podman/podman.sock /mnt/wsl/podman-sockets/%[1]s/podman-root.sock
ExecStop=umount /mnt/wsl/podman-sockets/%[1]s/podman-root.sock
`

const bindMountUserService = `
[Unit]
Description=Bind mount for user podman sockets
After=podman.socket

[Service]
RemainAfterExit=true
Type=oneshot
# Consistency with system service (supports racing)
ExecStartPre=mkdir -p -m 777 /mnt/wsl/podman-sockets
ExecStartPre=mkdir -p -m 777 /mnt/wsl/podman-sockets/%[1]s
ExecStartPre=touch /mnt/wsl/podman-sockets/%[1]s/podman-user.sock
# Relies on /etc/fstab entry for user mounting
ExecStart=mount /mnt/wsl/podman-sockets/%[1]s/podman-user.sock
ExecStop=umount /mnt/wsl/podman-sockets/%[1]s/podman-user.sock
`

const bindMountFsTab = `/run/user/1000/podman/podman.sock /mnt/wsl/podman-sockets/%s/podman-user.sock none noauto,user,bind,defaults 0 0
`
const (
	defaultTargetWants     = "default.target.wants"
	userSystemdPath        = "/home/%[1]s/.config/systemd/user"
	sysSystemdPath         = "/etc/systemd/system"
	userSystemdWants       = userSystemdPath + "/" + defaultTargetWants
	sysSystemdWants        = sysSystemdPath + "/" + defaultTargetWants
	bindUnitFileName       = "podman-mnt-bindings.service"
	bindUserUnitPath       = userSystemdPath + "/" + bindUnitFileName
	bindUserUnitWant       = userSystemdWants + "/" + bindUnitFileName
	bindSysUnitPath        = sysSystemdPath + "/" + bindUnitFileName
	bindSysUnitWant        = sysSystemdWants + "/" + bindUnitFileName
	podmanSocketDropin     = "podman.socket.d"
	podmanSocketDropinPath = sysSystemdPath + "/" + podmanSocketDropin
)

const configBindServices = "mkdir -p " + userSystemdWants + " " + sysSystemdWants + " " + podmanSocketDropinPath + "\n" +
	"ln -fs " + bindUserUnitPath + " " + bindUserUnitWant + "\n" +
	"ln -fs " + bindSysUnitPath + " " + bindSysUnitWant + "\n"

const overrideSocketGroup = `
[Socket]
SocketMode=0660
SocketGroup=wheel
`

const proxyConfigSetup = `#!/bin/bash

SYSTEMD_CONF=/etc/systemd/system.conf.d/default-env.conf
ENVD_CONF=/etc/environment.d/default-env.conf
PROFILE_CONF=/etc/profile.d/default-env.sh

IFS="|"
read proxies

mkdir -p /etc/profile.d /etc/environment.d /etc/systemd/system.conf.d/
rm -f $SYSTEMD_CONF
for proxy in $proxies; do
	output+="$proxy "
done
echo "[Manager]" >> $SYSTEMD_CONF
echo -ne "DefaultEnvironment=" >> $SYSTEMD_CONF

echo $output >> $SYSTEMD_CONF
rm -f $ENVD_CONF
for proxy in $proxies; do
	echo "$proxy" >> $ENVD_CONF
done
rm -f $PROFILE_CONF
for proxy in $proxies; do
	echo "export $proxy" >> $PROFILE_CONF
done
`

const proxyConfigAttempt = `if [ -f /usr/local/bin/proxyinit ]; \
then /usr/local/bin/proxyinit; \
else exit 42; \
fi`

const clearProxySettings = `rm -f /etc/systemd/system.conf.d/default-env.conf \
	   /etc/environment.d/default-env.conf \
	   /etc/profile.d/default-env.sh`

const wslInstallError = `Could not %s. See previous output for any potential failure details.
If you can not resolve the issue, and rerunning fails, try the "wsl --install" process
outlined in the following article:

http://docs.microsoft.com/en-us/windows/wsl/install

`

const wslKernelError = `Could not %s. See previous output for any potential failure details.
If you can not resolve the issue, try rerunning the "podman machine init command". If that fails
try the "wsl --update" command and then rerun "podman machine init". Finally, if all else fails,
try following the steps outlined in the following article:

http://docs.microsoft.com/en-us/windows/wsl/install

`

const wslInstallKernel = "install the WSL Kernel"

const wslOldVersion = `Automatic installation of WSL can not be performed on this version of Windows
Either update to Build 19041 (or later), or perform the manual installation steps
outlined in the following article:

http://docs.microsoft.com/en-us/windows/wsl/install\

`

const (
	gvProxy        = "gvproxy.exe"
	winSShProxy    = "win-sshproxy.exe"
	winSshProxyTid = "win-sshproxy.tid"
	pipePrefix     = "npipe:////./pipe/"
	globalPipe     = "docker_engine"
	userModeDist   = "podman-net-usermode"
	rootfulSock    = "/run/podman/podman.sock"
	rootlessSock   = "/run/user/1000/podman/podman.sock"
)

type MachineVM struct {
	// ConfigPath is the path to the configuration file
	ConfigPath string
	// Created contains the original created time instead of querying the file mod time
	Created time.Time
	// ImageStream is the version of fcos being used
	ImageStream string
	// ImagePath is the fq path to
	ImagePath string
	// LastUp contains the last recorded uptime
	LastUp time.Time
	// Name of the vm
	Name string
	// Whether this machine should run in a rootful or rootless manner
	Rootful bool
	// SSH identity, username, etc
	vmconfigs.SSHConfig
	// machine version
	Version int
	// Whether to use user-mode networking
	UserModeNetworking bool
	// Used at runtime for serializing write operations
	lock *lockfile.LockFile
}

type ExitCodeError struct {
	code uint
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("Process failed with exit code: %d", e.code)
}

func getConfigPath(name string) (string, error) {
	return getConfigPathExt(name, "json")
}

func getConfigPathExt(name string, extension string) (string, error) {
	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return "", err
	}

	return filepath.Join(vmConfigDir, fmt.Sprintf("%s.%s", name, extension)), nil
}

// readAndMigrate returns the content of the VM's
// configuration file in json
func readAndMigrate(configPath string, name string) (*MachineVM, error) {
	vm := new(MachineVM)
	b, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%v: %w", name, machine.ErrNoSuchVM)
		}
		return vm, err
	}
	err = json.Unmarshal(b, vm)
	if err == nil && vm.Version < currentMachineVersion {
		err = vm.migrateMachine(configPath)
	}

	return vm, err
}

func (v *MachineVM) migrateMachine(configPath string) error {
	if v.Created.IsZero() {
		if err := v.migrate40(configPath); err != nil {
			return err
		}
	}

	// Update older machines to use lingering
	if err := enableUserLinger(v, toDist(v.Name)); err != nil {
		return err
	}

	// Update older machines missing unqualified search config
	if err := configureRegistries(v, toDist(v.Name)); err != nil {
		return err
	}

	v.Version = currentMachineVersion
	return v.writeConfig()
}

func (v *MachineVM) migrate40(configPath string) error {
	v.ConfigPath = configPath
	fi, err := os.Stat(configPath)
	if err != nil {
		return err
	}
	v.Created = fi.ModTime()
	v.LastUp = getLegacyLastStart(v)
	return nil
}

func getLegacyLastStart(vm *MachineVM) time.Time {
	vmDataDir, err := machine.GetDataDir(vmtype)
	if err != nil {
		return vm.Created
	}
	distDir := filepath.Join(vmDataDir, "wsldist")
	start := filepath.Join(distDir, vm.Name, "laststart")
	info, err := os.Stat(start)
	if err != nil {
		return vm.Created
	}
	return info.ModTime()
}

// Init writes the json configuration file to the filesystem for
// other verbs (start, stop)
func (v *MachineVM) Init(opts machine.InitOptions) (bool, error) {
	var (
		err error
	)
	// cleanup half-baked files if init fails at any point
	callbackFuncs := machine.InitCleanup()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	if cont, err := checkAndInstallWSL(opts); !cont {
		appendOutputIfError(opts.ReExec, err)
		return cont, err
	}

	_ = setupWslProxyEnv()
	v.IdentityPath = util.GetIdentityPath(v.Name)
	v.Rootful = opts.Rootful
	v.Version = currentMachineVersion

	if v.UserModeNetworking {
		if err = verifyWSLUserModeCompat(); err != nil {
			return false, err
		}
	}

	if err = downloadDistro(v, opts); err != nil {
		return false, err
	}
	callbackFuncs.Add(v.removeMachineImage)

	const prompt = "Importing operating system into WSL (this may take a few minutes on a new WSL install)..."
	dist, err := provisionWSLDist(v.Name, v.ImagePath, prompt)
	if err != nil {
		return false, err
	}
	callbackFuncs.Add(v.unprovisionWSL)

	if v.UserModeNetworking {
		if err = installUserModeDist(dist, v.ImagePath); err != nil {
			_ = unregisterDist(dist)
			return false, err
		}
	}

	fmt.Println("Configuring system...")
	if err = configureSystem(v, dist); err != nil {
		return false, err
	}

	if err = installScripts(dist); err != nil {
		return false, err
	}

	if err = createKeys(v, dist); err != nil {
		return false, err
	}
	callbackFuncs.Add(v.removeSSHKeys)

	// Cycle so that user change goes into effect
	_ = terminateDist(dist)

	if err = v.writeConfig(); err != nil {
		return false, err
	}
	callbackFuncs.Add(v.removeMachineConfig)

	if err = setupConnections(v, opts); err != nil {
		return false, err
	}
	callbackFuncs.Add(v.removeSystemConnections)
	return true, nil
}

func (v *MachineVM) unprovisionWSL() error {
	if err := terminateDist(toDist(v.Name)); err != nil {
		logrus.Error(err)
	}
	if err := unregisterDist(toDist(v.Name)); err != nil {
		logrus.Error(err)
	}

	vmDataDir, err := machine.GetDataDir(vmtype)
	if err != nil {
		return err
	}
	distDir := filepath.Join(vmDataDir, "wsldist")
	distTarget := filepath.Join(distDir, v.Name)
	return utils.GuardedRemoveAll(distTarget)
}

func (v *MachineVM) removeMachineConfig() error {
	return utils.GuardedRemoveAll(v.ConfigPath)
}

func (v *MachineVM) removeMachineImage() error {
	return utils.GuardedRemoveAll(v.ImagePath)
}

func (v *MachineVM) removeSSHKeys() error {
	if err := utils.GuardedRemoveAll(fmt.Sprintf("%s.pub", v.IdentityPath)); err != nil {
		logrus.Error(err)
	}
	return utils.GuardedRemoveAll(v.IdentityPath)
}

func (v *MachineVM) removeSystemConnections() error {
	return machine.RemoveConnections(v.Name, fmt.Sprintf("%s-root", v.Name))
}

func downloadDistro(v *MachineVM, opts machine.InitOptions) error {
	var (
		dd  machine.DistributionDownload
		err error
	)

	// The default FCOS stream names are reserved indicators for the standard Fedora fetch
	if machine.IsValidFCOSStreamString(opts.ImagePath) {
		v.ImageStream = opts.ImagePath
		dd, err = NewFedoraDownloader(vmtype, v.Name, opts.ImagePath)
	} else {
		v.ImageStream = "custom"
		dd, err = machine.NewGenericDownloader(vmtype, v.Name, opts.ImagePath)
	}
	if err != nil {
		return err
	}

	v.ImagePath = dd.Get().LocalUncompressedFile
	return machine.DownloadImage(dd)
}

func (v *MachineVM) writeConfig() error {
	return machine.WriteConfig(v.ConfigPath, v)
}

func constructSSHUris(v *MachineVM) ([]url.URL, []string) {
	uri := machine.SSHRemoteConnection.MakeSSHURL(machine.LocalhostIP, rootlessSock, strconv.Itoa(v.Port), v.RemoteUsername)
	uriRoot := machine.SSHRemoteConnection.MakeSSHURL(machine.LocalhostIP, rootfulSock, strconv.Itoa(v.Port), "root")

	uris := []url.URL{uri, uriRoot}
	names := []string{v.Name, v.Name + "-root"}

	return uris, names
}

func setupConnections(v *MachineVM, opts machine.InitOptions) error {
	uris, names := constructSSHUris(v)

	// The first connection defined when connections is empty will become the default
	// regardless of IsDefault, so order according to rootful
	if opts.Rootful {
		uris[0], names[0], uris[1], names[1] = uris[1], names[1], uris[0], names[0]
	}

	// We need to prevent racing connection updates to containers.conf globally
	// across all backends to prevent connection overwrites
	flock, err := obtainGlobalConfigLock()
	if err != nil {
		return fmt.Errorf("could not obtain global lock: %w", err)
	}
	defer flock.unlock()

	for i := 0; i < 2; i++ {
		if err := machine.AddConnection(&uris[i], names[i], v.IdentityPath, opts.IsDefault && i == 0); err != nil {
			return err
		}
	}

	return nil
}

func provisionWSLDist(name string, imagePath string, prompt string) (string, error) {
	vmDataDir, err := machine.GetDataDir(vmtype)
	if err != nil {
		return "", err
	}

	distDir := filepath.Join(vmDataDir, "wsldist")
	distTarget := filepath.Join(distDir, name)
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return "", fmt.Errorf("could not create wsldist directory: %w", err)
	}

	dist := toDist(name)
	fmt.Println(prompt)
	if err = runCmdPassThrough("wsl", "--import", dist, distTarget, imagePath, "--version", "2"); err != nil {
		return "", fmt.Errorf("the WSL import of guest OS failed: %w", err)
	}

	// Fixes newuidmap
	if err = wslInvoke(dist, "rpm", "--restore", "shadow-utils"); err != nil {
		return "", fmt.Errorf("package permissions restore of shadow-utils on guest OS failed: %w", err)
	}

	return dist, nil
}

func createKeys(v *MachineVM, dist string) error {
	user := v.RemoteUsername

	if err := terminateDist(dist); err != nil {
		return fmt.Errorf("could not cycle WSL dist: %w", err)
	}

	key, err := wslCreateKeys(v.IdentityPath, dist)
	if err != nil {
		return fmt.Errorf("could not create ssh keys: %w", err)
	}

	if err := wslPipe(key+"\n", dist, "sh", "-c", "mkdir -p /root/.ssh;"+
		"cat >> /root/.ssh/authorized_keys; chmod 600 /root/.ssh/authorized_keys"); err != nil {
		return fmt.Errorf("could not create root authorized keys on guest OS: %w", err)
	}

	userAuthCmd := withUser("mkdir -p /home/[USER]/.ssh;"+
		"cat >> /home/[USER]/.ssh/authorized_keys; chown -R [USER]:[USER] /home/[USER]/.ssh;"+
		"chmod 600 /home/[USER]/.ssh/authorized_keys", user)
	if err := wslPipe(key+"\n", dist, "sh", "-c", userAuthCmd); err != nil {
		return fmt.Errorf("could not create '%s' authorized keys on guest OS: %w", v.RemoteUsername, err)
	}

	return nil
}

func configureSystem(v *MachineVM, dist string) error {
	user := v.RemoteUsername
	if err := wslInvoke(dist, "sh", "-c", fmt.Sprintf(appendPort, v.Port, v.Port)); err != nil {
		return fmt.Errorf("could not configure SSH port for guest OS: %w", err)
	}

	if err := wslPipe(withUser(configServices, user), dist, "sh"); err != nil {
		return fmt.Errorf("could not configure systemd settings for guest OS: %w", err)
	}

	if err := wslPipe(sudoers, dist, "sh", "-c", "cat >> /etc/sudoers"); err != nil {
		return fmt.Errorf("could not add wheel to sudoers: %w", err)
	}

	if err := wslPipe(overrideSysusers, dist, "sh", "-c",
		"cat > /etc/systemd/system/systemd-sysusers.service.d/override.conf"); err != nil {
		return fmt.Errorf("could not generate systemd-sysusers override for guest OS: %w", err)
	}

	lingerCmd := withUser("cat > /home/[USER]/.config/systemd/[USER]/linger-example.service", user)
	if err := wslPipe(lingerService, dist, "sh", "-c", lingerCmd); err != nil {
		return fmt.Errorf("could not generate linger service for guest OS: %w", err)
	}

	if err := enableUserLinger(v, dist); err != nil {
		return err
	}

	if err := wslPipe(withUser(lingerSetup, user), dist, "sh"); err != nil {
		return fmt.Errorf("could not configure systemd settings for guest OS: %w", err)
	}

	if err := wslPipe(containersConf, dist, "sh", "-c", "cat > /etc/containers/containers.conf"); err != nil {
		return fmt.Errorf("could not create containers.conf for guest OS: %w", err)
	}

	if err := configureRegistries(v, dist); err != nil {
		return err
	}

	if err := v.setupPodmanDockerSock(dist, v.Rootful); err != nil {
		return err
	}

	if err := wslInvoke(dist, "sh", "-c", "echo wsl > /etc/containers/podman-machine"); err != nil {
		return fmt.Errorf("could not create podman-machine file for guest OS: %w", err)
	}

	if err := configureBindMounts(dist, user); err != nil {
		return err
	}

	return changeDistUserModeNetworking(dist, user, v.ImagePath, v.UserModeNetworking)
}

func configureBindMounts(dist string, user string) error {
	if err := wslPipe(fmt.Sprintf(bindMountSystemService, dist), dist, "sh", "-c", "cat > /etc/systemd/system/podman-mnt-bindings.service"); err != nil {
		return fmt.Errorf("could not create podman binding service file for guest OS: %w", err)
	}

	catUserService := "cat > " + getUserUnitPath(user)
	if err := wslPipe(getBindMountUserService(dist), dist, "sh", "-c", catUserService); err != nil {
		return fmt.Errorf("could not create podman binding user service file for guest OS: %w", err)
	}

	if err := wslPipe(getBindMountFsTab(dist), dist, "sh", "-c", "cat >> /etc/fstab"); err != nil {
		return fmt.Errorf("could not create podman binding fstab entry for guest OS: %w", err)
	}

	if err := wslPipe(getConfigBindServicesScript(user), dist, "sh"); err != nil {
		return fmt.Errorf("could not configure podman binding services for guest OS: %w", err)
	}

	catGroupDropin := fmt.Sprintf("cat > %s/%s", podmanSocketDropinPath, "10-group.conf")
	if err := wslPipe(overrideSocketGroup, dist, "sh", "-c", catGroupDropin); err != nil {
		return fmt.Errorf("could not configure podman socket group override: %w", err)
	}

	return nil
}

func getConfigBindServicesScript(user string) string {
	return fmt.Sprintf(configBindServices, user)
}

func getBindMountUserService(dist string) string {
	return fmt.Sprintf(bindMountUserService, dist)
}

func getUserUnitPath(user string) string {
	return fmt.Sprintf(bindUserUnitPath, user)
}

func getBindMountFsTab(dist string) string {
	return fmt.Sprintf(bindMountFsTab, dist)
}

func (v *MachineVM) setupPodmanDockerSock(dist string, rootful bool) error {
	content := machine.GetPodmanDockerTmpConfig(1000, rootful, true)

	if err := wslPipe(content, dist, "sh", "-c", "cat > "+machine.PodmanDockerTmpConfPath); err != nil {
		return fmt.Errorf("could not create internal docker sock conf: %w", err)
	}

	return nil
}

func configureProxy(dist string, useProxy bool, quiet bool) error {
	if !useProxy {
		_ = wslInvoke(dist, "sh", "-c", clearProxySettings)
		return nil
	}
	var content string
	for i, key := range config.ProxyEnv {
		if value, _ := os.LookupEnv(key); len(value) > 0 {
			var suffix string
			if i < (len(config.ProxyEnv) - 1) {
				suffix = "|"
			}
			content = fmt.Sprintf("%s%s=\"%s\"%s", content, key, value, suffix)
		}
	}

	if err := wslPipe(content, dist, "sh", "-c", proxyConfigAttempt); err != nil {
		const failMessage = "Failure creating proxy configuration"
		if exitErr, isExit := err.(*exec.ExitError); isExit && exitErr.ExitCode() != 42 {
			return fmt.Errorf("%v: %w", failMessage, err)
		}
		if !quiet {
			fmt.Println("Installing proxy support")
		}
		_ = wslPipe(proxyConfigSetup, dist, "sh", "-c",
			"cat > /usr/local/bin/proxyinit; chmod 755 /usr/local/bin/proxyinit")

		if err = wslPipe(content, dist, "/usr/local/bin/proxyinit"); err != nil {
			return fmt.Errorf("%v: %w", failMessage, err)
		}
	}

	return nil
}

func enableUserLinger(v *MachineVM, dist string) error {
	lingerCmd := "mkdir -p /var/lib/systemd/linger; touch /var/lib/systemd/linger/" + v.RemoteUsername
	if err := wslInvoke(dist, "sh", "-c", lingerCmd); err != nil {
		return fmt.Errorf("could not enable linger for remote user on guest OS: %w", err)
	}

	return nil
}

func configureRegistries(v *MachineVM, dist string) error {
	cmd := "cat > /etc/containers/registries.conf.d/999-podman-machine.conf"
	if err := wslPipe(registriesConf, dist, "sh", "-c", cmd); err != nil {
		return fmt.Errorf("could not configure registries on guest OS: %w", err)
	}

	return nil
}

func installScripts(dist string) error {
	if err := wslPipe(enterns, dist, "sh", "-c",
		"cat > /usr/local/bin/enterns; chmod 755 /usr/local/bin/enterns"); err != nil {
		return fmt.Errorf("could not create enterns script for guest OS: %w", err)
	}

	if err := wslPipe(profile, dist, "sh", "-c",
		"cat > /etc/profile.d/enterns.sh"); err != nil {
		return fmt.Errorf("could not create motd profile script for guest OS: %w", err)
	}

	if err := wslPipe(wslmotd, dist, "sh", "-c", "cat > /etc/wslmotd"); err != nil {
		return fmt.Errorf("could not create a WSL MOTD for guest OS: %w", err)
	}

	if err := wslPipe(bootstrap, dist, "sh", "-c",
		"cat > /root/bootstrap; chmod 755 /root/bootstrap"); err != nil {
		return fmt.Errorf("could not create bootstrap script for guest OS: %w", err)
	}

	if err := wslPipe(proxyConfigSetup, dist, "sh", "-c",
		"cat > /usr/local/bin/proxyinit; chmod 755 /usr/local/bin/proxyinit"); err != nil {
		return fmt.Errorf("could not create proxyinit script for guest OS: %w", err)
	}

	return nil
}

func writeWslConf(dist string, user string) error {
	if err := wslPipe(withUser(wslConf, user), dist, "sh", "-c", "cat > /etc/wsl.conf"); err != nil {
		return fmt.Errorf("could not configure wsl config for guest OS: %w", err)
	}

	return nil
}

func checkAndInstallWSL(opts machine.InitOptions) (bool, error) {
	if wutil.IsWSLInstalled() {
		return true, nil
	}

	admin := hasAdminRights()

	if !IsWSLFeatureEnabled() {
		return false, attemptFeatureInstall(opts, admin)
	}

	skip := false
	if !opts.ReExec && !admin {
		fmt.Println("Launching WSL Kernel Install...")
		if err := launchElevate(wslInstallKernel); err != nil {
			return false, err
		}

		skip = true
	}

	if !skip {
		if err := installWslKernel(); err != nil {
			fmt.Fprintf(os.Stderr, wslKernelError, wslInstallKernel)
			return false, err
		}

		if opts.ReExec {
			return false, nil
		}
	}

	return true, nil
}

func attemptFeatureInstall(opts machine.InitOptions, admin bool) error {
	if !winVersionAtLeast(10, 0, 18362) {
		return errors.New("your version of Windows does not support WSL. Update to Windows 10 Build 19041 or later")
	} else if !winVersionAtLeast(10, 0, 19041) {
		fmt.Fprint(os.Stderr, wslOldVersion)
		return errors.New("the WSL can not be automatically installed")
	}

	message := "WSL is not installed on this system, installing it.\n\n"

	if !admin {
		message += "Since you are not running as admin, a new window will open and " +
			"require you to approve administrator privileges.\n\n"
	}

	message += "NOTE: A system reboot will be required as part of this process. " +
		"If you prefer, you may abort now, and perform a manual installation using the \"wsl --install\" command."

	if !opts.ReExec && MessageBox(message, "Podman Machine", false) != 1 {
		return errors.New("the WSL installation aborted")
	}

	if !opts.ReExec && !admin {
		return launchElevate("install the Windows WSL Features")
	}

	return installWsl()
}

func launchElevate(operation string) error {
	truncateElevatedOutputFile()
	err := relaunchElevatedWait()
	if err != nil {
		if eerr, ok := err.(*ExitCodeError); ok {
			if eerr.code == ErrorSuccessRebootRequired {
				fmt.Println("Reboot is required to continue installation, please reboot at your convenience")
				return nil
			}
		}

		fmt.Fprintf(os.Stderr, "Elevated process failed with error: %v\n\n", err)
		dumpOutputFile()
		fmt.Fprintf(os.Stderr, wslInstallError, operation)
	}
	return err
}

func installWsl() error {
	log, err := getElevatedOutputFileWrite()
	if err != nil {
		return err
	}
	defer log.Close()
	if err := runCmdPassThroughTee(log, "dism", "/online", "/enable-feature",
		"/featurename:Microsoft-Windows-Subsystem-Linux", "/all", "/norestart"); isMsiError(err) {
		return fmt.Errorf("could not enable WSL Feature: %w", err)
	}

	if err = runCmdPassThroughTee(log, "dism", "/online", "/enable-feature",
		"/featurename:VirtualMachinePlatform", "/all", "/norestart"); isMsiError(err) {
		return fmt.Errorf("could not enable Virtual Machine Feature: %w", err)
	}
	log.Close()

	return reboot()
}

func installWslKernel() error {
	log, err := getElevatedOutputFileWrite()
	if err != nil {
		return err
	}
	defer log.Close()

	message := "Installing WSL Kernel Update"
	fmt.Println(message)
	fmt.Fprintln(log, message)

	backoff := 500 * time.Millisecond
	for i := 0; i < 5; i++ {
		err = runCmdPassThroughTee(log, "wsl", "--update")
		if err == nil {
			break
		}
		// In case of unusual circumstances (e.g. race with installer actions)
		// retry a few times
		message = "An error occurred attempting the WSL Kernel update, retrying..."
		fmt.Println(message)
		fmt.Fprintln(log, message)
		time.Sleep(backoff)
		backoff *= 2
	}

	if err != nil {
		return fmt.Errorf("could not install WSL Kernel: %w", err)
	}

	return nil
}

func getElevatedOutputFileName() (string, error) {
	dir, err := homedir.GetDataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "podman-elevated-output.log"), nil
}

func dumpOutputFile() {
	file, err := getElevatedOutputFileRead()
	if err != nil {
		logrus.Debug("could not find elevated child output file")
		return
	}
	defer file.Close()
	_, _ = io.Copy(os.Stdout, file)
}

func getElevatedOutputFileRead() (*os.File, error) {
	return getElevatedOutputFile(os.O_RDONLY)
}

func getElevatedOutputFileWrite() (*os.File, error) {
	return getElevatedOutputFile(os.O_WRONLY | os.O_CREATE | os.O_APPEND)
}

func appendOutputIfError(write bool, err error) {
	if write && err == nil {
		return
	}

	if file, check := getElevatedOutputFileWrite(); check == nil {
		defer file.Close()
		fmt.Fprintf(file, "Error: %v\n", err)
	}
}

func truncateElevatedOutputFile() error {
	name, err := getElevatedOutputFileName()
	if err != nil {
		return err
	}

	return os.Truncate(name, 0)
}

func getElevatedOutputFile(mode int) (*os.File, error) {
	name, err := getElevatedOutputFileName()
	if err != nil {
		return nil, err
	}

	dir, err := homedir.GetDataHome()
	if err != nil {
		return nil, err
	}

	if err = os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return os.OpenFile(name, mode, 0644)
}

func isMsiError(err error) bool {
	if err == nil {
		return false
	}

	if eerr, ok := err.(*exec.ExitError); ok {
		switch eerr.ExitCode() {
		case 0:
			fallthrough
		case ErrorSuccessRebootInitiated:
			fallthrough
		case ErrorSuccessRebootRequired:
			return false
		}
	}

	return true
}
func toDist(name string) string {
	if !strings.HasPrefix(name, "podman") {
		name = "podman-" + name
	}
	return name
}

func withUser(s string, user string) string {
	return strings.ReplaceAll(s, "[USER]", user)
}

func wslInvoke(dist string, arg ...string) error {
	newArgs := []string{"-u", "root", "-d", dist}
	newArgs = append(newArgs, arg...)
	return runCmdPassThrough("wsl", newArgs...)
}

func wslPipe(input string, dist string, arg ...string) error {
	newArgs := []string{"-u", "root", "-d", dist}
	newArgs = append(newArgs, arg...)
	return pipeCmdPassThrough("wsl", input, newArgs...)
}

func wslCreateKeys(identityPath string, dist string) (string, error) {
	return machine.CreateSSHKeysPrefix(identityPath, true, false, "wsl", "-u", "root", "-d", dist)
}

func runCmdPassThrough(name string, arg ...string) error {
	logrus.Debugf("Running command: %s %v", name, arg)
	cmd := exec.Command(name, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmdPassThroughTee(out io.Writer, name string, arg ...string) error {
	logrus.Debugf("Running command: %s %v", name, arg)

	// TODO - Perhaps improve this with a conpty pseudo console so that
	//        dism installer text bars mirror console behavior (redraw)
	cmd := exec.Command(name, arg...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, out)
	cmd.Stderr = io.MultiWriter(os.Stderr, out)
	return cmd.Run()
}

func pipeCmdPassThrough(name string, input string, arg ...string) error {
	logrus.Debugf("Running command: %s %v", name, arg)
	cmd := exec.Command(name, arg...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupWslProxyEnv() (hasProxy bool) {
	current, _ := os.LookupEnv("WSLENV")
	for _, key := range config.ProxyEnv {
		if value, _ := os.LookupEnv(key); len(value) < 1 {
			continue
		}

		hasProxy = true
		delim := ""
		if len(current) > 0 {
			delim = ":"
		}
		current = fmt.Sprintf("%s%s%s/u", current, delim, key)
	}
	if hasProxy {
		os.Setenv("WSLENV", current)
	}
	return
}

func (v *MachineVM) Set(_ string, opts machine.SetOptions) ([]error, error) {
	// If one setting fails to be applied, the others settings will not fail and still be applied.
	// The setting(s) that failed to be applied will have its errors returned in setErrors
	var setErrors []error

	v.lock.Lock()
	defer v.lock.Unlock()

	if opts.Rootful != nil && v.Rootful != *opts.Rootful {
		err := v.setRootful(*opts.Rootful)
		if err != nil {
			setErrors = append(setErrors, fmt.Errorf("setting rootful option: %w", err))
		} else {
			if v.isRunning() {
				logrus.Warn("restart is necessary for rootful change to go into effect")
			}
			v.Rootful = *opts.Rootful
		}
	}

	if opts.CPUs != nil {
		setErrors = append(setErrors, errors.New("changing CPUs not supported for WSL machines"))
	}

	if opts.Memory != nil {
		setErrors = append(setErrors, errors.New("changing memory not supported for WSL machines"))
	}

	if opts.USBs != nil {
		setErrors = append(setErrors, errors.New("changing USBs not supported for WSL machines"))
	}

	if opts.DiskSize != nil {
		setErrors = append(setErrors, errors.New("changing disk size not supported for WSL machines"))
	}

	if opts.UserModeNetworking != nil && *opts.UserModeNetworking != v.UserModeNetworking {
		update := true
		if v.isRunning() {
			update = false
			setErrors = append(setErrors, fmt.Errorf("user-mode networking can only be changed when the machine is not running"))
		} else {
			dist := toDist(v.Name)
			if err := changeDistUserModeNetworking(dist, v.RemoteUsername, v.ImagePath, *opts.UserModeNetworking); err != nil {
				update = false
				setErrors = append(setErrors, err)
			}
		}

		if update {
			v.UserModeNetworking = *opts.UserModeNetworking
		}
	}
	err := v.writeConfig()
	if err != nil {
		setErrors = append(setErrors, err)
	}

	if len(setErrors) > 0 {
		return setErrors, setErrors[0]
	}
	return setErrors, nil
}

func (v *MachineVM) Start(name string, opts machine.StartOptions) error {
	v.lock.Lock()
	defer v.lock.Unlock()

	if v.isRunning() {
		return machine.ErrVMAlreadyRunning
	}

	dist := toDist(name)
	useProxy := setupWslProxyEnv()
	if err := configureProxy(dist, useProxy, opts.Quiet); err != nil {
		return err
	}

	if !machine.IsLocalPortAvailable(v.Port) {
		logrus.Warnf("SSH port conflict detected, reassigning a new port")
		if err := v.reassignSshPort(); err != nil {
			return err
		}
	}

	// Startup user-mode networking if enabled
	if err := v.startUserModeNetworking(); err != nil {
		return err
	}

	err := wslInvoke(dist, "/root/bootstrap")
	if err != nil {
		return fmt.Errorf("the WSL bootstrap script failed: %w", err)
	}

	if !v.Rootful && !opts.NoInfo {
		fmt.Printf("\nThis machine is currently configured in rootless mode. If your containers\n")
		fmt.Printf("require root permissions (e.g. ports < 1024), or if you run into compatibility\n")
		fmt.Printf("issues with non-podman clients, you can switch using the following command: \n")

		suffix := ""
		if name != machine.DefaultMachineName {
			suffix = " " + name
		}
		fmt.Printf("\n\tpodman machine set --rootful%s\n\n", suffix)
	}

	globalName, pipeName, err := launchWinProxy(v)
	if !opts.NoInfo {
		if err != nil {
			fmt.Fprintln(os.Stderr, "API forwarding for Docker API clients is not available due to the following startup failures.")
			fmt.Fprintf(os.Stderr, "\t%s\n", err.Error())
			fmt.Fprintln(os.Stderr, "\nPodman clients are still able to connect.")
		} else {
			fmt.Printf("API forwarding listening on: %s\n", pipeName)
			if globalName {
				fmt.Printf("\nDocker API clients default to this address. You do not need to set DOCKER_HOST.\n")
			} else {
				fmt.Printf("\nAnother process was listening on the default Docker API pipe address.\n")
				fmt.Printf("You can still connect Docker API clients by setting DOCKER HOST using the\n")
				fmt.Printf("following powershell command in your terminal session:\n")
				fmt.Printf("\n\t$Env:DOCKER_HOST = '%s'\n", pipeName)
				fmt.Printf("\nOr in a classic CMD prompt:\n")
				fmt.Printf("\n\tset DOCKER_HOST=%s\n", pipeName)
				fmt.Printf("\nAlternatively, terminate the other process and restart podman machine.\n")
			}
		}
	}

	_, _, err = v.updateTimeStamps(true)
	return err
}

func obtainGlobalConfigLock() (*fileLock, error) {
	lockDir, err := machine.GetGlobalDataDir()
	if err != nil {
		return nil, err
	}

	// Lock file needs to be above all backends
	// TODO: This should be changed to a common.Config lock mechanism when available
	return lockFile(filepath.Join(lockDir, "podman-config.lck"))
}

func (v *MachineVM) reassignSshPort() error {
	dist := toDist(v.Name)
	newPort, err := machine.AllocateMachinePort()
	if err != nil {
		return err
	}

	success := false
	defer func() {
		if !success {
			if err := machine.ReleaseMachinePort(newPort); err != nil {
				logrus.Warnf("could not release port allocation as part of failure rollback (%d): %w", newPort, err)
			}
		}
	}()

	// We need to prevent racing connection updates to containers.conf globally
	// across all backends to prevent connection overwrites
	flock, err := obtainGlobalConfigLock()
	if err != nil {
		return fmt.Errorf("could not obtain global lock: %w", err)
	}
	defer flock.unlock()

	// Write a transient invalid port, to force a retry on failure
	oldPort := v.Port
	v.Port = 0
	if err := v.writeConfig(); err != nil {
		return err
	}

	if err := machine.ReleaseMachinePort(oldPort); err != nil {
		logrus.Warnf("could not release current ssh port allocation (%d): %w", oldPort, err)
	}

	if err := wslInvoke(dist, "sh", "-c", fmt.Sprintf(changePort, newPort)); err != nil {
		return fmt.Errorf("could not change SSH port for guest OS: %w", err)
	}

	v.Port = newPort
	uris, names := constructSSHUris(v)
	for i := 0; i < 2; i++ {
		if err := machine.ChangeConnectionURI(names[i], &uris[i]); err != nil {
			return err
		}
	}

	// Write updated port back
	if err := v.writeConfig(); err != nil {
		return err
	}

	success = true

	return nil
}

func findExecutablePeer(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(exe), name), nil
}

func launchWinProxy(v *MachineVM) (bool, string, error) {
	machinePipe := toDist(v.Name)
	if !machine.PipeNameAvailable(machinePipe) {
		return false, "", fmt.Errorf("could not start api proxy since expected pipe is not available: %s", machinePipe)
	}

	globalName := false
	if machine.PipeNameAvailable(globalPipe) {
		globalName = true
	}

	command, err := findExecutablePeer(winSShProxy)
	if err != nil {
		return globalName, "", err
	}

	stateDir, err := getWinProxyStateDir(v)
	if err != nil {
		return globalName, "", err
	}

	destSock := rootlessSock
	forwardUser := v.RemoteUsername

	if v.Rootful {
		destSock = rootfulSock
		forwardUser = "root"
	}

	dest := fmt.Sprintf("ssh://%s@localhost:%d%s", forwardUser, v.Port, destSock)
	args := []string{v.Name, stateDir, pipePrefix + machinePipe, dest, v.IdentityPath}
	waitPipe := machinePipe
	if globalName {
		args = append(args, pipePrefix+globalPipe, dest, v.IdentityPath)
		waitPipe = globalPipe
	}

	cmd := exec.Command(command, args...)
	if err := cmd.Start(); err != nil {
		return globalName, "", err
	}

	return globalName, pipePrefix + waitPipe, machine.WaitPipeExists(waitPipe, 80, func() error {
		active, exitCode := machine.GetProcessState(cmd.Process.Pid)
		if !active {
			return fmt.Errorf("win-sshproxy.exe failed to start, exit code: %d (see windows event logs)", exitCode)
		}

		return nil
	})
}

func getWinProxyStateDir(v *MachineVM) (string, error) {
	dir, err := machine.GetDataDir(vmtype)
	if err != nil {
		return "", err
	}
	stateDir := filepath.Join(dir, v.Name)
	if err = os.MkdirAll(stateDir, 0755); err != nil {
		return "", err
	}

	return stateDir, nil
}

func IsWSLFeatureEnabled() bool {
	return wutil.SilentExec("wsl", "--set-default-version", "2") == nil
}

func isWSLRunning(dist string) (bool, error) {
	return wslCheckExists(dist, true)
}

func isWSLExist(dist string) (bool, error) {
	return wslCheckExists(dist, false)
}

func wslCheckExists(dist string, running bool) (bool, error) {
	all, err := getAllWSLDistros(running)
	if err != nil {
		return false, err
	}

	_, exists := all[dist]
	return exists, nil
}

func getAllWSLDistros(running bool) (map[string]struct{}, error) {
	args := []string{"-l", "--quiet"}
	if running {
		args = append(args, "--running")
	}
	cmd := exec.Command("wsl", args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}

	all := make(map[string]struct{})
	scanner := bufio.NewScanner(transform.NewReader(out, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 {
			all[fields[0]] = struct{}{}
		}
	}

	_ = cmd.Wait()

	return all, nil
}

func isSystemdRunning(dist string) (bool, error) {
	cmd := exec.Command("wsl", "-u", "root", "-d", dist, "sh")
	cmd.Stdin = strings.NewReader(sysdpid + "\necho $SYSDPID\n")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	if err = cmd.Start(); err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(out)
	result := false
	if scanner.Scan() {
		text := scanner.Text()
		i, err := strconv.Atoi(text)
		if err == nil && i > 0 {
			result = true
		}
	}

	_ = cmd.Wait()

	return result, nil
}

func (v *MachineVM) Stop(name string, _ machine.StopOptions) error {
	v.lock.Lock()
	defer v.lock.Unlock()

	dist := toDist(v.Name)

	wsl, err := isWSLRunning(dist)
	if err != nil {
		return err
	}

	sysd := false
	if wsl {
		sysd, err = isSystemdRunning(dist)
		if err != nil {
			return err
		}
	}

	if !wsl || !sysd {
		return nil
	}

	// Stop user-mode networking if enabled
	if err := v.stopUserModeNetworking(dist); err != nil {
		fmt.Fprintf(os.Stderr, "Could not cleanly stop user-mode networking: %s\n", err.Error())
	}

	_, _, _ = v.updateTimeStamps(true)

	if err := stopWinProxy(v); err != nil {
		fmt.Fprintf(os.Stderr, "Could not stop API forwarding service (win-sshproxy.exe): %s\n", err.Error())
	}

	cmd := exec.Command("wsl", "-u", "root", "-d", dist, "sh")
	cmd.Stdin = strings.NewReader(waitTerm)
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("executing wait command: %w", err)
	}

	exitCmd := exec.Command("wsl", "-u", "root", "-d", dist, "/usr/local/bin/enterns", "systemctl", "exit", "0")
	if err = exitCmd.Run(); err != nil {
		return fmt.Errorf("stopping sysd: %w", err)
	}

	if err = cmd.Wait(); err != nil {
		return err
	}

	return terminateDist(dist)
}

func terminateDist(dist string) error {
	cmd := exec.Command("wsl", "--terminate", dist)
	return cmd.Run()
}

func unregisterDist(dist string) error {
	cmd := exec.Command("wsl", "--unregister", dist)
	return cmd.Run()
}

func (v *MachineVM) State(bypass bool) (define.Status, error) {
	if v.isRunning() {
		return define.Running, nil
	}

	return define.Stopped, nil
}

func stopWinProxy(v *MachineVM) error {
	pid, tid, tidFile, err := readWinProxyTid(v)
	if err != nil {
		return err
	}

	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return nil
	}
	sendQuit(tid)
	_ = waitTimeout(proc, 20*time.Second)
	_ = os.Remove(tidFile)

	return nil
}

func waitTimeout(proc *os.Process, timeout time.Duration) bool {
	done := make(chan bool)
	go func() {
		proc.Wait()
		done <- true
	}()
	ret := false
	select {
	case <-time.After(timeout):
		proc.Kill()
		<-done
	case <-done:
		ret = true
		break
	}

	return ret
}

func readWinProxyTid(v *MachineVM) (uint32, uint32, string, error) {
	stateDir, err := getWinProxyStateDir(v)
	if err != nil {
		return 0, 0, "", err
	}

	tidFile := filepath.Join(stateDir, winSshProxyTid)
	contents, err := os.ReadFile(tidFile)
	if err != nil {
		return 0, 0, "", err
	}

	var pid, tid uint32
	fmt.Sscanf(string(contents), "%d:%d", &pid, &tid)
	return pid, tid, tidFile, nil
}

//nolint:cyclop
func (v *MachineVM) Remove(name string, opts machine.RemoveOptions) (string, func() error, error) {
	var files []string

	if v.isRunning() {
		if !opts.Force {
			return "", nil, &machine.ErrVMRunningCannotDestroyed{Name: v.Name}
		}
		if err := v.Stop(v.Name, machine.StopOptions{}); err != nil {
			return "", nil, err
		}
	}

	v.lock.Lock()
	defer v.lock.Unlock()

	// Collect all the files that need to be destroyed
	if !opts.SaveKeys {
		files = append(files, v.IdentityPath, v.IdentityPath+".pub")
	}
	if !opts.SaveImage {
		files = append(files, v.ImagePath)
	}

	vmConfigDir, err := machine.GetConfDir(vmtype)
	if err != nil {
		return "", nil, err
	}
	files = append(files, filepath.Join(vmConfigDir, v.Name+".json"))

	vmDataDir, err := machine.GetDataDir(vmtype)
	if err != nil {
		return "", nil, err
	}
	files = append(files, filepath.Join(vmDataDir, "wsldist", v.Name))

	confirmationMessage := "\nThe following files will be deleted:\n\n"
	for _, msg := range files {
		confirmationMessage += msg + "\n"
	}

	confirmationMessage += "\n"
	return confirmationMessage, func() error {
		if err := machine.RemoveConnections(v.Name, v.Name+"-root"); err != nil {
			logrus.Error(err)
		}
		if err := runCmdPassThrough("wsl", "--unregister", toDist(v.Name)); err != nil {
			logrus.Error(err)
		}
		for _, f := range files {
			if err := utils.GuardedRemoveAll(f); err != nil {
				logrus.Error(err)
			}
		}
		if err := machine.ReleaseMachinePort(v.Port); err != nil {
			logrus.Warnf("could not release port allocation as part of removal (%d): %w", v.Port, err)
		}

		return nil
	}, nil
}

func (v *MachineVM) isRunning() bool {
	dist := toDist(v.Name)

	wsl, err := isWSLRunning(dist)
	if err != nil {
		return false
	}

	sysd := false
	if wsl {
		sysd, err = isSystemdRunning(dist)

		if err != nil {
			return false
		}
	}

	return sysd
}

// SSH opens an interactive SSH session to the vm specified.
// Added ssh function to VM interface: pkg/machine/config/go : line 58
func (v *MachineVM) SSH(name string, opts machine.SSHOptions) error {
	if !v.isRunning() {
		return fmt.Errorf("vm %q is not running.", v.Name)
	}

	username := opts.Username
	if username == "" {
		username = v.RemoteUsername
	}

	sshDestination := username + "@localhost"
	port := strconv.Itoa(v.Port)

	args := []string{"-i", v.IdentityPath, "-p", port, sshDestination,
		"-o", "IdentitiesOnly yes",
		"-o", "UserKnownHostsFile /dev/null",
		"-o", "StrictHostKeyChecking no"}
	if len(opts.Args) > 0 {
		args = append(args, opts.Args...)
	} else {
		fmt.Printf("Connecting to vm %s. To close connection, use `~.` or `exit`\n", v.Name)
	}

	cmd := exec.Command("ssh", args...)
	logrus.Debugf("Executing: ssh %v\n", args)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func (v *MachineVM) updateTimeStamps(updateLast bool) (time.Time, time.Time, error) {
	var err error
	if updateLast {
		v.LastUp = time.Now()
		err = v.writeConfig()
	}

	return v.Created, v.LastUp, err
}

func getDiskSize(vm *MachineVM) uint64 {
	vmDataDir, err := machine.GetDataDir(vmtype)
	if err != nil {
		return 0
	}
	distDir := filepath.Join(vmDataDir, "wsldist")
	disk := filepath.Join(distDir, vm.Name, "ext4.vhdx")
	info, err := os.Stat(disk)
	if err != nil {
		return 0
	}
	return uint64(info.Size())
}

func getCPUs(vm *MachineVM) (uint64, error) {
	dist := toDist(vm.Name)
	if run, _ := isWSLRunning(dist); !run {
		return 0, nil
	}
	cmd := exec.Command("wsl", "-u", "root", "-d", dist, "nproc")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	if err = cmd.Start(); err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(out)
	var result string
	for scanner.Scan() {
		result = scanner.Text()
	}
	_ = cmd.Wait()

	ret, err := strconv.Atoi(result)
	return uint64(ret), err
}

func getMem(vm *MachineVM) (uint64, error) {
	dist := toDist(vm.Name)
	if run, _ := isWSLRunning(dist); !run {
		return 0, nil
	}
	cmd := exec.Command("wsl", "-u", "root", "-d", dist, "cat", "/proc/meminfo")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, err
	}
	if err = cmd.Start(); err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(out)
	var (
		total, available uint64
		t, a             int
	)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if strings.HasPrefix(fields[0], "MemTotal") && len(fields) >= 2 {
			t, err = strconv.Atoi(fields[1])
			total = uint64(t) * 1024
		} else if strings.HasPrefix(fields[0], "MemAvailable") && len(fields) >= 2 {
			a, err = strconv.Atoi(fields[1])
			available = uint64(a) * 1024
		}
		if err != nil {
			break
		}
	}
	_ = cmd.Wait()

	return total - available, err
}

func (v *MachineVM) setRootful(rootful bool) error {
	if err := machine.SetRootful(rootful, v.Name, v.Name+"-root"); err != nil {
		return err
	}

	dist := toDist(v.Name)
	return v.setupPodmanDockerSock(dist, rootful)
}

// Inspect returns verbose detail about the machine
func (v *MachineVM) Inspect() (*machine.InspectInfo, error) {
	state, err := v.State(false)
	if err != nil {
		return nil, err
	}

	connInfo := new(machine.ConnectionConfig)
	machinePipe := toDist(v.Name)
	connInfo.PodmanPipe = &define.VMFile{Path: `\\.\pipe\` + machinePipe}

	created, lastUp, _ := v.updateTimeStamps(state == define.Running)
	return &machine.InspectInfo{
		ConfigPath:     define.VMFile{Path: v.ConfigPath},
		ConnectionInfo: *connInfo,
		Created:        created,
		Image: machine.ImageConfig{
			ImagePath:   define.VMFile{Path: v.ImagePath},
			ImageStream: v.ImageStream,
		},
		LastUp:             lastUp,
		Name:               v.Name,
		Resources:          v.getResources(),
		SSHConfig:          v.SSHConfig,
		State:              state,
		UserModeNetworking: v.UserModeNetworking,
		Rootful:            v.Rootful,
	}, nil
}

func (v *MachineVM) getResources() (resources vmconfigs.ResourceConfig) {
	resources.CPUs, _ = getCPUs(v)
	resources.Memory, _ = getMem(v)
	resources.DiskSize = getDiskSize(v)
	return
}
