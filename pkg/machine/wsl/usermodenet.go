//go:build windows
// +build windows

package wsl

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containers/podman/v4/pkg/machine"
	"github.com/containers/podman/v4/pkg/machine/wsl/wutil"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/sirupsen/logrus"
)

const startUserModeNet = `
set -e
STATE=/mnt/wsl/podman-usermodenet
mkdir -p $STATE
cp -f /mnt/wsl/resolv.conf $STATE/resolv.orig
ip route show default > $STATE/route.dat
ROUTE=$(<$STATE/route.dat)
if [[ $ROUTE =~ .*192\.168\.127\.1.* ]]; then
	exit 2
fi
if [[ ! $ROUTE =~ default\ via ]]; then
	exit 3
fi
nohup /usr/local/bin/vm -iface podman-usermode -stop-if-exist ignore -url "stdio:$GVPROXY?listen-stdio=accept" > /var/log/vm.log 2> /var/log/vm.err  < /dev/null &
echo $! > $STATE/vm.pid
sleep 1
ps -eo args | grep -q -m1 ^/usr/local/bin/vm || exit 42
`

const stopUserModeNet = `
STATE=/mnt/wsl/podman-usermodenet
if [[ ! -f "$STATE/vm.pid" || ! -f "$STATE/route.dat" ]]; then
	exit 2
fi
cp -f $STATE/resolv.orig /mnt/wsl/resolv.conf
GPID=$(<$STATE/vm.pid)
kill $GPID > /dev/null
while kill -0 $GPID > /dev/null 2>&1; do
	sleep 1
done
ip route del default > /dev/null 2>&1
ROUTE=$(<$STATE/route.dat)
if [[ ! $ROUTE =~ default\ via ]]; then
	exit 3
fi
ip route add $ROUTE
rm -rf /mnt/wsl/podman-usermodenet
`

func verifyWSLUserModeCompat() error {
	if wutil.IsWSLStoreVersionInstalled() {
		return nil
	}

	prefix := ""
	if !winVersionAtLeast(10, 0, 19043) {
		prefix = "upgrade to 22H2, "
	}

	return fmt.Errorf("user-mode networking requires a newer version of WSL: "+
		"%sapply all outstanding windows updates, and then run `wsl --update`",
		prefix)
}

func (v *MachineVM) startUserModeNetworking() error {
	if !v.UserModeNetworking {
		return nil
	}

	exe, err := findExecutablePeer(gvProxy)
	if err != nil {
		return fmt.Errorf("could not locate %s, which is necessary for user-mode networking, please reinstall", gvProxy)
	}

	flock, err := v.obtainUserModeNetLock()
	if err != nil {
		return err
	}
	defer flock.unlock()

	running, err := isWSLRunning(userModeDist)
	if err != nil {
		return err
	}
	running = running && isGvProxyVMRunning()

	// Start or reuse
	if !running {
		if err := v.launchUserModeNetDist(exe); err != nil {
			return err
		}
	}

	if err := createUserModeResolvConf(toDist(v.Name)); err != nil {
		return err
	}

	// Register in-use
	err = v.addUserModeNetEntry()
	if err != nil {
		return err
	}

	return nil
}

func (v *MachineVM) stopUserModeNetworking(dist string) error {
	if !v.UserModeNetworking {
		return nil
	}

	flock, err := v.obtainUserModeNetLock()
	if err != nil {
		return err
	}
	defer flock.unlock()

	err = v.removeUserModeNetEntry()
	if err != nil {
		return err
	}

	count, err := v.cleanupAndCountNetEntries()
	if err != nil {
		return err
	}

	// Leave running if still in-use
	if count > 0 {
		return nil
	}

	fmt.Println("Stopping user-mode networking...")

	err = wslPipe(stopUserModeNet, userModeDist, "bash")
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			switch exitErr.ExitCode() {
			case 2:
				err = fmt.Errorf("startup state was missing")
			case 3:
				err = fmt.Errorf("route state is missing a default route")
			}
		}
		logrus.Warnf("problem tearing down user-mode networking cleanly, forcing: %s", err.Error())
	}

	return terminateDist(userModeDist)
}

func isGvProxyVMRunning() bool {
	return wslInvoke(userModeDist, "bash", "-c", "ps -eo args | grep -q -m1 ^/usr/local/bin/vm || exit 42") == nil
}

