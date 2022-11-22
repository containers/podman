//go:build linux
// +build linux

package chroot

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containers/buildah/copier"
	"github.com/containers/storage/pkg/mount"
	"github.com/containers/storage/pkg/unshare"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
	"golang.org/x/sys/unix"
)

var (
	rlimitsMap = map[string]int{
		"RLIMIT_AS":         unix.RLIMIT_AS,
		"RLIMIT_CORE":       unix.RLIMIT_CORE,
		"RLIMIT_CPU":        unix.RLIMIT_CPU,
		"RLIMIT_DATA":       unix.RLIMIT_DATA,
		"RLIMIT_FSIZE":      unix.RLIMIT_FSIZE,
		"RLIMIT_LOCKS":      unix.RLIMIT_LOCKS,
		"RLIMIT_MEMLOCK":    unix.RLIMIT_MEMLOCK,
		"RLIMIT_MSGQUEUE":   unix.RLIMIT_MSGQUEUE,
		"RLIMIT_NICE":       unix.RLIMIT_NICE,
		"RLIMIT_NOFILE":     unix.RLIMIT_NOFILE,
		"RLIMIT_NPROC":      unix.RLIMIT_NPROC,
		"RLIMIT_RSS":        unix.RLIMIT_RSS,
		"RLIMIT_RTPRIO":     unix.RLIMIT_RTPRIO,
		"RLIMIT_RTTIME":     unix.RLIMIT_RTTIME,
		"RLIMIT_SIGPENDING": unix.RLIMIT_SIGPENDING,
		"RLIMIT_STACK":      unix.RLIMIT_STACK,
	}
	rlimitsReverseMap = map[int]string{}
)

type runUsingChrootSubprocOptions struct {
	Spec        *specs.Spec
	BundlePath  string
	UIDMappings []syscall.SysProcIDMap
	GIDMappings []syscall.SysProcIDMap
}

func setPlatformUnshareOptions(spec *specs.Spec, cmd *unshare.Cmd) error {
	// If we have configured ID mappings, set them here so that they can apply to the child.
	hostUidmap, hostGidmap, err := unshare.GetHostIDMappings("")
	if err != nil {
		return err
	}
	uidmap, gidmap := spec.Linux.UIDMappings, spec.Linux.GIDMappings
	if len(uidmap) == 0 {
		// No UID mappings are configured for the container.  Borrow our parent's mappings.
		uidmap = append([]specs.LinuxIDMapping{}, hostUidmap...)
		for i := range uidmap {
			uidmap[i].HostID = uidmap[i].ContainerID
		}
	}
	if len(gidmap) == 0 {
		// No GID mappings are configured for the container.  Borrow our parent's mappings.
		gidmap = append([]specs.LinuxIDMapping{}, hostGidmap...)
		for i := range gidmap {
			gidmap[i].HostID = gidmap[i].ContainerID
		}
	}

	cmd.UnshareFlags = syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS
	requestedUserNS := false
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == specs.UserNamespace {
			requestedUserNS = true
		}
	}
	if len(spec.Linux.UIDMappings) > 0 || len(spec.Linux.GIDMappings) > 0 || requestedUserNS {
		cmd.UnshareFlags = cmd.UnshareFlags | syscall.CLONE_NEWUSER
		cmd.UidMappings = uidmap
		cmd.GidMappings = gidmap
		cmd.GidMappingsEnableSetgroups = true
	}
	cmd.OOMScoreAdj = spec.Process.OOMScoreAdj
	return nil
}

func setContainerHostname(name string) {
	if err := unix.Sethostname([]byte(name)); err != nil {
		logrus.Debugf("failed to set hostname %q for process: %v", name, err)
	}
}

// logNamespaceDiagnostics knows which namespaces we want to create.
// Output debug messages when that differs from what we're being asked to do.
func logNamespaceDiagnostics(spec *specs.Spec) {
	sawMountNS := false
	sawUTSNS := false
	for _, ns := range spec.Linux.Namespaces {
		switch ns.Type {
		case specs.CgroupNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join cgroup namespace, sorry about that")
			} else {
				logrus.Debugf("unable to create cgroup namespace, sorry about that")
			}
		case specs.IPCNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join IPC namespace, sorry about that")
			} else {
				logrus.Debugf("unable to create IPC namespace, sorry about that")
			}
		case specs.MountNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join mount namespace %q, creating a new one", ns.Path)
			}
			sawMountNS = true
		case specs.NetworkNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join network namespace, sorry about that")
			} else {
				logrus.Debugf("unable to create network namespace, sorry about that")
			}
		case specs.PIDNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join PID namespace, sorry about that")
			} else {
				logrus.Debugf("unable to create PID namespace, sorry about that")
			}
		case specs.UserNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join user namespace, sorry about that")
			}
		case specs.UTSNamespace:
			if ns.Path != "" {
				logrus.Debugf("unable to join UTS namespace %q, creating a new one", ns.Path)
			}
			sawUTSNS = true
		}
	}
	if !sawMountNS {
		logrus.Debugf("mount namespace not requested, but creating a new one anyway")
	}
	if !sawUTSNS {
		logrus.Debugf("UTS namespace not requested, but creating a new one anyway")
	}
}

// setApparmorProfile sets the apparmor profile for ourselves, and hopefully any child processes that we'll start.
func setApparmorProfile(spec *specs.Spec) error {
	if !apparmor.IsEnabled() || spec.Process.ApparmorProfile == "" {
		return nil
	}
	if err := apparmor.ApplyProfile(spec.Process.ApparmorProfile); err != nil {
		return fmt.Errorf("setting apparmor profile to %q: %w", spec.Process.ApparmorProfile, err)
	}
	return nil
}

// setCapabilities sets capabilities for ourselves, to be more or less inherited by any processes that we'll start.
func setCapabilities(spec *specs.Spec, keepCaps ...string) error {
	currentCaps, err := capability.NewPid2(0)
	if err != nil {
		return fmt.Errorf("reading capabilities of current process: %w", err)
	}
	if err := currentCaps.Load(); err != nil {
		return fmt.Errorf("loading capabilities: %w", err)
	}
	caps, err := capability.NewPid2(0)
	if err != nil {
		return fmt.Errorf("reading capabilities of current process: %w", err)
	}
	capMap := map[capability.CapType][]string{
		capability.BOUNDING:    spec.Process.Capabilities.Bounding,
		capability.EFFECTIVE:   spec.Process.Capabilities.Effective,
		capability.INHERITABLE: []string{},
		capability.PERMITTED:   spec.Process.Capabilities.Permitted,
		capability.AMBIENT:     spec.Process.Capabilities.Ambient,
	}
	knownCaps := capability.List()
	noCap := capability.Cap(-1)
	for capType, capList := range capMap {
		for _, capToSet := range capList {
			cap := noCap
			for _, c := range knownCaps {
				if strings.EqualFold("CAP_"+c.String(), capToSet) {
					cap = c
					break
				}
			}
			if cap == noCap {
				return fmt.Errorf("mapping capability %q to a number", capToSet)
			}
			caps.Set(capType, cap)
		}
		for _, capToSet := range keepCaps {
			cap := noCap
			for _, c := range knownCaps {
				if strings.EqualFold("CAP_"+c.String(), capToSet) {
					cap = c
					break
				}
			}
			if cap == noCap {
				return fmt.Errorf("mapping capability %q to a number", capToSet)
			}
			if currentCaps.Get(capType, cap) {
				caps.Set(capType, cap)
			}
		}
	}
	if err = caps.Apply(capability.CAPS | capability.BOUNDS | capability.AMBS); err != nil {
		return fmt.Errorf("setting capabilities: %w", err)
	}
	return nil
}

func makeRlimit(limit specs.POSIXRlimit) unix.Rlimit {
	return unix.Rlimit{Cur: limit.Soft, Max: limit.Hard}
}

func createPlatformContainer(options runUsingChrootExecSubprocOptions) error {
	return errors.New("unsupported createPlatformContainer")
}

func makeReadOnly(mntpoint string, flags uintptr) error {
	var fs unix.Statfs_t
	// Make sure it's read-only.
	if err := unix.Statfs(mntpoint, &fs); err != nil {
		return fmt.Errorf("checking if directory %q was bound read-only: %w", mntpoint, err)
	}
	if fs.Flags&unix.ST_RDONLY == 0 {
		if err := unix.Mount(mntpoint, mntpoint, "bind", flags|unix.MS_REMOUNT, ""); err != nil {
			return fmt.Errorf("remounting %s in mount namespace read-only: %w", mntpoint, err)
		}
	}
	return nil
}