func (v *MachineVM) launchUserModeNetDist(exeFile string) error {
	fmt.Println("Starting user-mode networking...")

	exe, err := specgen.ConvertWinMountPath(exeFile)
	if err != nil {
		return err
	}

	cmdStr := fmt.Sprintf("GVPROXY=%q\n%s", exe, startUserModeNet)
	if err := wslPipe(cmdStr, userModeDist, "bash"); err != nil {
		_ = terminateDist(userModeDist)

		if exitErr, ok := err.(*exec.ExitError); ok {
			switch exitErr.ExitCode() {
			case 2:
				return fmt.Errorf("another user-mode network is running, only one can be used at a time: shut down all machines and run wsl --shutdown if this is unexpected")
			case 3:
				err = fmt.Errorf("route state is missing a default route: shutdown all machines and run wsl --shutdown to recover")
			}
		}

		return fmt.Errorf("error setting up user-mode networking: %w", err)
	}

	return nil
}

func installUserModeDist(dist string, imagePath string) error {
	if err := verifyWSLUserModeCompat(); err != nil {
		return err
	}

	exists, err := isWSLExist(userModeDist)
	if err != nil {
		return err
	}

	if !exists {
		if err := wslInvoke(dist, "test", "-f", "/usr/local/bin/vm"); err != nil {
			return fmt.Errorf("existing machine is too old, can't install user-mode networking dist until machine is reinstalled (using podman machine rm, then podman machine init)")
		}

		const prompt = "Installing user-mode networking distribution..."
		if _, err := provisionWSLDist(userModeDist, imagePath, prompt); err != nil {
			return err
		}

		_ = terminateDist(userModeDist)
	}

	return nil
}

func createUserModeResolvConf(dist string) error {
	err := wslPipe(resolvConfUserNet, dist, "bash", "-c", "(rm -f /etc/resolv.conf; cat > /etc/resolv.conf)")
	if err != nil {
		return fmt.Errorf("could not create resolv.conf: %w", err)
	}
	return err
}

func (v *MachineVM) getUserModeNetDir() (string, error) {
	vmDataDir, err := machine.GetDataDir(vmtype)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(vmDataDir, userModeDist)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("could not create %s directory: %w", userModeDist, err)
	}

	return dir, nil
}

func (v *MachineVM) getUserModeNetEntriesDir() (string, error) {
	netDir, err := v.getUserModeNetDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(netDir, "entries")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("could not create %s/entries directory: %w", userModeDist, err)
	}

	return dir, nil
}

func (v *MachineVM) addUserModeNetEntry() error {
	entriesDir, err := v.getUserModeNetEntriesDir()
	if err != nil {
		return err
	}

	path := filepath.Join(entriesDir, toDist(v.Name))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not add user-mode networking registration: %w", err)
	}
	file.Close()
	return nil
}

func (v *MachineVM) removeUserModeNetEntry() error {
	entriesDir, err := v.getUserModeNetEntriesDir()
	if err != nil {
		return err
	}

	path := filepath.Join(entriesDir, toDist(v.Name))
	return os.Remove(path)
}

func (v *MachineVM) cleanupAndCountNetEntries() (uint, error) {
	entriesDir, err := v.getUserModeNetEntriesDir()
	if err != nil {
		return 0, err
	}

	allDists, err := getAllWSLDistros(true)
	if err != nil {
		return 0, err
	}

	var count uint = 0
	files, err := os.ReadDir(entriesDir)
	if err != nil {
		return 0, err
	}

	for _, file := range files {
		_, running := allDists[file.Name()]
		if !running {
			_ = os.Remove(filepath.Join(entriesDir, file.Name()))
			continue
		}
		count++
	}

	return count, nil
}

func (v *MachineVM) obtainUserModeNetLock() (*fileLock, error) {
	dir, err := v.getUserModeNetDir()

	if err != nil {
		return nil, err
	}

	var flock *fileLock
	lockPath := filepath.Join(dir, "podman-usermodenet.lck")
	if flock, err = lockFile(lockPath); err != nil {
		return nil, fmt.Errorf("could not lock user-mode networking lock file: %w", err)
	}

	return flock, nil
}

func changeDistUserModeNetworking(dist string, user string, image string, enable bool) error {
	// Only install if user-mode is being enabled and there was an image path passed
	if enable {
		if len(image) <= 0 {
			return errors.New("existing machine configuration is corrupt, no image is defined")
		}
		if err := installUserModeDist(dist, image); err != nil {
			return err
		}
	}

	if err := writeWslConf(dist, user); err != nil {
		return err
	}

	if enable {
		return appendDisableAutoResolve(dist)
	}

	return nil
}

func appendDisableAutoResolve(dist string) error {
	if err := wslPipe(wslConfUserNet, dist, "sh", "-c", "cat >> /etc/wsl.conf"); err != nil {
		return fmt.Errorf("could not append resolv config to wsl.conf: %w", err)
	}

	return nil
}