// setupChrootBindMounts actually bind mounts things under the rootfs, and returns a
// callback that will clean up its work.
func setupChrootBindMounts(spec *specs.Spec, bundlePath string) (undoBinds func() error, err error) {
	var fs unix.Statfs_t
	undoBinds = func() error {
		if err2 := unix.Unmount(spec.Root.Path, unix.MNT_DETACH); err2 != nil {
			retries := 0
			for (err2 == unix.EBUSY || err2 == unix.EAGAIN) && retries < 50 {
				time.Sleep(50 * time.Millisecond)
				err2 = unix.Unmount(spec.Root.Path, unix.MNT_DETACH)
				retries++
			}
			if err2 != nil {
				logrus.Warnf("pkg/chroot: error unmounting %q (retried %d times): %v", spec.Root.Path, retries, err2)
				if err == nil {
					err = err2
				}
			}
		}
		return err
	}

	// Now bind mount all of those things to be under the rootfs's location in this
	// mount namespace.
	commonFlags := uintptr(unix.MS_BIND | unix.MS_REC | unix.MS_PRIVATE)
	bindFlags := commonFlags | unix.MS_NODEV
	devFlags := commonFlags | unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_RDONLY
	procFlags := devFlags | unix.MS_NODEV
	sysFlags := devFlags | unix.MS_NODEV

	// Bind /dev read-only.
	subDev := filepath.Join(spec.Root.Path, "/dev")
	if err := unix.Mount("/dev", subDev, "bind", devFlags, ""); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(subDev, 0755)
			if err == nil {
				err = unix.Mount("/dev", subDev, "bind", devFlags, "")
			}
		}
		if err != nil {
			return undoBinds, fmt.Errorf("bind mounting /dev from host into mount namespace: %w", err)
		}
	}
	// Make sure it's read-only.
	if err = unix.Statfs(subDev, &fs); err != nil {
		return undoBinds, fmt.Errorf("checking if directory %q was bound read-only: %w", subDev, err)
	}
	if fs.Flags&unix.ST_RDONLY == 0 {
		if err := unix.Mount(subDev, subDev, "bind", devFlags|unix.MS_REMOUNT, ""); err != nil {
			return undoBinds, fmt.Errorf("remounting /dev in mount namespace read-only: %w", err)
		}
	}
	logrus.Debugf("bind mounted %q to %q", "/dev", filepath.Join(spec.Root.Path, "/dev"))

	// Bind /proc read-only.
	subProc := filepath.Join(spec.Root.Path, "/proc")
	if err := unix.Mount("/proc", subProc, "bind", procFlags, ""); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(subProc, 0755)
			if err == nil {
				err = unix.Mount("/proc", subProc, "bind", procFlags, "")
			}
		}
		if err != nil {
			return undoBinds, fmt.Errorf("bind mounting /proc from host into mount namespace: %w", err)
		}
	}
	logrus.Debugf("bind mounted %q to %q", "/proc", filepath.Join(spec.Root.Path, "/proc"))

	// Bind /sys read-only.
	subSys := filepath.Join(spec.Root.Path, "/sys")
	if err := unix.Mount("/sys", subSys, "bind", sysFlags, ""); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = os.Mkdir(subSys, 0755)
			if err == nil {
				err = unix.Mount("/sys", subSys, "bind", sysFlags, "")
			}
		}
		if err != nil {
			return undoBinds, fmt.Errorf("bind mounting /sys from host into mount namespace: %w", err)
		}
	}
	if err := makeReadOnly(subSys, sysFlags); err != nil {
		return undoBinds, err
	}

	mnts, _ := mount.GetMounts()
	for _, m := range mnts {
		if !strings.HasPrefix(m.Mountpoint, "/sys/") &&
			m.Mountpoint != "/sys" {
			continue
		}
		subSys := filepath.Join(spec.Root.Path, m.Mountpoint)
		if err := unix.Mount(m.Mountpoint, subSys, "bind", sysFlags, ""); err != nil {
			msg := fmt.Sprintf("could not bind mount %q, skipping: %v", m.Mountpoint, err)
			if strings.HasPrefix(m.Mountpoint, "/sys") {
				logrus.Infof(msg)
			} else {
				logrus.Warningf(msg)
			}
			continue
		}
		if err := makeReadOnly(subSys, sysFlags); err != nil {
			return undoBinds, err
		}
	}
	logrus.Debugf("bind mounted %q to %q", "/sys", filepath.Join(spec.Root.Path, "/sys"))

	// Bind mount in everything we've been asked to mount.
	for _, m := range spec.Mounts {
		// Skip anything that we just mounted.
		switch m.Destination {
		case "/dev", "/proc", "/sys":
			logrus.Debugf("already bind mounted %q on %q", m.Destination, filepath.Join(spec.Root.Path, m.Destination))
			continue
		default:
			if strings.HasPrefix(m.Destination, "/dev/") {
				continue
			}
			if strings.HasPrefix(m.Destination, "/proc/") {
				continue
			}
			if strings.HasPrefix(m.Destination, "/sys/") {
				continue
			}
		}
		// Skip anything that isn't a bind or tmpfs mount.
		if m.Type != "bind" && m.Type != "tmpfs" && m.Type != "overlay" {
			logrus.Debugf("skipping mount of type %q on %q", m.Type, m.Destination)
			continue
		}
		// If the target is there, we can just mount it.
		var srcinfo os.FileInfo
		switch m.Type {
		case "bind":
			srcinfo, err = os.Stat(m.Source)
			if err != nil {
				return undoBinds, fmt.Errorf("examining %q for mounting in mount namespace: %w", m.Source, err)
			}
		case "overlay":
			fallthrough
		case "tmpfs":
			srcinfo, err = os.Stat("/")
			if err != nil {
				return undoBinds, fmt.Errorf("examining / to use as a template for a %s: %w", m.Type, err)
			}
		}
		target := filepath.Join(spec.Root.Path, m.Destination)
		// Check if target is a symlink
		stat, err := os.Lstat(target)
		// If target is a symlink, follow the link and ensure the destination exists
		if err == nil && stat != nil && (stat.Mode()&os.ModeSymlink != 0) {
			target, err = copier.Eval(spec.Root.Path, m.Destination, copier.EvalOptions{})
			if err != nil {
				return nil, fmt.Errorf("evaluating symlink %q: %w", target, err)
			}
			// Stat the destination of the evaluated symlink
			_, err = os.Stat(target)
		}
		if err != nil {
			// If the target can't be stat()ted, check the error.
			if !errors.Is(err, os.ErrNotExist) {
				return undoBinds, fmt.Errorf("examining %q for mounting in mount namespace: %w", target, err)
			}
			// The target isn't there yet, so create it.
			if srcinfo.IsDir() {
				if err = os.MkdirAll(target, 0755); err != nil {
					return undoBinds, fmt.Errorf("creating mountpoint %q in mount namespace: %w", target, err)
				}
			} else {
				if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
					return undoBinds, fmt.Errorf("ensuring parent of mountpoint %q (%q) is present in mount namespace: %w", target, filepath.Dir(target), err)
				}
				var file *os.File
				if file, err = os.OpenFile(target, os.O_WRONLY|os.O_CREATE, 0755); err != nil {
					return undoBinds, fmt.Errorf("creating mountpoint %q in mount namespace: %w", target, err)
				}
				file.Close()
			}
		}
		requestFlags := uintptr(0)
		expectedFlags := uintptr(0)
		for _, option := range m.Options {
			switch option {
			case "nodev":
				requestFlags |= unix.MS_NODEV
				expectedFlags |= unix.ST_NODEV
			case "dev":
				requestFlags &= ^uintptr(unix.MS_NODEV)
				expectedFlags &= ^uintptr(unix.ST_NODEV)
			case "noexec":
				requestFlags |= unix.MS_NOEXEC
				expectedFlags |= unix.ST_NOEXEC
			case "exec":
				requestFlags &= ^uintptr(unix.MS_NOEXEC)
				expectedFlags &= ^uintptr(unix.ST_NOEXEC)
			case "nosuid":
				requestFlags |= unix.MS_NOSUID
				expectedFlags |= unix.ST_NOSUID
			case "suid":
				requestFlags &= ^uintptr(unix.MS_NOSUID)
				expectedFlags &= ^uintptr(unix.ST_NOSUID)
			case "ro":
				requestFlags |= unix.MS_RDONLY
				expectedFlags |= unix.ST_RDONLY
			case "rw":
				requestFlags &= ^uintptr(unix.MS_RDONLY)
				expectedFlags &= ^uintptr(unix.ST_RDONLY)
			}
		}
		switch m.Type {
		case "bind":
			// Do the bind mount.
			logrus.Debugf("bind mounting %q on %q", m.Destination, filepath.Join(spec.Root.Path, m.Destination))
			if err := unix.Mount(m.Source, target, "", bindFlags|requestFlags, ""); err != nil {
				return undoBinds, fmt.Errorf("bind mounting %q from host to %q in mount namespace (%q): %w", m.Source, m.Destination, target, err)
			}
			if (requestFlags & unix.MS_RDONLY) != 0 {
				if err = unix.Statfs(target, &fs); err != nil {
					return undoBinds, fmt.Errorf("checking if directory %q was bound read-only: %w", target, err)
				}
				// we need to make sure these flags are maintained in the REMOUNT operation
				additionalFlags := uintptr(fs.Flags) & (unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_NODEV)
				if err := unix.Mount("", target, "", unix.MS_REMOUNT|unix.MS_BIND|unix.MS_RDONLY|additionalFlags, ""); err != nil {
					return undoBinds, fmt.Errorf("setting flags on the bind mount %q from host to %q in mount namespace (%q): %w", m.Source, m.Destination, target, err)
				}
			}
			logrus.Debugf("bind mounted %q to %q", m.Source, target)
		case "tmpfs":
			// Mount a tmpfs.
			if err := mount.Mount(m.Source, target, m.Type, strings.Join(append(m.Options, "private"), ",")); err != nil {
				return undoBinds, fmt.Errorf("mounting tmpfs to %q in mount namespace (%q, %q): %w", m.Destination, target, strings.Join(m.Options, ","), err)
			}
			logrus.Debugf("mounted a tmpfs to %q", target)
		case "overlay":
			// Mount a overlay.
			if err := mount.Mount(m.Source, target, m.Type, strings.Join(append(m.Options, "private"), ",")); err != nil {
				return undoBinds, fmt.Errorf("mounting overlay to %q in mount namespace (%q, %q): %w", m.Destination, target, strings.Join(m.Options, ","), err)
			}
			logrus.Debugf("mounted a overlay to %q", target)
		}
		if err = unix.Statfs(target, &fs); err != nil {
			return undoBinds, fmt.Errorf("checking if directory %q was bound read-only: %w", target, err)
		}
		if uintptr(fs.Flags)&expectedFlags != expectedFlags {
			if err := unix.Mount(target, target, "bind", requestFlags|unix.MS_REMOUNT, ""); err != nil {
				return undoBinds, fmt.Errorf("remounting %q in mount namespace with expected flags: %w", target, err)
			}
		}
	}

	// Set up any read-only paths that we need to.  If we're running inside
	// of a container, some of these locations will already be read-only.
	for _, roPath := range spec.Linux.ReadonlyPaths {
		r := filepath.Join(spec.Root.Path, roPath)
		target, err := filepath.EvalSymlinks(r)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// No target, no problem.
				continue
			}
			return undoBinds, fmt.Errorf("checking %q for symlinks before marking it read-only: %w", r, err)
		}
		// Check if the location is already read-only.
		var fs unix.Statfs_t
		if err = unix.Statfs(target, &fs); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// No target, no problem.
				continue
			}
			return undoBinds, fmt.Errorf("checking if directory %q is already read-only: %w", target, err)
		}
		if fs.Flags&unix.ST_RDONLY != 0 {
			continue
		}
		// Mount the location over itself, so that we can remount it as read-only.
		roFlags := uintptr(unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_RDONLY)
		if err := unix.Mount(target, target, "", roFlags|unix.MS_BIND|unix.MS_REC, ""); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// No target, no problem.
				continue
			}
			return undoBinds, fmt.Errorf("bind mounting %q onto itself in preparation for making it read-only: %w", target, err)
		}
		// Remount the location read-only.
		if err = unix.Statfs(target, &fs); err != nil {
			return undoBinds, fmt.Errorf("checking if directory %q was bound read-only: %w", target, err)
		}
		if fs.Flags&unix.ST_RDONLY == 0 {
			if err := unix.Mount(target, target, "", roFlags|unix.MS_BIND|unix.MS_REMOUNT, ""); err != nil {
				return undoBinds, fmt.Errorf("remounting %q in mount namespace read-only: %w", target, err)
			}
		}
		// Check again.
		if err = unix.Statfs(target, &fs); err != nil {
			return undoBinds, fmt.Errorf("checking if directory %q was remounted read-only: %w", target, err)
		}
		if fs.Flags&unix.ST_RDONLY == 0 {
			return undoBinds, fmt.Errorf("verifying that %q in mount namespace was remounted read-only: %w", target, err)
		}
	}

	// Create an empty directory for to use for masking directories.
	roEmptyDir := filepath.Join(bundlePath, "empty")
	if len(spec.Linux.MaskedPaths) > 0 {
		if err := os.Mkdir(roEmptyDir, 0700); err != nil {
			return undoBinds, fmt.Errorf("creating empty directory %q: %w", roEmptyDir, err)
		}
	}

	// Set up any masked paths that we need to.  If we're running inside of
	// a container, some of these locations will already be read-only tmpfs
	// filesystems or bind mounted to os.DevNull.  If we're not running
	// inside of a container, and nobody else has done that, we'll do it.
	for _, masked := range spec.Linux.MaskedPaths {
		t := filepath.Join(spec.Root.Path, masked)
		target, err := filepath.EvalSymlinks(t)
		if err != nil {
			target = t
		}
		// Get some info about the target.
		targetinfo, err := os.Stat(target)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// No target, no problem.
				continue
			}
			return undoBinds, fmt.Errorf("examining %q for masking in mount namespace: %w", target, err)
		}
		if targetinfo.IsDir() {
			// The target's a directory.  Check if it's a read-only filesystem.
			var statfs unix.Statfs_t
			if err = unix.Statfs(target, &statfs); err != nil {
				return undoBinds, fmt.Errorf("checking if directory %q is a mountpoint: %w", target, err)
			}
			isReadOnly := statfs.Flags&unix.MS_RDONLY != 0
			// Check if any of the IDs we're mapping could read it.
			var stat unix.Stat_t
			if err = unix.Stat(target, &stat); err != nil {
				return undoBinds, fmt.Errorf("checking permissions on directory %q: %w", target, err)
			}
			isAccessible := false
			if stat.Mode&unix.S_IROTH|unix.S_IXOTH != 0 {
				isAccessible = true
			}
			if !isAccessible && stat.Mode&unix.S_IROTH|unix.S_IXOTH != 0 {
				if len(spec.Linux.GIDMappings) > 0 {
					for _, mapping := range spec.Linux.GIDMappings {
						if stat.Gid >= mapping.ContainerID && stat.Gid < mapping.ContainerID+mapping.Size {
							isAccessible = true
							break
						}
					}
				}
			}
			if !isAccessible && stat.Mode&unix.S_IRUSR|unix.S_IXUSR != 0 {
				if len(spec.Linux.UIDMappings) > 0 {
					for _, mapping := range spec.Linux.UIDMappings {
						if stat.Uid >= mapping.ContainerID && stat.Uid < mapping.ContainerID+mapping.Size {
							isAccessible = true
							break
						}
					}
				}
			}
			// Check if it's empty.
			hasContent := false
			directory, err := os.Open(target)
			if err != nil {
				if !os.IsPermission(err) {
					return undoBinds, fmt.Errorf("opening directory %q: %w", target, err)
				}
			} else {
				names, err := directory.Readdirnames(0)
				directory.Close()
				if err != nil {
					return undoBinds, fmt.Errorf("reading contents of directory %q: %w", target, err)
				}
				hasContent = false
				for _, name := range names {
					switch name {
					case ".", "..":
						continue
					default:
						hasContent = true
					}
					if hasContent {
						break
					}
				}
			}
			// The target's a directory, so read-only bind mount an empty directory on it.
			roFlags := uintptr(syscall.MS_BIND | syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_RDONLY)
			if !isReadOnly || (hasContent && isAccessible) {
				if err = unix.Mount(roEmptyDir, target, "bind", roFlags, ""); err != nil {
					return undoBinds, fmt.Errorf("masking directory %q in mount namespace: %w", target, err)
				}
				if err = unix.Statfs(target, &fs); err != nil {
					return undoBinds, fmt.Errorf("checking if directory %q was mounted read-only in mount namespace: %w", target, err)
				}
				if fs.Flags&unix.ST_RDONLY == 0 {
					if err = unix.Mount(target, target, "", roFlags|syscall.MS_REMOUNT, ""); err != nil {
						return undoBinds, fmt.Errorf("making sure directory %q in mount namespace is read only: %w", target, err)
					}
				}
			}
		} else {
			// If the target's is not a directory or os.DevNull, bind mount os.DevNull over it.
			if !isDevNull(targetinfo) {
				if err = unix.Mount(os.DevNull, target, "", uintptr(syscall.MS_BIND|syscall.MS_RDONLY|syscall.MS_PRIVATE), ""); err != nil {
					return undoBinds, fmt.Errorf("masking non-directory %q in mount namespace: %w", target, err)
				}
			}
		}
	}
	return undoBinds, nil
}

// setPdeathsig sets a parent-death signal for the process
func setPdeathsig(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
