//go:build linux
// +build linux

package buildah

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/containers/buildah/bind"
	"github.com/containers/buildah/chroot"
	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/internal"
	internalParse "github.com/containers/buildah/internal/parse"
	internalUtil "github.com/containers/buildah/internal/util"
	"github.com/containers/buildah/pkg/overlay"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/pkg/sshagent"
	"github.com/containers/buildah/util"
	"github.com/containers/common/libnetwork/etchosts"
	"github.com/containers/common/libnetwork/network"
	nettypes "github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/capabilities"
	"github.com/containers/common/pkg/chown"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/subscriptions"
	imagetypes "github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/containers/storage/pkg/reexec"
	"github.com/containers/storage/pkg/stringid"
	"github.com/containers/storage/pkg/unshare"
	storagetypes "github.com/containers/storage/types"
	"github.com/docker/go-units"
	"github.com/docker/libnetwork/resolvconf"
	"github.com/docker/libnetwork/types"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/runtime-spec/specs-go"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// ContainerDevices is an alias for a slice of github.com/opencontainers/runc/libcontainer/configs.Device structures.
type ContainerDevices define.ContainerDevices

func setChildProcess() error {
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, uintptr(1), 0, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "prctl(PR_SET_CHILD_SUBREAPER, 1): %v\n", err)
		return err
	}
	return nil
}

// Run runs the specified command in the container's root filesystem.
func (b *Builder) Run(command []string, options RunOptions) error {
	p, err := ioutil.TempDir("", define.Package)
	if err != nil {
		return err
	}
	// On some hosts like AH, /tmp is a symlink and we need an
	// absolute path.
	path, err := filepath.EvalSymlinks(p)
	if err != nil {
		return err
	}
	logrus.Debugf("using %q to hold bundle data", path)
	defer func() {
		if err2 := os.RemoveAll(path); err2 != nil {
			options.Logger.Error(err2)
		}
	}()

	gp, err := generate.New("linux")
	if err != nil {
		return errors.Wrapf(err, "error generating new 'linux' runtime spec")
	}
	g := &gp

	isolation := options.Isolation
	if isolation == define.IsolationDefault {
		isolation = b.Isolation
		if isolation == define.IsolationDefault {
			isolation = define.IsolationOCI
		}
	}
	if err := checkAndOverrideIsolationOptions(isolation, &options); err != nil {
		return err
	}

	// hardwire the environment to match docker build to avoid subtle and hard-to-debug differences due to containers.conf
	b.configureEnvironment(g, options, []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"})

	if b.CommonBuildOpts == nil {
		return errors.Errorf("Invalid format on container you must recreate the container")
	}

	if err := addCommonOptsToSpec(b.CommonBuildOpts, g); err != nil {
		return err
	}

	if options.WorkingDir != "" {
		g.SetProcessCwd(options.WorkingDir)
	} else if b.WorkDir() != "" {
		g.SetProcessCwd(b.WorkDir())
	}
	setupSelinux(g, b.ProcessLabel, b.MountLabel)
	mountPoint, err := b.Mount(b.MountLabel)
	if err != nil {
		return errors.Wrapf(err, "error mounting container %q", b.ContainerID)
	}
	defer func() {
		if err := b.Unmount(); err != nil {
			options.Logger.Errorf("error unmounting container: %v", err)
		}
	}()
	g.SetRootPath(mountPoint)
	if len(command) > 0 {
		command = runLookupPath(g, command)
		g.SetProcessArgs(command)
	} else {
		g.SetProcessArgs(nil)
	}

	// Mount devices if any and if session is rootless attempt a bind-mount
	// just like podman.
	if unshare.IsRootless() {
		// We are going to create bind mounts for devices
		// but we need to make sure that we don't override
		// anything which is already in OCI spec.
		mounts := make(map[string]interface{})
		for _, m := range g.Mounts() {
			mounts[m.Destination] = true
		}
		newMounts := []spec.Mount{}
		for _, d := range b.Devices {
			// Default permission is read-only.
			perm := "ro"
			// Get permission configured for this device but only process `write`
			// permission in rootless since `mknod` is not supported anyways.
			if strings.Contains(string(d.Rule.Permissions), "w") {
				perm = "rw"
			}
			devMnt := spec.Mount{
				Destination: d.Destination,
				Type:        parse.TypeBind,
				Source:      d.Source,
				Options:     []string{"slave", "nosuid", "noexec", perm, "rbind"},
			}
			// Podman parity: podman skips these two devices hence we do the same.
			if d.Path == "/dev/ptmx" || strings.HasPrefix(d.Path, "/dev/tty") {
				continue
			}
			// Device is already in OCI spec do not re-mount.
			if _, found := mounts[d.Path]; found {
				continue
			}
			newMounts = append(newMounts, devMnt)
		}
		g.Config.Mounts = append(newMounts, g.Config.Mounts...)
	} else {
		for _, d := range b.Devices {
			sDev := spec.LinuxDevice{
				Type:     string(d.Type),
				Path:     d.Path,
				Major:    d.Major,
				Minor:    d.Minor,
				FileMode: &d.FileMode,
				UID:      &d.Uid,
				GID:      &d.Gid,
			}
			g.AddDevice(sDev)
			g.AddLinuxResourcesDevice(true, string(d.Type), &d.Major, &d.Minor, string(d.Permissions))
		}
	}

	setupMaskedPaths(g)
	setupReadOnlyPaths(g)

	setupTerminal(g, options.Terminal, options.TerminalSize)

	configureNetwork, configureNetworks, err := b.configureNamespaces(g, &options)
	if err != nil {
		return err
	}

	// rootless and networks are not supported
	if len(configureNetworks) > 0 && isolation == IsolationOCIRootless {
		return errors.New("cannot use networks as rootless")
	}

	homeDir, err := b.configureUIDGID(g, mountPoint, options)
	if err != nil {
		return err
	}

	g.SetProcessApparmorProfile(b.CommonBuildOpts.ApparmorProfile)

	// Now grab the spec from the generator.  Set the generator to nil so that future contributors
	// will quickly be able to tell that they're supposed to be modifying the spec directly from here.
	spec := g.Config
	g = nil

	// Set the seccomp configuration using the specified profile name.  Some syscalls are
	// allowed if certain capabilities are to be granted (example: CAP_SYS_CHROOT and chroot),
	// so we sorted out the capabilities lists first.
	if err = setupSeccomp(spec, b.CommonBuildOpts.SeccompProfilePath); err != nil {
		return err
	}

	uid, gid := spec.Process.User.UID, spec.Process.User.GID
	if spec.Linux != nil {
		uid, gid, err = util.GetHostIDs(spec.Linux.UIDMappings, spec.Linux.GIDMappings, uid, gid)
		if err != nil {
			return err
		}
	}

	idPair := &idtools.IDPair{UID: int(uid), GID: int(gid)}

	mode := os.FileMode(0755)
	coptions := copier.MkdirOptions{
		ChownNew: idPair,
		ChmodNew: &mode,
	}
	if err := copier.Mkdir(mountPoint, filepath.Join(mountPoint, spec.Process.Cwd), coptions); err != nil {
		return err
	}

	bindFiles := make(map[string]string)
	namespaceOptions := append(b.NamespaceOptions, options.NamespaceOptions...)
	volumes := b.Volumes()

	// Figure out who owns files that will appear to be owned by UID/GID 0 in the container.
	rootUID, rootGID, err := util.GetHostRootIDs(spec)
	if err != nil {
		return err
	}
	rootIDPair := &idtools.IDPair{UID: int(rootUID), GID: int(rootGID)}

	hostFile := ""
	if !options.NoHosts && !contains(volumes, config.DefaultHostsFile) && options.ConfigureNetwork != define.NetworkDisabled {
		hostFile, err = b.generateHosts(path, rootIDPair, mountPoint)
		if err != nil {
			return err
		}
		bindFiles[config.DefaultHostsFile] = hostFile
	}

	// generate /etc/hostname if the user intentionally did not override
	if !(contains(volumes, "/etc/hostname")) {
		if _, ok := bindFiles["/etc/hostname"]; !ok {
			hostFile, err := b.generateHostname(path, spec.Hostname, rootIDPair)
			if err != nil {
				return err
			}
			// Bind /etc/hostname
			bindFiles["/etc/hostname"] = hostFile
		}
	}

	if !(contains(volumes, "/etc/resolv.conf") || (len(b.CommonBuildOpts.DNSServers) == 1 && strings.ToLower(b.CommonBuildOpts.DNSServers[0]) == "none")) {
		resolvFile, err := b.addResolvConf(path, rootIDPair, b.CommonBuildOpts.DNSServers, b.CommonBuildOpts.DNSSearch, b.CommonBuildOpts.DNSOptions, namespaceOptions)
		if err != nil {
			return err
		}
		// Only bind /etc/resolv.conf if there's a network
		if options.ConfigureNetwork != define.NetworkDisabled {
			bindFiles["/etc/resolv.conf"] = resolvFile
		}
	}
	// Empty file, so no need to recreate if it exists
	if _, ok := bindFiles["/run/.containerenv"]; !ok {
		containerenvPath := filepath.Join(path, "/run/.containerenv")
		if err = os.MkdirAll(filepath.Dir(containerenvPath), 0755); err != nil {
			return err
		}

		rootless := 0
		if unshare.IsRootless() {
			rootless = 1
		}
		// Populate the .containerenv with container information
		containerenv := fmt.Sprintf(`
engine="buildah-%s"
name=%q
id=%q
image=%q
imageid=%q
rootless=%d
`, define.Version, b.Container, b.ContainerID, b.FromImage, b.FromImageID, rootless)

		if err = ioutils.AtomicWriteFile(containerenvPath, []byte(containerenv), 0755); err != nil {
			return err
		}
		if err := label.Relabel(containerenvPath, b.MountLabel, false); err != nil {
			return err
		}

		bindFiles["/run/.containerenv"] = containerenvPath
	}
	runArtifacts, err := b.setupMounts(options.SystemContext, mountPoint, spec, path, options.Mounts, bindFiles, volumes, b.CommonBuildOpts.Volumes, options.Secrets, options.SSHSources, options.RunMounts, options.ContextDir, options.StageMountPoints)
	if err != nil {
		return errors.Wrapf(err, "error resolving mountpoints for container %q", b.ContainerID)
	}
	if runArtifacts.SSHAuthSock != "" {
		sshenv := "SSH_AUTH_SOCK=" + runArtifacts.SSHAuthSock
		spec.Process.Env = append(spec.Process.Env, sshenv)
	}

	// following run was called from `buildah run`
	// and some images were mounted for this run
	// add them to cleanup artifacts
	if len(options.ExternalImageMounts) > 0 {
		runArtifacts.MountedImages = append(runArtifacts.MountedImages, options.ExternalImageMounts...)
	}

	defer func() {
		if err := b.cleanupRunMounts(options.SystemContext, mountPoint, runArtifacts); err != nil {
			options.Logger.Errorf("unable to cleanup run mounts %v", err)
		}
	}()

	defer b.cleanupTempVolumes()

	switch isolation {
	case define.IsolationOCI:
		var moreCreateArgs []string
		if options.NoPivot {
			moreCreateArgs = append(moreCreateArgs, "--no-pivot")
		}
		err = b.runUsingRuntimeSubproc(isolation, options, configureNetwork, configureNetworks, moreCreateArgs, spec,
			mountPoint, path, define.Package+"-"+filepath.Base(path), b.Container, hostFile)
	case IsolationChroot:
		err = chroot.RunUsingChroot(spec, path, homeDir, options.Stdin, options.Stdout, options.Stderr)
	case IsolationOCIRootless:
		moreCreateArgs := []string{"--no-new-keyring"}
		if options.NoPivot {
			moreCreateArgs = append(moreCreateArgs, "--no-pivot")
		}
		err = b.runUsingRuntimeSubproc(isolation, options, configureNetwork, configureNetworks, moreCreateArgs, spec,
			mountPoint, path, define.Package+"-"+filepath.Base(path), b.Container, hostFile)
	default:
		err = errors.Errorf("don't know how to run this command")
	}
	return err
}

func addCommonOptsToSpec(commonOpts *define.CommonBuildOptions, g *generate.Generator) error {
	// Resources - CPU
	if commonOpts.CPUPeriod != 0 {
		g.SetLinuxResourcesCPUPeriod(commonOpts.CPUPeriod)
	}
	if commonOpts.CPUQuota != 0 {
		g.SetLinuxResourcesCPUQuota(commonOpts.CPUQuota)
	}
	if commonOpts.CPUShares != 0 {
		g.SetLinuxResourcesCPUShares(commonOpts.CPUShares)
	}
	if commonOpts.CPUSetCPUs != "" {
		g.SetLinuxResourcesCPUCpus(commonOpts.CPUSetCPUs)
	}
	if commonOpts.CPUSetMems != "" {
		g.SetLinuxResourcesCPUMems(commonOpts.CPUSetMems)
	}

	// Resources - Memory
	if commonOpts.Memory != 0 {
		g.SetLinuxResourcesMemoryLimit(commonOpts.Memory)
	}
	if commonOpts.MemorySwap != 0 {
		g.SetLinuxResourcesMemorySwap(commonOpts.MemorySwap)
	}

	// cgroup membership
	if commonOpts.CgroupParent != "" {
		g.SetLinuxCgroupsPath(commonOpts.CgroupParent)
	}

	defaultContainerConfig, err := config.Default()
	if err != nil {
		return errors.Wrapf(err, "failed to get container config")
	}
	// Other process resource limits
	if err := addRlimits(commonOpts.Ulimit, g, defaultContainerConfig.Containers.DefaultUlimits); err != nil {
		return err
	}

	logrus.Debugf("Resources: %#v", commonOpts)
	return nil
}

func runSetupBuiltinVolumes(mountLabel, mountPoint, containerDir string, builtinVolumes []string, rootUID, rootGID int) ([]specs.Mount, error) {
	var mounts []specs.Mount
	hostOwner := idtools.IDPair{UID: rootUID, GID: rootGID}
	// Add temporary copies of the contents of volume locations at the
	// volume locations, unless we already have something there.
	for _, volume := range builtinVolumes {
		volumePath := filepath.Join(containerDir, "buildah-volumes", digest.Canonical.FromString(volume).Hex())
		initializeVolume := false
		// If we need to, create the directory that we'll use to hold
		// the volume contents.  If we do need to create it, then we'll
		// need to populate it, too, so make a note of that.
		if _, err := os.Stat(volumePath); err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
			logrus.Debugf("setting up built-in volume path at %q for %q", volumePath, volume)
			if err = os.MkdirAll(volumePath, 0755); err != nil {
				return nil, err
			}
			if err = label.Relabel(volumePath, mountLabel, false); err != nil {
				return nil, err
			}
			initializeVolume = true
		}
		// Make sure the volume exists in the rootfs and read its attributes.
		createDirPerms := os.FileMode(0755)
		err := copier.Mkdir(mountPoint, filepath.Join(mountPoint, volume), copier.MkdirOptions{
			ChownNew: &hostOwner,
			ChmodNew: &createDirPerms,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "ensuring volume path %q", filepath.Join(mountPoint, volume))
		}
		srcPath, err := copier.Eval(mountPoint, filepath.Join(mountPoint, volume), copier.EvalOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "evaluating path %q", srcPath)
		}
		stat, err := os.Stat(srcPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		// If we need to populate the mounted volume's contents with
		// content from the rootfs, set it up now.
		if initializeVolume {
			if err = os.Chmod(volumePath, stat.Mode().Perm()); err != nil {
				return nil, err
			}
			if err = os.Chown(volumePath, int(stat.Sys().(*syscall.Stat_t).Uid), int(stat.Sys().(*syscall.Stat_t).Gid)); err != nil {
				return nil, err
			}
			logrus.Debugf("populating directory %q for volume %q using contents of %q", volumePath, volume, srcPath)
			if err = extractWithTar(mountPoint, srcPath, volumePath); err != nil && !os.IsNotExist(errors.Cause(err)) {
				return nil, errors.Wrapf(err, "error populating directory %q for volume %q using contents of %q", volumePath, volume, srcPath)
			}
		}
		// Add the bind mount.
		mounts = append(mounts, specs.Mount{
			Source:      volumePath,
			Destination: volume,
			Type:        "bind",
			Options:     []string{"bind"},
		})
	}
	return mounts, nil
}

func (b *Builder) setupMounts(context *imagetypes.SystemContext, mountPoint string, spec *specs.Spec, bundlePath string, optionMounts []specs.Mount, bindFiles map[string]string, builtinVolumes, volumeMounts []string, secrets map[string]define.Secret, sshSources map[string]*sshagent.Source, runFileMounts []string, contextDir string, stageMountPoints map[string]internal.StageMountDetails) (*runMountArtifacts, error) {
	// Start building a new list of mounts.
	var mounts []specs.Mount
	haveMount := func(destination string) bool {
		for _, mount := range mounts {
			if mount.Destination == destination {
				// Already have something to mount there.
				return true
			}
		}
		return false
	}

	specMounts, err := setupSpecialMountSpecChanges(spec, b.CommonBuildOpts.ShmSize)
	if err != nil {
		return nil, err
	}

	// Get the list of files we need to bind into the container.
	bindFileMounts := runSetupBoundFiles(bundlePath, bindFiles)

	// After this point we need to know the per-container persistent storage directory.
	cdir, err := b.store.ContainerDirectory(b.ContainerID)
	if err != nil {
		return nil, errors.Wrapf(err, "error determining work directory for container %q", b.ContainerID)
	}

	// Figure out which UID and GID to tell the subscriptions package to use
	// for files that it creates.
	rootUID, rootGID, err := util.GetHostRootIDs(spec)
	if err != nil {
		return nil, err
	}

	// Get host UID and GID of the container process.
	processUID, processGID, err := util.GetHostIDs(spec.Linux.UIDMappings, spec.Linux.GIDMappings, spec.Process.User.UID, spec.Process.User.GID)
	if err != nil {
		return nil, err
	}

	// Get the list of subscriptions mounts.
	subscriptionMounts := subscriptions.MountsWithUIDGID(b.MountLabel, cdir, b.DefaultMountsFilePath, mountPoint, int(rootUID), int(rootGID), unshare.IsRootless(), false)

	// Get the list of mounts that are just for this Run() call.
	// TODO: acui: de-spaghettify run mounts
	runMounts, mountArtifacts, err := b.runSetupRunMounts(context, runFileMounts, secrets, stageMountPoints, sshSources, cdir, contextDir, spec.Linux.UIDMappings, spec.Linux.GIDMappings, int(rootUID), int(rootGID), int(processUID), int(processGID))
	if err != nil {
		return nil, err
	}
	// Add temporary copies of the contents of volume locations at the
	// volume locations, unless we already have something there.
	builtins, err := runSetupBuiltinVolumes(b.MountLabel, mountPoint, cdir, builtinVolumes, int(rootUID), int(rootGID))
	if err != nil {
		return nil, err
	}

	// Get the list of explicitly-specified volume mounts.
	volumes, err := b.runSetupVolumeMounts(spec.Linux.MountLabel, volumeMounts, optionMounts, int(rootUID), int(rootGID), int(processUID), int(processGID))
	if err != nil {
		return nil, err
	}

	// prepare list of mount destinations which can be cleaned up safely.
	// we can clean bindFiles, subscriptionMounts and specMounts
	// everything other than these might have users content
	mountArtifacts.RunMountTargets = append(append(append(mountArtifacts.RunMountTargets, cleanableDestinationListFromMounts(bindFileMounts)...), cleanableDestinationListFromMounts(subscriptionMounts)...), cleanableDestinationListFromMounts(specMounts)...)

	allMounts := util.SortMounts(append(append(append(append(append(volumes, builtins...), runMounts...), subscriptionMounts...), bindFileMounts...), specMounts...))
	// Add them all, in the preferred order, except where they conflict with something that was previously added.
	for _, mount := range allMounts {
		if haveMount(mount.Destination) {
			// Already mounting something there, no need to bother with this one.
			continue
		}
		// Add the mount.
		mounts = append(mounts, mount)
	}

	// Set the list in the spec.
	spec.Mounts = mounts
	return mountArtifacts, nil
}

// Destinations which can be cleaned up after every RUN
func cleanableDestinationListFromMounts(mounts []spec.Mount) []string {
	mountDest := []string{}
	for _, mount := range mounts {
		// Add all destination to mountArtifacts so that they can be cleaned up later
		if mount.Destination != "" {
			// we dont want to remove destinations with  /etc, /dev, /sys, /proc as rootfs already contains these files
			// and unionfs will create a `whiteout` i.e `.wh` files on removal of overlapping files from these directories.
			// everything other than these will be cleanedup
			if !strings.HasPrefix(mount.Destination, "/etc") && !strings.HasPrefix(mount.Destination, "/dev") && !strings.HasPrefix(mount.Destination, "/sys") && !strings.HasPrefix(mount.Destination, "/proc") {
				mountDest = append(mountDest, mount.Destination)
			}
		}
	}
	return mountDest
}

// addResolvConf copies files from host and sets them up to bind mount into container
func (b *Builder) addResolvConf(rdir string, chownOpts *idtools.IDPair, dnsServers, dnsSearch, dnsOptions []string, namespaceOptions define.NamespaceOptions) (string, error) {
	resolvConf := "/etc/resolv.conf"

	stat, err := os.Stat(resolvConf)
	if err != nil {
		return "", err
	}
	contents, err := ioutil.ReadFile(resolvConf)
	// resolv.conf doesn't have to exists
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}

	netns := false
	ns := namespaceOptions.Find(string(spec.NetworkNamespace))
	if ns != nil && !ns.Host {
		netns = true
	}

	nameservers := resolvconf.GetNameservers(contents, types.IPv4)
	// check if systemd-resolved is used, assume it is used when 127.0.0.53 is the only nameserver
	if len(nameservers) == 1 && nameservers[0] == "127.0.0.53" && netns {
		// read the actual resolv.conf file for systemd-resolved
		resolvedContents, err := ioutil.ReadFile("/run/systemd/resolve/resolv.conf")
		if err != nil {
			if !os.IsNotExist(err) {
				return "", errors.Wrapf(err, "detected that systemd-resolved is in use, but could not locate real resolv.conf")
			}
		} else {
			contents = resolvedContents
		}
	}

	// Ensure that the container's /etc/resolv.conf is compatible with its
	// network configuration.
	if netns {
		// FIXME handle IPv6
		resolve, err := resolvconf.FilterResolvDNS(contents, true)
		if err != nil {
			return "", errors.Wrapf(err, "error parsing host resolv.conf")
		}
		contents = resolve.Content
	}
	search := resolvconf.GetSearchDomains(contents)
	nameservers = resolvconf.GetNameservers(contents, types.IP)
	options := resolvconf.GetOptions(contents)

	defaultContainerConfig, err := config.Default()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get container config")
	}
	dnsSearch = append(defaultContainerConfig.Containers.DNSSearches, dnsSearch...)
	if len(dnsSearch) > 0 {
		search = dnsSearch
	}

	if b.Isolation == IsolationOCIRootless {
		if ns != nil && !ns.Host && ns.Path == "" {
			// if we are using slirp4netns, also add the built-in DNS server.
			logrus.Debugf("adding slirp4netns 10.0.2.3 built-in DNS server")
			nameservers = append([]string{"10.0.2.3"}, nameservers...)
		}
	}

	dnsServers = append(defaultContainerConfig.Containers.DNSServers, dnsServers...)
	if len(dnsServers) != 0 {
		dns, err := getDNSIP(dnsServers)
		if err != nil {
			return "", errors.Wrapf(err, "error getting dns servers")
		}
		nameservers = []string{}
		for _, server := range dns {
			nameservers = append(nameservers, server.String())
		}
	}

	dnsOptions = append(defaultContainerConfig.Containers.DNSOptions, dnsOptions...)
	if len(dnsOptions) != 0 {
		options = dnsOptions
	}

	cfile := filepath.Join(rdir, filepath.Base(resolvConf))
	if _, err = resolvconf.Build(cfile, nameservers, search, options); err != nil {
		return "", errors.Wrapf(err, "error building resolv.conf for container %s", b.ContainerID)
	}

	uid := int(stat.Sys().(*syscall.Stat_t).Uid)
	gid := int(stat.Sys().(*syscall.Stat_t).Gid)
	if chownOpts != nil {
		uid = chownOpts.UID
		gid = chownOpts.GID
	}
	if err = os.Chown(cfile, uid, gid); err != nil {
		return "", err
	}

	if err := label.Relabel(cfile, b.MountLabel, false); err != nil {
		return "", err
	}
	return cfile, nil
}

// generateHosts creates a containers hosts file
func (b *Builder) generateHosts(rdir string, chownOpts *idtools.IDPair, imageRoot string) (string, error) {
	conf, err := config.Default()
	if err != nil {
		return "", err
	}

	path, err := etchosts.GetBaseHostFile(conf.Containers.BaseHostsFile, imageRoot)
	if err != nil {
		return "", err
	}

	targetfile := filepath.Join(rdir, "hosts")
	if err := etchosts.New(&etchosts.Params{
		BaseFile:                 path,
		ExtraHosts:               b.CommonBuildOpts.AddHost,
		HostContainersInternalIP: etchosts.GetHostContainersInternalIP(conf, nil, nil),
		TargetFile:               targetfile,
	}); err != nil {
		return "", err
	}

	uid := 0
	gid := 0
	if chownOpts != nil {
		uid = chownOpts.UID
		gid = chownOpts.GID
	}
	if err = os.Chown(targetfile, uid, gid); err != nil {
		return "", err
	}
	if err := label.Relabel(targetfile, b.MountLabel, false); err != nil {
		return "", err
	}

	return targetfile, nil
}

// generateHostname creates a containers /etc/hostname file
func (b *Builder) generateHostname(rdir, hostname string, chownOpts *idtools.IDPair) (string, error) {
	var err error
	hostnamePath := "/etc/hostname"

	var hostnameBuffer bytes.Buffer
	hostnameBuffer.Write([]byte(fmt.Sprintf("%s\n", hostname)))

	cfile := filepath.Join(rdir, filepath.Base(hostnamePath))
	if err = ioutils.AtomicWriteFile(cfile, hostnameBuffer.Bytes(), 0644); err != nil {
		return "", errors.Wrapf(err, "error writing /etc/hostname into the container")
	}

	uid := 0
	gid := 0
	if chownOpts != nil {
		uid = chownOpts.UID
		gid = chownOpts.GID
	}
	if err = os.Chown(cfile, uid, gid); err != nil {
		return "", err
	}
	if err := label.Relabel(cfile, b.MountLabel, false); err != nil {
		return "", err
	}

	return cfile, nil
}

func setupTerminal(g *generate.Generator, terminalPolicy TerminalPolicy, terminalSize *specs.Box) {
	switch terminalPolicy {
	case DefaultTerminal:
		onTerminal := term.IsTerminal(unix.Stdin) && term.IsTerminal(unix.Stdout) && term.IsTerminal(unix.Stderr)
		if onTerminal {
			logrus.Debugf("stdio is a terminal, defaulting to using a terminal")
		} else {
			logrus.Debugf("stdio is not a terminal, defaulting to not using a terminal")
		}
		g.SetProcessTerminal(onTerminal)
	case WithTerminal:
		g.SetProcessTerminal(true)
	case WithoutTerminal:
		g.SetProcessTerminal(false)
	}
	if terminalSize != nil {
		g.SetProcessConsoleSize(terminalSize.Width, terminalSize.Height)
	}
}

func runUsingRuntime(options RunOptions, configureNetwork bool, moreCreateArgs []string, spec *specs.Spec, bundlePath, containerName string,
	containerCreateW io.WriteCloser, containerStartR io.ReadCloser) (wstatus unix.WaitStatus, err error) {
	if options.Logger == nil {
		options.Logger = logrus.StandardLogger()
	}

	// Lock the caller to a single OS-level thread.
	runtime.LockOSThread()

	// Set up bind mounts for things that a namespaced user might not be able to get to directly.
	unmountAll, err := bind.SetupIntermediateMountNamespace(spec, bundlePath)
	if unmountAll != nil {
		defer func() {
			if err := unmountAll(); err != nil {
				options.Logger.Error(err)
			}
		}()
	}
	if err != nil {
		return 1, err
	}

	// Write the runtime configuration.
	specbytes, err := json.Marshal(spec)
	if err != nil {
		return 1, errors.Wrapf(err, "error encoding configuration %#v as json", spec)
	}
	if err = ioutils.AtomicWriteFile(filepath.Join(bundlePath, "config.json"), specbytes, 0600); err != nil {
		return 1, errors.Wrapf(err, "error storing runtime configuration")
	}

	logrus.Debugf("config = %v", string(specbytes))

	// Decide which runtime to use.
	runtime := options.Runtime
	if runtime == "" {
		runtime = util.Runtime()
	}
	localRuntime := util.FindLocalRuntime(runtime)
	if localRuntime != "" {
		runtime = localRuntime
	}

	// Default to just passing down our stdio.
	getCreateStdio := func() (io.ReadCloser, io.WriteCloser, io.WriteCloser) {
		return os.Stdin, os.Stdout, os.Stderr
	}

	// Figure out how we're doing stdio handling, and create pipes and sockets.
	var stdio sync.WaitGroup
	var consoleListener *net.UnixListener
	var errorFds, closeBeforeReadingErrorFds []int
	stdioPipe := make([][]int, 3)
	copyConsole := false
	copyPipes := false
	finishCopy := make([]int, 2)
	if err = unix.Pipe(finishCopy); err != nil {
		return 1, errors.Wrapf(err, "error creating pipe for notifying to stop stdio")
	}
	finishedCopy := make(chan struct{}, 1)
	var pargs []string
	if spec.Process != nil {
		pargs = spec.Process.Args
		if spec.Process.Terminal {
			copyConsole = true
			// Create a listening socket for accepting the container's terminal's PTY master.
			socketPath := filepath.Join(bundlePath, "console.sock")
			consoleListener, err = net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
			if err != nil {
				return 1, errors.Wrapf(err, "error creating socket %q to receive terminal descriptor", consoleListener.Addr())
			}
			// Add console socket arguments.
			moreCreateArgs = append(moreCreateArgs, "--console-socket", socketPath)
		} else {
			copyPipes = true
			// Figure out who should own the pipes.
			uid, gid, err := util.GetHostRootIDs(spec)
			if err != nil {
				return 1, err
			}
			// Create stdio pipes.
			if stdioPipe, err = runMakeStdioPipe(int(uid), int(gid)); err != nil {
				return 1, err
			}
			if err = runLabelStdioPipes(stdioPipe, spec.Process.SelinuxLabel, spec.Linux.MountLabel); err != nil {
				return 1, err
			}
			errorFds = []int{stdioPipe[unix.Stdout][0], stdioPipe[unix.Stderr][0]}
			closeBeforeReadingErrorFds = []int{stdioPipe[unix.Stdout][1], stdioPipe[unix.Stderr][1]}
			// Set stdio to our pipes.
			getCreateStdio = func() (io.ReadCloser, io.WriteCloser, io.WriteCloser) {
				stdin := os.NewFile(uintptr(stdioPipe[unix.Stdin][0]), "/dev/stdin")
				stdout := os.NewFile(uintptr(stdioPipe[unix.Stdout][1]), "/dev/stdout")
				stderr := os.NewFile(uintptr(stdioPipe[unix.Stderr][1]), "/dev/stderr")
				return stdin, stdout, stderr
			}
		}
	} else {
		if options.Quiet {
			// Discard stdout.
			getCreateStdio = func() (io.ReadCloser, io.WriteCloser, io.WriteCloser) {
				return os.Stdin, nil, os.Stderr
			}
		}
	}

	runtimeArgs := options.Args[:]
	if options.CgroupManager == config.SystemdCgroupsManager {
		runtimeArgs = append(runtimeArgs, "--systemd-cgroup")
	}

	// Build the commands that we'll execute.
	pidFile := filepath.Join(bundlePath, "pid")
	args := append(append(append(runtimeArgs, "create", "--bundle", bundlePath, "--pid-file", pidFile), moreCreateArgs...), containerName)
	create := exec.Command(runtime, args...)
	setPdeathsig(create)
	create.Dir = bundlePath
	stdin, stdout, stderr := getCreateStdio()
	create.Stdin, create.Stdout, create.Stderr = stdin, stdout, stderr

	args = append(options.Args, "start", containerName)
	start := exec.Command(runtime, args...)
	setPdeathsig(start)
	start.Dir = bundlePath
	start.Stderr = os.Stderr

	kill := func(signal string) *exec.Cmd {
		args := append(options.Args, "kill", containerName)
		if signal != "" {
			args = append(args, signal)
		}
		kill := exec.Command(runtime, args...)
		kill.Dir = bundlePath
		kill.Stderr = os.Stderr
		return kill
	}

	args = append(options.Args, "delete", containerName)
	del := exec.Command(runtime, args...)
	del.Dir = bundlePath
	del.Stderr = os.Stderr

	// Actually create the container.
	logrus.Debugf("Running %q", create.Args)
	err = create.Run()
	if err != nil {
		return 1, errors.Wrapf(err, "error from %s creating container for %v: %s", runtime, pargs, runCollectOutput(options.Logger, errorFds, closeBeforeReadingErrorFds))
	}
	defer func() {
		err2 := del.Run()
		if err2 != nil {
			if err == nil {
				err = errors.Wrapf(err2, "error deleting container")
			} else {
				options.Logger.Infof("error from %s deleting container: %v", runtime, err2)
			}
		}
	}()

	// Make sure we read the container's exit status when it exits.
	pidValue, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return 1, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidValue)))
	if err != nil {
		return 1, errors.Wrapf(err, "error parsing pid %s as a number", string(pidValue))
	}
	var stopped uint32
	var reaping sync.WaitGroup
	reaping.Add(1)
	go func() {
		defer reaping.Done()
		var err error
		_, err = unix.Wait4(pid, &wstatus, 0, nil)
		if err != nil {
			wstatus = 0
			options.Logger.Errorf("error waiting for container child process %d: %v\n", pid, err)
		}
		atomic.StoreUint32(&stopped, 1)
	}()

	if configureNetwork {
		if _, err := containerCreateW.Write([]byte{1}); err != nil {
			return 1, err
		}
		containerCreateW.Close()
		logrus.Debug("waiting for parent start message")
		b := make([]byte, 1)
		if _, err := containerStartR.Read(b); err != nil {
			return 1, errors.Wrap(err, "did not get container start message from parent")
		}
		containerStartR.Close()
	}

	if copyPipes {
		// We don't need the ends of the pipes that belong to the container.
		stdin.Close()
		if stdout != nil {
			stdout.Close()
		}
		stderr.Close()
	}

	// Handle stdio for the container in the background.
	stdio.Add(1)
	go runCopyStdio(options.Logger, &stdio, copyPipes, stdioPipe, copyConsole, consoleListener, finishCopy, finishedCopy, spec)

	// Start the container.
	logrus.Debugf("Running %q", start.Args)
	err = start.Run()
	if err != nil {
		return 1, errors.Wrapf(err, "error from %s starting container", runtime)
	}
	defer func() {
		if atomic.LoadUint32(&stopped) == 0 {
			if err := kill("").Run(); err != nil {
				options.Logger.Infof("error from %s stopping container: %v", runtime, err)
			}
			atomic.StoreUint32(&stopped, 1)
		}
	}()

	// Wait for the container to exit.
	interrupted := make(chan os.Signal, 100)
	go func() {
		for range interrupted {
			if err := kill("SIGKILL").Run(); err != nil {
				logrus.Errorf("%v sending SIGKILL", err)
			}
		}
	}()
	signal.Notify(interrupted, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	for {
		now := time.Now()
		var state specs.State
		args = append(options.Args, "state", containerName)
		stat := exec.Command(runtime, args...)
		stat.Dir = bundlePath
		stat.Stderr = os.Stderr
		stateOutput, err := stat.Output()
		if err != nil {
			if atomic.LoadUint32(&stopped) != 0 {
				// container exited
				break
			}
			return 1, errors.Wrapf(err, "error reading container state from %s (got output: %q)", runtime, string(stateOutput))
		}
		if err = json.Unmarshal(stateOutput, &state); err != nil {
			return 1, errors.Wrapf(err, "error parsing container state %q from %s", string(stateOutput), runtime)
		}
		switch state.Status {
		case "running":
		case "stopped":
			atomic.StoreUint32(&stopped, 1)
		default:
			return 1, errors.Errorf("container status unexpectedly changed to %q", state.Status)
		}
		if atomic.LoadUint32(&stopped) != 0 {
			break
		}
		select {
		case <-finishedCopy:
			atomic.StoreUint32(&stopped, 1)
		case <-time.After(time.Until(now.Add(100 * time.Millisecond))):
			continue
		}
		if atomic.LoadUint32(&stopped) != 0 {
			break
		}
	}
	signal.Stop(interrupted)
	close(interrupted)

	// Close the writing end of the stop-handling-stdio notification pipe.
	unix.Close(finishCopy[1])
	// Wait for the stdio copy goroutine to flush.
	stdio.Wait()
	// Wait until we finish reading the exit status.
	reaping.Wait()

	return wstatus, nil
}

func runCollectOutput(logger *logrus.Logger, fds, closeBeforeReadingFds []int) string { //nolint:interfacer
	for _, fd := range closeBeforeReadingFds {
		unix.Close(fd)
	}
	var b bytes.Buffer
	buf := make([]byte, 8192)
	for _, fd := range fds {
		nread, err := unix.Read(fd, buf)
		if err != nil {
			if errno, isErrno := err.(syscall.Errno); isErrno {
				switch errno {
				default:
					logger.Errorf("error reading from pipe %d: %v", fd, err)
				case syscall.EINTR, syscall.EAGAIN:
				}
			} else {
				logger.Errorf("unable to wait for data from pipe %d: %v", fd, err)
			}
			continue
		}
		for nread > 0 {
			r := buf[:nread]
			if nwritten, err := b.Write(r); err != nil || nwritten != len(r) {
				if nwritten != len(r) {
					logger.Errorf("error buffering data from pipe %d: %v", fd, err)
					break
				}
			}
			nread, err = unix.Read(fd, buf)
			if err != nil {
				if errno, isErrno := err.(syscall.Errno); isErrno {
					switch errno {
					default:
						logger.Errorf("error reading from pipe %d: %v", fd, err)
					case syscall.EINTR, syscall.EAGAIN:
					}
				} else {
					logger.Errorf("unable to wait for data from pipe %d: %v", fd, err)
				}
				break
			}
		}
	}
	return b.String()
}

func setupRootlessNetwork(pid int) (teardown func(), err error) {
	slirp4netns, err := exec.LookPath("slirp4netns")
	if err != nil {
		return nil, err
	}

	rootlessSlirpSyncR, rootlessSlirpSyncW, err := os.Pipe()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create slirp4netns sync pipe")
	}
	defer rootlessSlirpSyncR.Close()

	// Be sure there are no fds inherited to slirp4netns except the sync pipe
	files, err := ioutil.ReadDir("/proc/self/fd")
	if err != nil {
		return nil, errors.Wrapf(err, "cannot list open fds")
	}
	for _, f := range files {
		fd, err := strconv.Atoi(f.Name())
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse fd")
		}
		if fd == int(rootlessSlirpSyncW.Fd()) {
			continue
		}
		unix.CloseOnExec(fd)
	}

	cmd := exec.Command(slirp4netns, "--mtu", "65520", "-r", "3", "-c", strconv.Itoa(pid), "tap0")
	setPdeathsig(cmd)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	cmd.ExtraFiles = []*os.File{rootlessSlirpSyncW}

	err = cmd.Start()
	rootlessSlirpSyncW.Close()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot start slirp4netns")
	}

	b := make([]byte, 1)
	for {
		if err := rootlessSlirpSyncR.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
			return nil, errors.Wrapf(err, "error setting slirp4netns pipe timeout")
		}
		if _, err := rootlessSlirpSyncR.Read(b); err == nil {
			break
		} else {
			if os.IsTimeout(err) {
				// Check if the process is still running.
				var status syscall.WaitStatus
				_, err := syscall.Wait4(cmd.Process.Pid, &status, syscall.WNOHANG, nil)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to read slirp4netns process status")
				}
				if status.Exited() || status.Signaled() {
					return nil, errors.New("slirp4netns failed")
				}

				continue
			}
			return nil, errors.Wrapf(err, "failed to read from slirp4netns sync pipe")
		}
	}

	return func() {
		cmd.Process.Kill() // nolint:errcheck
		cmd.Wait()         // nolint:errcheck
	}, nil
}

func (b *Builder) runConfigureNetwork(pid int, isolation define.Isolation, options RunOptions, configureNetworks []string, containerName string) (teardown func(), netStatus map[string]nettypes.StatusBlock, err error) {
	if isolation == IsolationOCIRootless {
		teardown, err = setupRootlessNetwork(pid)
		return teardown, nil, err
	}

	if len(configureNetworks) == 0 {
		configureNetworks = []string{b.NetworkInterface.DefaultNetworkName()}
	}

	// Make sure we can access the container's network namespace,
	// even after it exits, to successfully tear down the
	// interfaces.  Ensure this by opening a handle to the network
	// namespace, and using our copy to both configure and
	// deconfigure it.
	netns := fmt.Sprintf("/proc/%d/ns/net", pid)
	netFD, err := unix.Open(netns, unix.O_RDONLY, 0)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error opening network namespace")
	}
	mynetns := fmt.Sprintf("/proc/%d/fd/%d", unix.Getpid(), netFD)

	networks := make(map[string]nettypes.PerNetworkOptions, len(configureNetworks))
	for i, network := range configureNetworks {
		networks[network] = nettypes.PerNetworkOptions{
			InterfaceName: fmt.Sprintf("eth%d", i),
		}
	}

	opts := nettypes.NetworkOptions{
		ContainerID:   containerName,
		ContainerName: containerName,
		Networks:      networks,
	}
	netStatus, err = b.NetworkInterface.Setup(mynetns, nettypes.SetupOptions{NetworkOptions: opts})
	if err != nil {
		return nil, nil, err
	}

	teardown = func() {
		err := b.NetworkInterface.Teardown(mynetns, nettypes.TeardownOptions{NetworkOptions: opts})
		if err != nil {
			options.Logger.Errorf("failed to cleanup network: %v", err)
		}
	}

	return teardown, netStatus, nil
}

func setNonblock(logger *logrus.Logger, fd int, description string, nonblocking bool) (bool, error) { //nolint:interfacer
	mask, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		return false, err
	}
	blocked := mask&unix.O_NONBLOCK == 0

	if err := unix.SetNonblock(fd, nonblocking); err != nil {
		if nonblocking {
			logger.Errorf("error setting %s to nonblocking: %v", description, err)
		} else {
			logger.Errorf("error setting descriptor %s blocking: %v", description, err)
		}
	}
	return blocked, err
}

func runCopyStdio(logger *logrus.Logger, stdio *sync.WaitGroup, copyPipes bool, stdioPipe [][]int, copyConsole bool, consoleListener *net.UnixListener, finishCopy []int, finishedCopy chan struct{}, spec *specs.Spec) {
	defer func() {
		unix.Close(finishCopy[0])
		if copyPipes {
			unix.Close(stdioPipe[unix.Stdin][1])
			unix.Close(stdioPipe[unix.Stdout][0])
			unix.Close(stdioPipe[unix.Stderr][0])
		}
		stdio.Done()
		finishedCopy <- struct{}{}
		close(finishedCopy)
	}()
	// Map describing where data on an incoming descriptor should go.
	relayMap := make(map[int]int)
	// Map describing incoming and outgoing descriptors.
	readDesc := make(map[int]string)
	writeDesc := make(map[int]string)
	// Buffers.
	relayBuffer := make(map[int]*bytes.Buffer)
	// Set up the terminal descriptor or pipes for polling.
	if copyConsole {
		// Accept a connection over our listening socket.
		fd, err := runAcceptTerminal(logger, consoleListener, spec.Process.ConsoleSize)
		if err != nil {
			logger.Errorf("%v", err)
			return
		}
		terminalFD := fd
		// Input from our stdin, output from the terminal descriptor.
		relayMap[unix.Stdin] = terminalFD
		readDesc[unix.Stdin] = "stdin"
		relayBuffer[terminalFD] = new(bytes.Buffer)
		writeDesc[terminalFD] = "container terminal input"
		relayMap[terminalFD] = unix.Stdout
		readDesc[terminalFD] = "container terminal output"
		relayBuffer[unix.Stdout] = new(bytes.Buffer)
		writeDesc[unix.Stdout] = "output"
		// Set our terminal's mode to raw, to pass handling of special
		// terminal input to the terminal in the container.
		if term.IsTerminal(unix.Stdin) {
			if state, err := term.MakeRaw(unix.Stdin); err != nil {
				logger.Warnf("error setting terminal state: %v", err)
			} else {
				defer func() {
					if err = term.Restore(unix.Stdin, state); err != nil {
						logger.Errorf("unable to restore terminal state: %v", err)
					}
				}()
			}
		}
	}
	if copyPipes {
		// Input from our stdin, output from the stdout and stderr pipes.
		relayMap[unix.Stdin] = stdioPipe[unix.Stdin][1]
		readDesc[unix.Stdin] = "stdin"
		relayBuffer[stdioPipe[unix.Stdin][1]] = new(bytes.Buffer)
		writeDesc[stdioPipe[unix.Stdin][1]] = "container stdin"
		relayMap[stdioPipe[unix.Stdout][0]] = unix.Stdout
		readDesc[stdioPipe[unix.Stdout][0]] = "container stdout"
		relayBuffer[unix.Stdout] = new(bytes.Buffer)
		writeDesc[unix.Stdout] = "stdout"
		relayMap[stdioPipe[unix.Stderr][0]] = unix.Stderr
		readDesc[stdioPipe[unix.Stderr][0]] = "container stderr"
		relayBuffer[unix.Stderr] = new(bytes.Buffer)
		writeDesc[unix.Stderr] = "stderr"
	}
	// Set our reading descriptors to non-blocking.
	for rfd, wfd := range relayMap {
		blocked, err := setNonblock(logger, rfd, readDesc[rfd], true)
		if err != nil {
			return
		}
		if blocked {
			defer setNonblock(logger, rfd, readDesc[rfd], false) // nolint:errcheck
		}
		setNonblock(logger, wfd, writeDesc[wfd], false) // nolint:errcheck
	}

	if copyPipes {
		setNonblock(logger, stdioPipe[unix.Stdin][1], writeDesc[stdioPipe[unix.Stdin][1]], true) // nolint:errcheck
	}

	runCopyStdioPassData(copyPipes, stdioPipe, finishCopy, relayMap, relayBuffer, readDesc, writeDesc)
}

func canRetry(err error) bool {
	if errno, isErrno := err.(syscall.Errno); isErrno {
		return errno == syscall.EINTR || errno == syscall.EAGAIN
	}
	return false
}

func runCopyStdioPassData(copyPipes bool, stdioPipe [][]int, finishCopy []int, relayMap map[int]int, relayBuffer map[int]*bytes.Buffer, readDesc map[int]string, writeDesc map[int]string) {
	closeStdin := false

	// Pass data back and forth.
	pollTimeout := -1
	for len(relayMap) > 0 {
		// Start building the list of descriptors to poll.
		pollFds := make([]unix.PollFd, 0, len(relayMap)+1)
		// Poll for a notification that we should stop handling stdio.
		pollFds = append(pollFds, unix.PollFd{Fd: int32(finishCopy[0]), Events: unix.POLLIN | unix.POLLHUP})
		// Poll on our reading descriptors.
		for rfd := range relayMap {
			pollFds = append(pollFds, unix.PollFd{Fd: int32(rfd), Events: unix.POLLIN | unix.POLLHUP})
		}
		buf := make([]byte, 8192)
		// Wait for new data from any input descriptor, or a notification that we're done.
		_, err := unix.Poll(pollFds, pollTimeout)
		if !util.LogIfNotRetryable(err, fmt.Sprintf("error waiting for stdio/terminal data to relay: %v", err)) {
			return
		}
		removes := make(map[int]struct{})
		for _, pollFd := range pollFds {
			// If this descriptor's just been closed from the other end, mark it for
			// removal from the set that we're checking for.
			if pollFd.Revents&unix.POLLHUP == unix.POLLHUP {
				removes[int(pollFd.Fd)] = struct{}{}
			}
			// If the descriptor was closed elsewhere, remove it from our list.
			if pollFd.Revents&unix.POLLNVAL != 0 {
				logrus.Debugf("error polling descriptor %s: closed?", readDesc[int(pollFd.Fd)])
				removes[int(pollFd.Fd)] = struct{}{}
			}
			// If the POLLIN flag isn't set, then there's no data to be read from this descriptor.
			if pollFd.Revents&unix.POLLIN == 0 {
				continue
			}
			// Read whatever there is to be read.
			readFD := int(pollFd.Fd)
			writeFD, needToRelay := relayMap[readFD]
			if needToRelay {
				n, err := unix.Read(readFD, buf)
				if !util.LogIfNotRetryable(err, fmt.Sprintf("unable to read %s data: %v", readDesc[readFD], err)) {
					return
				}
				// If it's zero-length on our stdin and we're
				// using pipes, it's an EOF, so close the stdin
				// pipe's writing end.
				if n == 0 && !canRetry(err) && int(pollFd.Fd) == unix.Stdin {
					removes[int(pollFd.Fd)] = struct{}{}
				} else if n > 0 {
					// Buffer the data in case we get blocked on where they need to go.
					nwritten, err := relayBuffer[writeFD].Write(buf[:n])
					if err != nil {
						logrus.Debugf("buffer: %v", err)
						continue
					}
					if nwritten != n {
						logrus.Debugf("buffer: expected to buffer %d bytes, wrote %d", n, nwritten)
						continue
					}
					// If this is the last of the data we'll be able to read from this
					// descriptor, read all that there is to read.
					for pollFd.Revents&unix.POLLHUP == unix.POLLHUP {
						nr, err := unix.Read(readFD, buf)
						util.LogIfUnexpectedWhileDraining(err, fmt.Sprintf("read %s: %v", readDesc[readFD], err))
						if nr <= 0 {
							break
						}
						nwritten, err := relayBuffer[writeFD].Write(buf[:nr])
						if err != nil {
							logrus.Debugf("buffer: %v", err)
							break
						}
						if nwritten != nr {
							logrus.Debugf("buffer: expected to buffer %d bytes, wrote %d", nr, nwritten)
							break
						}
					}
				}
			}
		}
		// Try to drain the output buffers.  Set the default timeout
		// for the next poll() to 100ms if we still have data to write.
		pollTimeout = -1
		for writeFD := range relayBuffer {
			if relayBuffer[writeFD].Len() > 0 {
				n, err := unix.Write(writeFD, relayBuffer[writeFD].Bytes())
				if !util.LogIfNotRetryable(err, fmt.Sprintf("unable to write %s data: %v", writeDesc[writeFD], err)) {
					return
				}
				if n > 0 {
					relayBuffer[writeFD].Next(n)
				}
				if closeStdin && writeFD == stdioPipe[unix.Stdin][1] && stdioPipe[unix.Stdin][1] >= 0 && relayBuffer[stdioPipe[unix.Stdin][1]].Len() == 0 {
					logrus.Debugf("closing stdin")
					unix.Close(stdioPipe[unix.Stdin][1])
					stdioPipe[unix.Stdin][1] = -1
				}
			}
			if relayBuffer[writeFD].Len() > 0 {
				pollTimeout = 100
			}
		}
		// Remove any descriptors which we don't need to poll any more from the poll descriptor list.
		for remove := range removes {
			if copyPipes && remove == unix.Stdin {
				closeStdin = true
				if relayBuffer[stdioPipe[unix.Stdin][1]].Len() == 0 {
					logrus.Debugf("closing stdin")
					unix.Close(stdioPipe[unix.Stdin][1])
					stdioPipe[unix.Stdin][1] = -1
				}
			}
			delete(relayMap, remove)
		}
		// If the we-can-return pipe had anything for us, we're done.
		for _, pollFd := range pollFds {
			if int(pollFd.Fd) == finishCopy[0] && pollFd.Revents != 0 {
				// The pipe is closed, indicating that we can stop now.
				return
			}
		}
	}
}

func runAcceptTerminal(logger *logrus.Logger, consoleListener *net.UnixListener, terminalSize *specs.Box) (int, error) {
	defer consoleListener.Close()
	c, err := consoleListener.AcceptUnix()
	if err != nil {
		return -1, errors.Wrapf(err, "error accepting socket descriptor connection")
	}
	defer c.Close()
	// Expect a control message over our new connection.
	b := make([]byte, 8192)
	oob := make([]byte, 8192)
	n, oobn, _, _, err := c.ReadMsgUnix(b, oob)
	if err != nil {
		return -1, errors.Wrapf(err, "error reading socket descriptor")
	}
	if n > 0 {
		logrus.Debugf("socket descriptor is for %q", string(b[:n]))
	}
	if oobn > len(oob) {
		return -1, errors.Errorf("too much out-of-bounds data (%d bytes)", oobn)
	}
	// Parse the control message.
	scm, err := unix.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return -1, errors.Wrapf(err, "error parsing out-of-bound data as a socket control message")
	}
	logrus.Debugf("control messages: %v", scm)
	// Expect to get a descriptor.
	terminalFD := -1
	for i := range scm {
		fds, err := unix.ParseUnixRights(&scm[i])
		if err != nil {
			return -1, errors.Wrapf(err, "error parsing unix rights control message: %v", &scm[i])
		}
		logrus.Debugf("fds: %v", fds)
		if len(fds) == 0 {
			continue
		}
		terminalFD = fds[0]
		break
	}
	if terminalFD == -1 {
		return -1, errors.Errorf("unable to read terminal descriptor")
	}
	// Set the pseudoterminal's size to the configured size, or our own.
	winsize := &unix.Winsize{}
	if terminalSize != nil {
		// Use configured sizes.
		winsize.Row = uint16(terminalSize.Height)
		winsize.Col = uint16(terminalSize.Width)
	} else {
		if term.IsTerminal(unix.Stdin) {
			// Use the size of our terminal.
			if winsize, err = unix.IoctlGetWinsize(unix.Stdin, unix.TIOCGWINSZ); err != nil {
				logger.Warnf("error reading size of controlling terminal: %v", err)
				winsize.Row = 0
				winsize.Col = 0
			}
		}
	}
	if winsize.Row != 0 && winsize.Col != 0 {
		if err = unix.IoctlSetWinsize(terminalFD, unix.TIOCSWINSZ, winsize); err != nil {
			logger.Warnf("error setting size of container pseudoterminal: %v", err)
		}
		// FIXME - if we're connected to a terminal, we should
		// be passing the updated terminal size down when we
		// receive a SIGWINCH.
	}
	return terminalFD, nil
}

// Create pipes to use for relaying stdio.
func runMakeStdioPipe(uid, gid int) ([][]int, error) {
	stdioPipe := make([][]int, 3)
	for i := range stdioPipe {
		stdioPipe[i] = make([]int, 2)
		if err := unix.Pipe(stdioPipe[i]); err != nil {
			return nil, errors.Wrapf(err, "error creating pipe for container FD %d", i)
		}
	}
	if err := unix.Fchown(stdioPipe[unix.Stdin][0], uid, gid); err != nil {
		return nil, errors.Wrapf(err, "error setting owner of stdin pipe descriptor")
	}
	if err := unix.Fchown(stdioPipe[unix.Stdout][1], uid, gid); err != nil {
		return nil, errors.Wrapf(err, "error setting owner of stdout pipe descriptor")
	}
	if err := unix.Fchown(stdioPipe[unix.Stderr][1], uid, gid); err != nil {
		return nil, errors.Wrapf(err, "error setting owner of stderr pipe descriptor")
	}
	return stdioPipe, nil
}

func runUsingRuntimeMain() {
	var options runUsingRuntimeSubprocOptions
	// Set logging.
	if level := os.Getenv("LOGLEVEL"); level != "" {
		if ll, err := strconv.Atoi(level); err == nil {
			logrus.SetLevel(logrus.Level(ll))
		}
	}
	// Unpack our configuration.
	confPipe := os.NewFile(3, "confpipe")
	if confPipe == nil {
		fmt.Fprintf(os.Stderr, "error reading options pipe\n")
		os.Exit(1)
	}
	defer confPipe.Close()
	if err := json.NewDecoder(confPipe).Decode(&options); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding options: %v\n", err)
		os.Exit(1)
	}
	// Set ourselves up to read the container's exit status.  We're doing this in a child process
	// so that we won't mess with the setting in a caller of the library.
	if err := setChildProcess(); err != nil {
		os.Exit(1)
	}
	ospec := options.Spec
	if ospec == nil {
		fmt.Fprintf(os.Stderr, "options spec not specified\n")
		os.Exit(1)
	}

	// open the pipes used to communicate with the parent process
	var containerCreateW *os.File
	var containerStartR *os.File
	if options.ConfigureNetwork {
		containerCreateW = os.NewFile(4, "containercreatepipe")
		if containerCreateW == nil {
			fmt.Fprintf(os.Stderr, "could not open fd 4\n")
			os.Exit(1)
		}
		containerStartR = os.NewFile(5, "containerstartpipe")
		if containerStartR == nil {
			fmt.Fprintf(os.Stderr, "could not open fd 5\n")
			os.Exit(1)
		}
	}

	// Run the container, start to finish.
	status, err := runUsingRuntime(options.Options, options.ConfigureNetwork, options.MoreCreateArgs, ospec, options.BundlePath, options.ContainerName, containerCreateW, containerStartR)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running container: %v\n", err)
		os.Exit(1)
	}
	// Pass the container's exit status back to the caller by exiting with the same status.
	if status.Exited() {
		os.Exit(status.ExitStatus())
	} else if status.Signaled() {
		fmt.Fprintf(os.Stderr, "container exited on %s\n", status.Signal())
		os.Exit(1)
	}
	os.Exit(1)
}

func setupNamespaces(logger *logrus.Logger, g *generate.Generator, namespaceOptions define.NamespaceOptions, idmapOptions define.IDMappingOptions, policy define.NetworkConfigurationPolicy) (configureNetwork bool, configureNetworks []string, configureUTS bool, err error) {
	// Set namespace options in the container configuration.
	configureUserns := false
	specifiedNetwork := false
	for _, namespaceOption := range namespaceOptions {
		switch namespaceOption.Name {
		case string(specs.UserNamespace):
			configureUserns = false
			if !namespaceOption.Host && namespaceOption.Path == "" {
				configureUserns = true
			}
		case string(specs.NetworkNamespace):
			specifiedNetwork = true
			configureNetwork = false
			if !namespaceOption.Host && (namespaceOption.Path == "" || !filepath.IsAbs(namespaceOption.Path)) {
				if namespaceOption.Path != "" && !filepath.IsAbs(namespaceOption.Path) {
					configureNetworks = strings.Split(namespaceOption.Path, ",")
					namespaceOption.Path = ""
				}
				configureNetwork = (policy != define.NetworkDisabled)
			}
		case string(specs.UTSNamespace):
			configureUTS = false
			if !namespaceOption.Host && namespaceOption.Path == "" {
				configureUTS = true
			}
		}
		if namespaceOption.Host {
			if err := g.RemoveLinuxNamespace(namespaceOption.Name); err != nil {
				return false, nil, false, errors.Wrapf(err, "error removing %q namespace for run", namespaceOption.Name)
			}
		} else if err := g.AddOrReplaceLinuxNamespace(namespaceOption.Name, namespaceOption.Path); err != nil {
			if namespaceOption.Path == "" {
				return false, nil, false, errors.Wrapf(err, "error adding new %q namespace for run", namespaceOption.Name)
			}
			return false, nil, false, errors.Wrapf(err, "error adding %q namespace %q for run", namespaceOption.Name, namespaceOption.Path)
		}
	}

	// If we've got mappings, we're going to have to create a user namespace.
	if len(idmapOptions.UIDMap) > 0 || len(idmapOptions.GIDMap) > 0 || configureUserns {
		if err := g.AddOrReplaceLinuxNamespace(string(specs.UserNamespace), ""); err != nil {
			return false, nil, false, errors.Wrapf(err, "error adding new %q namespace for run", string(specs.UserNamespace))
		}
		hostUidmap, hostGidmap, err := unshare.GetHostIDMappings("")
		if err != nil {
			return false, nil, false, err
		}
		for _, m := range idmapOptions.UIDMap {
			g.AddLinuxUIDMapping(m.HostID, m.ContainerID, m.Size)
		}
		if len(idmapOptions.UIDMap) == 0 {
			for _, m := range hostUidmap {
				g.AddLinuxUIDMapping(m.ContainerID, m.ContainerID, m.Size)
			}
		}
		for _, m := range idmapOptions.GIDMap {
			g.AddLinuxGIDMapping(m.HostID, m.ContainerID, m.Size)
		}
		if len(idmapOptions.GIDMap) == 0 {
			for _, m := range hostGidmap {
				g.AddLinuxGIDMapping(m.ContainerID, m.ContainerID, m.Size)
			}
		}
		if !specifiedNetwork {
			if err := g.AddOrReplaceLinuxNamespace(string(specs.NetworkNamespace), ""); err != nil {
				return false, nil, false, errors.Wrapf(err, "error adding new %q namespace for run", string(specs.NetworkNamespace))
			}
			configureNetwork = (policy != define.NetworkDisabled)
		}
	} else {
		if err := g.RemoveLinuxNamespace(string(specs.UserNamespace)); err != nil {
			return false, nil, false, errors.Wrapf(err, "error removing %q namespace for run", string(specs.UserNamespace))
		}
		if !specifiedNetwork {
			if err := g.RemoveLinuxNamespace(string(specs.NetworkNamespace)); err != nil {
				return false, nil, false, errors.Wrapf(err, "error removing %q namespace for run", string(specs.NetworkNamespace))
			}
		}
	}
	if configureNetwork && !unshare.IsRootless() {
		for name, val := range define.DefaultNetworkSysctl {
			// Check that the sysctl we are adding is actually supported
			// by the kernel
			p := filepath.Join("/proc/sys", strings.Replace(name, ".", "/", -1))
			_, err := os.Stat(p)
			if err != nil && !os.IsNotExist(err) {
				return false, nil, false, err
			}
			if err == nil {
				g.AddLinuxSysctl(name, val)
			} else {
				logger.Warnf("ignoring sysctl %s since %s doesn't exist", name, p)
			}
		}
	}
	return configureNetwork, configureNetworks, configureUTS, nil
}

func (b *Builder) configureNamespaces(g *generate.Generator, options *RunOptions) (bool, []string, error) {
	defaultNamespaceOptions, err := DefaultNamespaceOptions()
	if err != nil {
		return false, nil, err
	}

	namespaceOptions := defaultNamespaceOptions
	namespaceOptions.AddOrReplace(b.NamespaceOptions...)
	namespaceOptions.AddOrReplace(options.NamespaceOptions...)

	networkPolicy := options.ConfigureNetwork
	//Nothing was specified explicitly so network policy should be inherited from builder
	if networkPolicy == NetworkDefault {
		networkPolicy = b.ConfigureNetwork

		// If builder policy was NetworkDisabled and
		// we want to disable network for this run.
		// reset options.ConfigureNetwork to NetworkDisabled
		// since it will be treated as source of truth later.
		if networkPolicy == NetworkDisabled {
			options.ConfigureNetwork = networkPolicy
		}
	}

	configureNetwork, configureNetworks, configureUTS, err := setupNamespaces(options.Logger, g, namespaceOptions, b.IDMappingOptions, networkPolicy)
	if err != nil {
		return false, nil, err
	}

	if configureUTS {
		if options.Hostname != "" {
			g.SetHostname(options.Hostname)
		} else if b.Hostname() != "" {
			g.SetHostname(b.Hostname())
		} else {
			g.SetHostname(stringid.TruncateID(b.ContainerID))
		}
	} else {
		g.SetHostname("")
	}

	found := false
	spec := g.Config
	for i := range spec.Process.Env {
		if strings.HasPrefix(spec.Process.Env[i], "HOSTNAME=") {
			found = true
			break
		}
	}
	if !found {
		spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("HOSTNAME=%s", spec.Hostname))
	}

	return configureNetwork, configureNetworks, nil
}

func runSetupBoundFiles(bundlePath string, bindFiles map[string]string) (mounts []specs.Mount) {
	for dest, src := range bindFiles {
		options := []string{"rbind"}
		if strings.HasPrefix(src, bundlePath) {
			options = append(options, bind.NoBindOption)
		}
		mounts = append(mounts, specs.Mount{
			Source:      src,
			Destination: dest,
			Type:        "bind",
			Options:     options,
		})
	}
	return mounts
}

func addRlimits(ulimit []string, g *generate.Generator, defaultUlimits []string) error {
	var (
		ul  *units.Ulimit
		err error
	)

	ulimit = append(defaultUlimits, ulimit...)
	for _, u := range ulimit {
		if ul, err = units.ParseUlimit(u); err != nil {
			return errors.Wrapf(err, "ulimit option %q requires name=SOFT:HARD, failed to be parsed", u)
		}

		g.AddProcessRlimits("RLIMIT_"+strings.ToUpper(ul.Name), uint64(ul.Hard), uint64(ul.Soft))
	}
	return nil
}

func (b *Builder) cleanupTempVolumes() {
	for tempVolume, val := range b.TempVolumes {
		if val {
			if err := overlay.RemoveTemp(tempVolume); err != nil {
				b.Logger.Errorf(err.Error())
			}
			b.TempVolumes[tempVolume] = false
		}
	}
}

func (b *Builder) runSetupVolumeMounts(mountLabel string, volumeMounts []string, optionMounts []specs.Mount, rootUID, rootGID, processUID, processGID int) (mounts []specs.Mount, Err error) {
	// Make sure the overlay directory is clean before running
	containerDir, err := b.store.ContainerDirectory(b.ContainerID)
	if err != nil {
		return nil, errors.Wrapf(err, "error looking up container directory for %s", b.ContainerID)
	}
	if err := overlay.CleanupContent(containerDir); err != nil {
		return nil, errors.Wrapf(err, "error cleaning up overlay content for %s", b.ContainerID)
	}

	parseMount := func(mountType, host, container string, options []string) (specs.Mount, error) {
		var foundrw, foundro, foundz, foundZ, foundO, foundU bool
		var rootProp, upperDir, workDir string
		for _, opt := range options {
			switch opt {
			case "rw":
				foundrw = true
			case "ro":
				foundro = true
			case "z":
				foundz = true
			case "Z":
				foundZ = true
			case "O":
				foundO = true
			case "U":
				foundU = true
			case "private", "rprivate", "slave", "rslave", "shared", "rshared":
				rootProp = opt
			}

			if strings.HasPrefix(opt, "upperdir") {
				splitOpt := strings.SplitN(opt, "=", 2)
				if len(splitOpt) > 1 {
					upperDir = splitOpt[1]
				}
			}
			if strings.HasPrefix(opt, "workdir") {
				splitOpt := strings.SplitN(opt, "=", 2)
				if len(splitOpt) > 1 {
					workDir = splitOpt[1]
				}
			}
		}
		if !foundrw && !foundro {
			options = append(options, "rw")
		}
		if foundz {
			if err := label.Relabel(host, mountLabel, true); err != nil {
				return specs.Mount{}, err
			}
		}
		if foundZ {
			if err := label.Relabel(host, mountLabel, false); err != nil {
				return specs.Mount{}, err
			}
		}
		if foundU {
			if err := chown.ChangeHostPathOwnership(host, true, processUID, processGID); err != nil {
				return specs.Mount{}, err
			}
		}
		if foundO {
			if (upperDir != "" && workDir == "") || (workDir != "" && upperDir == "") {
				return specs.Mount{}, errors.New("if specifying upperdir then workdir must be specified or vice versa")
			}

			containerDir, err := b.store.ContainerDirectory(b.ContainerID)
			if err != nil {
				return specs.Mount{}, err
			}

			contentDir, err := overlay.TempDir(containerDir, rootUID, rootGID)
			if err != nil {
				return specs.Mount{}, errors.Wrapf(err, "failed to create TempDir in the %s directory", containerDir)
			}

			overlayOpts := overlay.Options{RootUID: rootUID,
				RootGID:                rootGID,
				UpperDirOptionFragment: upperDir,
				WorkDirOptionFragment:  workDir,
				GraphOpts:              b.store.GraphOptions(),
			}

			overlayMount, err := overlay.MountWithOptions(contentDir, host, container, &overlayOpts)
			if err == nil {
				b.TempVolumes[contentDir] = true
			}

			// If chown true, add correct ownership to the overlay temp directories.
			if foundU {
				if err := chown.ChangeHostPathOwnership(contentDir, true, processUID, processGID); err != nil {
					return specs.Mount{}, err
				}
			}

			return overlayMount, err
		}
		if rootProp == "" {
			options = append(options, "private")
		}
		if mountType != "tmpfs" {
			mountType = "bind"
			options = append(options, "rbind")
		}
		return specs.Mount{
			Destination: container,
			Type:        mountType,
			Source:      host,
			Options:     options,
		}, nil
	}

	// Bind mount volumes specified for this particular Run() invocation
	for _, i := range optionMounts {
		logrus.Debugf("setting up mounted volume at %q", i.Destination)
		mount, err := parseMount(i.Type, i.Source, i.Destination, i.Options)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}
	// Bind mount volumes given by the user when the container was created
	for _, i := range volumeMounts {
		var options []string
		spliti := parse.SplitStringWithColonEscape(i)
		if len(spliti) > 2 {
			options = strings.Split(spliti[2], ",")
		}
		options = append(options, "rbind")
		mount, err := parseMount("bind", spliti[0], spliti[1], options)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

func setupMaskedPaths(g *generate.Generator) {
	for _, mp := range []string{
		"/proc/acpi",
		"/proc/kcore",
		"/proc/keys",
		"/proc/latency_stats",
		"/proc/timer_list",
		"/proc/timer_stats",
		"/proc/sched_debug",
		"/proc/scsi",
		"/sys/firmware",
		"/sys/fs/selinux",
		"/sys/dev",
	} {
		g.AddLinuxMaskedPaths(mp)
	}
}

func setupReadOnlyPaths(g *generate.Generator) {
	for _, rp := range []string{
		"/proc/asound",
		"/proc/bus",
		"/proc/fs",
		"/proc/irq",
		"/proc/sys",
		"/proc/sysrq-trigger",
	} {
		g.AddLinuxReadonlyPaths(rp)
	}
}

func setupCapAdd(g *generate.Generator, caps ...string) error {
	for _, cap := range caps {
		if err := g.AddProcessCapabilityBounding(cap); err != nil {
			return errors.Wrapf(err, "error adding %q to the bounding capability set", cap)
		}
		if err := g.AddProcessCapabilityEffective(cap); err != nil {
			return errors.Wrapf(err, "error adding %q to the effective capability set", cap)
		}
		if err := g.AddProcessCapabilityPermitted(cap); err != nil {
			return errors.Wrapf(err, "error adding %q to the permitted capability set", cap)
		}
		if err := g.AddProcessCapabilityAmbient(cap); err != nil {
			return errors.Wrapf(err, "error adding %q to the ambient capability set", cap)
		}
	}
	return nil
}

func setupCapDrop(g *generate.Generator, caps ...string) error {
	for _, cap := range caps {
		if err := g.DropProcessCapabilityBounding(cap); err != nil {
			return errors.Wrapf(err, "error removing %q from the bounding capability set", cap)
		}
		if err := g.DropProcessCapabilityEffective(cap); err != nil {
			return errors.Wrapf(err, "error removing %q from the effective capability set", cap)
		}
		if err := g.DropProcessCapabilityPermitted(cap); err != nil {
			return errors.Wrapf(err, "error removing %q from the permitted capability set", cap)
		}
		if err := g.DropProcessCapabilityAmbient(cap); err != nil {
			return errors.Wrapf(err, "error removing %q from the ambient capability set", cap)
		}
	}
	return nil
}

func setupCapabilities(g *generate.Generator, defaultCapabilities, adds, drops []string) error {
	g.ClearProcessCapabilities()
	if err := setupCapAdd(g, defaultCapabilities...); err != nil {
		return err
	}
	for _, c := range adds {
		if strings.ToLower(c) == "all" {
			adds = capabilities.AllCapabilities()
			break
		}
	}
	for _, c := range drops {
		if strings.ToLower(c) == "all" {
			g.ClearProcessCapabilities()
			return nil
		}
	}
	if err := setupCapAdd(g, adds...); err != nil {
		return err
	}
	return setupCapDrop(g, drops...)
}

// Search for a command that isn't given as an absolute path using the $PATH
// under the rootfs.  We can't resolve absolute symbolic links without
// chroot()ing, which we may not be able to do, so just accept a link as a
// valid resolution.
func runLookupPath(g *generate.Generator, command []string) []string {
	// Look for the configured $PATH.
	spec := g.Config
	envPath := ""
	for i := range spec.Process.Env {
		if strings.HasPrefix(spec.Process.Env[i], "PATH=") {
			envPath = spec.Process.Env[i]
		}
	}
	// If there is no configured $PATH, supply one.
	if envPath == "" {
		defaultPath := "/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin"
		envPath = "PATH=" + defaultPath
		g.AddProcessEnv("PATH", defaultPath)
	}
	// No command, nothing to do.
	if len(command) == 0 {
		return command
	}
	// Command is already an absolute path, use it as-is.
	if filepath.IsAbs(command[0]) {
		return command
	}
	// For each element in the PATH,
	for _, pathEntry := range filepath.SplitList(envPath[5:]) {
		// if it's the empty string, it's ".", which is the Cwd,
		if pathEntry == "" {
			pathEntry = spec.Process.Cwd
		}
		// build the absolute path which it might be,
		candidate := filepath.Join(pathEntry, command[0])
		// check if it's there,
		if fi, err := os.Lstat(filepath.Join(spec.Root.Path, candidate)); fi != nil && err == nil {
			// and if it's not a directory, and either a symlink or executable,
			if !fi.IsDir() && ((fi.Mode()&os.ModeSymlink != 0) || (fi.Mode()&0111 != 0)) {
				// use that.
				return append([]string{candidate}, command[1:]...)
			}
		}
	}
	return command
}

func getDNSIP(dnsServers []string) (dns []net.IP, err error) {
	for _, i := range dnsServers {
		result := net.ParseIP(i)
		if result == nil {
			return dns, errors.Errorf("invalid IP address %s", i)
		}
		dns = append(dns, result)
	}
	return dns, nil
}

func (b *Builder) configureUIDGID(g *generate.Generator, mountPoint string, options RunOptions) (string, error) {
	// Set the user UID/GID/supplemental group list/capabilities lists.
	user, homeDir, err := b.userForRun(mountPoint, options.User)
	if err != nil {
		return "", err
	}
	if err := setupCapabilities(g, b.Capabilities, options.AddCapabilities, options.DropCapabilities); err != nil {
		return "", err
	}
	g.SetProcessUID(user.UID)
	g.SetProcessGID(user.GID)
	for _, gid := range user.AdditionalGids {
		g.AddProcessAdditionalGid(gid)
	}

	// Remove capabilities if not running as root except Bounding set
	if user.UID != 0 {
		bounding := g.Config.Process.Capabilities.Bounding
		g.ClearProcessCapabilities()
		g.Config.Process.Capabilities.Bounding = bounding
	}

	return homeDir, nil
}

func (b *Builder) configureEnvironment(g *generate.Generator, options RunOptions, defaultEnv []string) {
	g.ClearProcessEnv()

	if b.CommonBuildOpts.HTTPProxy {
		for _, envSpec := range config.ProxyEnv {
			if envVal, ok := os.LookupEnv(envSpec); ok {
				g.AddProcessEnv(envSpec, envVal)
			}
		}
	}

	for _, envSpec := range util.MergeEnv(util.MergeEnv(defaultEnv, b.Env()), options.Env) {
		env := strings.SplitN(envSpec, "=", 2)
		if len(env) > 1 {
			g.AddProcessEnv(env[0], env[1])
		}
	}
}

func addOrReplaceMount(moutns []specs.Mount, mount specs.Mount) []spec.Mount {
	for i := range moutns {
		if moutns[i].Destination == mount.Destination {
			moutns[i] = mount
			return moutns
		}
	}
	return append(moutns, mount)
}

// setupSpecialMountSpecChanges creates special mounts for depending on the namespaces
// logic taken from podman and adapted for buildah
// https://github.com/containers/podman/blob/4ba71f955a944790edda6e007e6d074009d437a7/pkg/specgen/generate/oci.go#L178
func setupSpecialMountSpecChanges(spec *spec.Spec, shmSize string) ([]specs.Mount, error) {
	mounts := spec.Mounts
	isRootless := unshare.IsRootless()
	isNewUserns := false
	isNetns := false
	isPidns := false
	isIpcns := false

	for _, namespace := range spec.Linux.Namespaces {
		switch namespace.Type {
		case specs.NetworkNamespace:
			isNetns = true
		case specs.UserNamespace:
			isNewUserns = true
		case specs.PIDNamespace:
			isPidns = true
		case specs.IPCNamespace:
			isIpcns = true
		}
	}

	addCgroup := true
	// mount sys when root and no userns or when both netns and userns are private
	canMountSys := (!isRootless && !isNewUserns) || (isNetns && isNewUserns)
	if !canMountSys {
		addCgroup = false
		sys := "/sys"
		sysMnt := specs.Mount{
			Destination: sys,
			Type:        "bind",
			Source:      sys,
			Options:     []string{bind.NoBindOption, "rprivate", "nosuid", "noexec", "nodev", "ro", "rbind"},
		}
		mounts = addOrReplaceMount(mounts, sysMnt)
	}

	gid5Available := true
	if isRootless {
		_, gids, err := unshare.GetHostIDMappings("")
		if err != nil {
			return nil, err
		}
		gid5Available = checkIdsGreaterThan5(gids)
	}
	if gid5Available && len(spec.Linux.GIDMappings) > 0 {
		gid5Available = checkIdsGreaterThan5(spec.Linux.GIDMappings)
	}
	if !gid5Available {
		// If we have no GID mappings, the gid=5 default option would fail, so drop it.
		devPts := specs.Mount{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"rprivate", "nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"},
		}
		mounts = addOrReplaceMount(mounts, devPts)
	}

	isUserns := isNewUserns || isRootless

	if isUserns && !isIpcns {
		devMqueue := "/dev/mqueue"
		devMqueueMnt := specs.Mount{
			Destination: devMqueue,
			Type:        "bind",
			Source:      devMqueue,
			Options:     []string{bind.NoBindOption, "bind", "nosuid", "noexec", "nodev"},
		}
		mounts = addOrReplaceMount(mounts, devMqueueMnt)
	}
	if isUserns && !isPidns {
		proc := "/proc"
		procMount := specs.Mount{
			Destination: proc,
			Type:        "bind",
			Source:      proc,
			Options:     []string{bind.NoBindOption, "rbind", "nosuid", "noexec", "nodev"},
		}
		mounts = addOrReplaceMount(mounts, procMount)
	}

	if addCgroup {
		cgroupMnt := specs.Mount{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"rprivate", "nosuid", "noexec", "nodev", "relatime", "rw"},
		}
		mounts = addOrReplaceMount(mounts, cgroupMnt)
	}

	// if userns and host ipc bind mount shm
	if isUserns && !isIpcns {
		// bind mount /dev/shm when it exists
		if _, err := os.Stat("/dev/shm"); err == nil {
			shmMount := specs.Mount{
				Source:      "/dev/shm",
				Type:        "bind",
				Destination: "/dev/shm",
				Options:     []string{bind.NoBindOption, "rbind", "nosuid", "noexec", "nodev"},
			}
			mounts = addOrReplaceMount(mounts, shmMount)
		}
	} else if shmSize != "" {
		shmMount := specs.Mount{
			Source:      "shm",
			Destination: "/dev/shm",
			Type:        "tmpfs",
			Options:     []string{"private", "nodev", "noexec", "nosuid", "mode=1777", "size=" + shmSize},
		}
		mounts = addOrReplaceMount(mounts, shmMount)
	}

	return mounts, nil
}

func checkIdsGreaterThan5(ids []spec.LinuxIDMapping) bool {
	for _, r := range ids {
		if r.ContainerID <= 5 && 5 < r.ContainerID+r.Size {
			return true
		}
	}
	return false
}

func (b *Builder) runUsingRuntimeSubproc(isolation define.Isolation, options RunOptions, configureNetwork bool, configureNetworks,
	moreCreateArgs []string, spec *specs.Spec, rootPath, bundlePath, containerName, buildContainerName, hostsFile string) (err error) {
	var confwg sync.WaitGroup
	config, conferr := json.Marshal(runUsingRuntimeSubprocOptions{
		Options:          options,
		Spec:             spec,
		RootPath:         rootPath,
		BundlePath:       bundlePath,
		ConfigureNetwork: configureNetwork,
		MoreCreateArgs:   moreCreateArgs,
		ContainerName:    containerName,
		Isolation:        isolation,
	})
	if conferr != nil {
		return errors.Wrapf(conferr, "error encoding configuration for %q", runUsingRuntimeCommand)
	}
	cmd := reexec.Command(runUsingRuntimeCommand)
	setPdeathsig(cmd)
	cmd.Dir = bundlePath
	cmd.Stdin = options.Stdin
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout = options.Stdout
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = options.Stderr
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	cmd.Env = util.MergeEnv(os.Environ(), []string{fmt.Sprintf("LOGLEVEL=%d", logrus.GetLevel())})
	preader, pwriter, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "error creating configuration pipe")
	}
	confwg.Add(1)
	go func() {
		_, conferr = io.Copy(pwriter, bytes.NewReader(config))
		if conferr != nil {
			conferr = errors.Wrapf(conferr, "error while copying configuration down pipe to child process")
		}
		confwg.Done()
	}()

	// create network configuration pipes
	var containerCreateR, containerCreateW fileCloser
	var containerStartR, containerStartW fileCloser
	if configureNetwork {
		containerCreateR.file, containerCreateW.file, err = os.Pipe()
		if err != nil {
			return errors.Wrapf(err, "error creating container create pipe")
		}
		defer containerCreateR.Close()
		defer containerCreateW.Close()

		containerStartR.file, containerStartW.file, err = os.Pipe()
		if err != nil {
			return errors.Wrapf(err, "error creating container create pipe")
		}
		defer containerStartR.Close()
		defer containerStartW.Close()
		cmd.ExtraFiles = []*os.File{containerCreateW.file, containerStartR.file}
	}

	cmd.ExtraFiles = append([]*os.File{preader}, cmd.ExtraFiles...)
	defer preader.Close()
	defer pwriter.Close()
	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "error while starting runtime")
	}

	interrupted := make(chan os.Signal, 100)
	go func() {
		for receivedSignal := range interrupted {
			if err := cmd.Process.Signal(receivedSignal); err != nil {
				logrus.Infof("%v while attempting to forward %v to child process", err, receivedSignal)
			}
		}
	}()
	signal.Notify(interrupted, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	if configureNetwork {
		// we already passed the fd to the child, now close the writer so we do not hang if the child closes it
		containerCreateW.Close()
		if err := waitForSync(containerCreateR.file); err != nil {
			// we do not want to return here since we want to capture the exit code from the child via cmd.Wait()
			// close the pipes here so that the child will not hang forever
			containerCreateR.Close()
			containerStartW.Close()
			logrus.Errorf("did not get container create message from subprocess: %v", err)
		} else {
			pidFile := filepath.Join(bundlePath, "pid")
			pidValue, err := ioutil.ReadFile(pidFile)
			if err != nil {
				return err
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(pidValue)))
			if err != nil {
				return errors.Wrapf(err, "error parsing pid %s as a number", string(pidValue))
			}

			teardown, netstatus, err := b.runConfigureNetwork(pid, isolation, options, configureNetworks, containerName)
			if teardown != nil {
				defer teardown()
			}
			if err != nil {
				return err
			}

			// only add hosts if we manage the hosts file
			if hostsFile != "" {
				var entries etchosts.HostEntries
				if netstatus != nil {
					entries = etchosts.GetNetworkHostEntries(netstatus, spec.Hostname, buildContainerName)
				} else {
					// we have slirp4netns, default to slirp4netns ip since this is not configurable in buildah
					entries = etchosts.HostEntries{{IP: "10.0.2.100", Names: []string{spec.Hostname, buildContainerName}}}
				}
				// make sure to sync this with (b *Builder) generateHosts()
				err = etchosts.Add(hostsFile, entries)
				if err != nil {
					return err
				}
			}

			logrus.Debug("network namespace successfully setup, send start message to child")
			_, err = containerStartW.file.Write([]byte{1})
			if err != nil {
				return err
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return errors.Wrapf(err, "error while running runtime")
	}
	confwg.Wait()
	signal.Stop(interrupted)
	close(interrupted)
	if err == nil {
		return conferr
	}
	if conferr != nil {
		logrus.Debugf("%v", conferr)
	}
	return err
}

// fileCloser is a helper struct to prevent closing the file twice in the code
// users must call (fileCloser).Close() and not fileCloser.File.Close()
type fileCloser struct {
	file   *os.File
	closed bool
}

func (f *fileCloser) Close() {
	if !f.closed {
		if err := f.file.Close(); err != nil {
			logrus.Errorf("failed to close file: %v", err)
		}
		f.closed = true
	}
}

// waitForSync waits for a maximum of 4 minutes to read something from the file
func waitForSync(pipeR *os.File) error {
	if err := pipeR.SetDeadline(time.Now().Add(4 * time.Minute)); err != nil {
		return err
	}
	b := make([]byte, 16)
	_, err := pipeR.Read(b)
	return err
}

func checkAndOverrideIsolationOptions(isolation define.Isolation, options *RunOptions) error {
	switch isolation {
	case IsolationOCIRootless:
		// only change the netns if the caller did not set it
		if ns := options.NamespaceOptions.Find(string(specs.NetworkNamespace)); ns == nil {
			if _, err := exec.LookPath("slirp4netns"); err != nil {
				// if slirp4netns is not installed we have to use the hosts net namespace
				options.NamespaceOptions.AddOrReplace(define.NamespaceOption{Name: string(specs.NetworkNamespace), Host: true})
			}
		}
		fallthrough
	case IsolationOCI:
		pidns := options.NamespaceOptions.Find(string(specs.PIDNamespace))
		userns := options.NamespaceOptions.Find(string(specs.UserNamespace))
		if (pidns != nil && pidns.Host) && (userns != nil && !userns.Host) {
			return errors.Errorf("not allowed to mix host PID namespace with container user namespace")
		}
	}
	return nil
}

// DefaultNamespaceOptions returns the default namespace settings from the
// runtime-tools generator library.
func DefaultNamespaceOptions() (define.NamespaceOptions, error) {
	cfg, err := config.Default()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get container config")
	}
	options := define.NamespaceOptions{
		{Name: string(specs.CgroupNamespace), Host: cfg.CgroupNS() == "host"},
		{Name: string(specs.IPCNamespace), Host: cfg.IPCNS() == "host"},
		{Name: string(specs.MountNamespace), Host: false},
		{Name: string(specs.NetworkNamespace), Host: cfg.NetNS() == "host"},
		{Name: string(specs.PIDNamespace), Host: cfg.PidNS() == "host"},
		{Name: string(specs.UserNamespace), Host: cfg.Containers.UserNS == "host"},
		{Name: string(specs.UTSNamespace), Host: cfg.UTSNS() == "host"},
	}
	return options, nil
}

func contains(volumes []string, v string) bool {
	for _, i := range volumes {
		if i == v {
			return true
		}
	}
	return false
}

type runUsingRuntimeSubprocOptions struct {
	Options          RunOptions
	Spec             *specs.Spec
	RootPath         string
	BundlePath       string
	ConfigureNetwork bool
	MoreCreateArgs   []string
	ContainerName    string
	Isolation        define.Isolation
}

func init() {
	reexec.Register(runUsingRuntimeCommand, runUsingRuntimeMain)
}

// runSetupRunMounts sets up mounts that exist only in this RUN, not in subsequent runs
func (b *Builder) runSetupRunMounts(context *imagetypes.SystemContext, mounts []string, secrets map[string]define.Secret, stageMountPoints map[string]internal.StageMountDetails, sshSources map[string]*sshagent.Source, containerWorkingDir string, contextDir string, uidmap []spec.LinuxIDMapping, gidmap []spec.LinuxIDMapping, rootUID int, rootGID int, processUID int, processGID int) ([]spec.Mount, *runMountArtifacts, error) {
	mountTargets := make([]string, 0, 10)
	tmpFiles := make([]string, 0, len(mounts))
	mountImages := make([]string, 0, 10)
	finalMounts := make([]specs.Mount, 0, len(mounts))
	agents := make([]*sshagent.AgentServer, 0, len(mounts))
	sshCount := 0
	defaultSSHSock := ""
	tokens := []string{}
	lockedTargets := []string{}
	for _, mount := range mounts {
		arr := strings.SplitN(mount, ",", 2)

		kv := strings.Split(arr[0], "=")
		if len(kv) != 2 || kv[0] != "type" {
			return nil, nil, errors.New("invalid mount type")
		}
		if len(arr) == 2 {
			tokens = strings.Split(arr[1], ",")
		}
		// For now, we only support type secret.
		switch kv[1] {
		case "secret":
			mount, envFile, err := getSecretMount(tokens, secrets, b.MountLabel, containerWorkingDir, uidmap, gidmap)
			if err != nil {
				return nil, nil, err
			}
			if mount != nil {
				finalMounts = append(finalMounts, *mount)
				mountTargets = append(mountTargets, mount.Destination)
				if envFile != "" {
					tmpFiles = append(tmpFiles, envFile)
				}
			}
		case "ssh":
			mount, agent, err := b.getSSHMount(tokens, sshCount, sshSources, b.MountLabel, uidmap, gidmap, b.ProcessLabel)
			if err != nil {
				return nil, nil, err
			}
			if mount != nil {
				finalMounts = append(finalMounts, *mount)
				mountTargets = append(mountTargets, mount.Destination)
				agents = append(agents, agent)
				if sshCount == 0 {
					defaultSSHSock = mount.Destination
				}
				// Count is needed as the default destination of the ssh sock inside the container is  /run/buildkit/ssh_agent.{i}
				sshCount++
			}
		case "bind":
			mount, image, err := b.getBindMount(context, tokens, contextDir, rootUID, rootGID, processUID, processGID, stageMountPoints)
			if err != nil {
				return nil, nil, err
			}
			finalMounts = append(finalMounts, *mount)
			mountTargets = append(mountTargets, mount.Destination)
			// only perform cleanup if image was mounted ignore everything else
			if image != "" {
				mountImages = append(mountImages, image)
			}
		case "tmpfs":
			mount, err := b.getTmpfsMount(tokens, rootUID, rootGID, processUID, processGID)
			if err != nil {
				return nil, nil, err
			}
			finalMounts = append(finalMounts, *mount)
			mountTargets = append(mountTargets, mount.Destination)
		case "cache":
			mount, lockedPaths, err := b.getCacheMount(tokens, rootUID, rootGID, processUID, processGID, stageMountPoints)
			if err != nil {
				return nil, nil, err
			}
			finalMounts = append(finalMounts, *mount)
			mountTargets = append(mountTargets, mount.Destination)
			lockedTargets = lockedPaths
		default:
			return nil, nil, errors.Errorf("invalid mount type %q", kv[1])
		}
	}
	artifacts := &runMountArtifacts{
		RunMountTargets: mountTargets,
		TmpFiles:        tmpFiles,
		Agents:          agents,
		MountedImages:   mountImages,
		SSHAuthSock:     defaultSSHSock,
		LockedTargets:   lockedTargets,
	}
	return finalMounts, artifacts, nil
}

func (b *Builder) getBindMount(context *imagetypes.SystemContext, tokens []string, contextDir string, rootUID, rootGID, processUID, processGID int, stageMountPoints map[string]internal.StageMountDetails) (*spec.Mount, string, error) {
	if contextDir == "" {
		return nil, "", errors.New("Context Directory for current run invocation is not configured")
	}
	var optionMounts []specs.Mount
	mount, image, err := internalParse.GetBindMount(context, tokens, contextDir, b.store, b.MountLabel, stageMountPoints)
	if err != nil {
		return nil, image, err
	}
	optionMounts = append(optionMounts, mount)
	volumes, err := b.runSetupVolumeMounts(b.MountLabel, nil, optionMounts, rootUID, rootGID, processUID, processGID)
	if err != nil {
		return nil, image, err
	}
	return &volumes[0], image, nil
}

func (b *Builder) getTmpfsMount(tokens []string, rootUID, rootGID, processUID, processGID int) (*spec.Mount, error) {
	var optionMounts []specs.Mount
	mount, err := internalParse.GetTmpfsMount(tokens)
	if err != nil {
		return nil, err
	}
	optionMounts = append(optionMounts, mount)
	volumes, err := b.runSetupVolumeMounts(b.MountLabel, nil, optionMounts, rootUID, rootGID, processUID, processGID)
	if err != nil {
		return nil, err
	}
	return &volumes[0], nil
}

func (b *Builder) getCacheMount(tokens []string, rootUID, rootGID, processUID, processGID int, stageMountPoints map[string]internal.StageMountDetails) (*spec.Mount, []string, error) {
	var optionMounts []specs.Mount
	mount, lockedTargets, err := internalParse.GetCacheMount(tokens, b.store, b.MountLabel, stageMountPoints)
	if err != nil {
		return nil, lockedTargets, err
	}
	optionMounts = append(optionMounts, mount)
	volumes, err := b.runSetupVolumeMounts(b.MountLabel, nil, optionMounts, rootUID, rootGID, processUID, processGID)
	if err != nil {
		return nil, lockedTargets, err
	}
	return &volumes[0], lockedTargets, nil
}

func getSecretMount(tokens []string, secrets map[string]define.Secret, mountlabel string, containerWorkingDir string, uidmap []spec.LinuxIDMapping, gidmap []spec.LinuxIDMapping) (*spec.Mount, string, error) {
	errInvalidSyntax := errors.New("secret should have syntax id=id[,target=path,required=bool,mode=uint,uid=uint,gid=uint")
	if len(tokens) == 0 {
		return nil, "", errInvalidSyntax
	}
	var err error
	var id, target string
	var required bool
	var uid, gid uint32
	var mode uint32 = 0400
	for _, val := range tokens {
		kv := strings.SplitN(val, "=", 2)
		switch kv[0] {
		case "id":
			id = kv[1]
		case "target", "dst", "destination":
			target = kv[1]
		case "required":
			required, err = strconv.ParseBool(kv[1])
			if err != nil {
				return nil, "", errInvalidSyntax
			}
		case "mode":
			mode64, err := strconv.ParseUint(kv[1], 8, 32)
			if err != nil {
				return nil, "", errInvalidSyntax
			}
			mode = uint32(mode64)
		case "uid":
			uid64, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return nil, "", errInvalidSyntax
			}
			uid = uint32(uid64)
		case "gid":
			gid64, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return nil, "", errInvalidSyntax
			}
			gid = uint32(gid64)
		default:
			return nil, "", errInvalidSyntax
		}
	}

	if id == "" {
		return nil, "", errInvalidSyntax
	}
	// Default location for secretis is /run/secrets/id
	if target == "" {
		target = "/run/secrets/" + id
	}

	secr, ok := secrets[id]
	if !ok {
		if required {
			return nil, "", errors.Errorf("secret required but no secret with id %s found", id)
		}
		return nil, "", nil
	}
	var data []byte
	var envFile string
	var ctrFileOnHost string

	switch secr.SourceType {
	case "env":
		data = []byte(os.Getenv(secr.Source))
		tmpFile, err := ioutil.TempFile("/dev/shm", "buildah*")
		if err != nil {
			return nil, "", err
		}
		envFile = tmpFile.Name()
		ctrFileOnHost = tmpFile.Name()
	case "file":
		data, err = ioutil.ReadFile(secr.Source)
		if err != nil {
			return nil, "", err
		}
		ctrFileOnHost = filepath.Join(containerWorkingDir, "secrets", id)
		_, err = os.Stat(ctrFileOnHost)
		if !os.IsNotExist(err) {
			return nil, "", err
		}
	default:
		return nil, "", errors.New("invalid source secret type")
	}

	// Copy secrets to container working dir (or tmp dir if it's an env), since we need to chmod,
	// chown and relabel it for the container user and we don't want to mess with the original file
	if err := os.MkdirAll(filepath.Dir(ctrFileOnHost), 0755); err != nil {
		return nil, "", err
	}
	if err := ioutil.WriteFile(ctrFileOnHost, data, 0644); err != nil {
		return nil, "", err
	}

	if err := label.Relabel(ctrFileOnHost, mountlabel, false); err != nil {
		return nil, "", err
	}
	hostUID, hostGID, err := util.GetHostIDs(uidmap, gidmap, uid, gid)
	if err != nil {
		return nil, "", err
	}
	if err := os.Lchown(ctrFileOnHost, int(hostUID), int(hostGID)); err != nil {
		return nil, "", err
	}
	if err := os.Chmod(ctrFileOnHost, os.FileMode(mode)); err != nil {
		return nil, "", err
	}
	newMount := specs.Mount{
		Destination: target,
		Type:        "bind",
		Source:      ctrFileOnHost,
		Options:     []string{"bind", "rprivate", "ro"},
	}
	return &newMount, envFile, nil
}

// getSSHMount parses the --mount type=ssh flag in the Containerfile, checks if there's an ssh source provided, and creates and starts an ssh-agent to be forwarded into the container
func (b *Builder) getSSHMount(tokens []string, count int, sshsources map[string]*sshagent.Source, mountlabel string, uidmap []spec.LinuxIDMapping, gidmap []spec.LinuxIDMapping, processLabel string) (*spec.Mount, *sshagent.AgentServer, error) {
	errInvalidSyntax := errors.New("ssh should have syntax id=id[,target=path,required=bool,mode=uint,uid=uint,gid=uint")

	var err error
	var id, target string
	var required bool
	var uid, gid uint32
	var mode uint32 = 400
	for _, val := range tokens {
		kv := strings.SplitN(val, "=", 2)
		if len(kv) < 2 {
			return nil, nil, errInvalidSyntax
		}
		switch kv[0] {
		case "id":
			id = kv[1]
		case "target", "dst", "destination":
			target = kv[1]
		case "required":
			required, err = strconv.ParseBool(kv[1])
			if err != nil {
				return nil, nil, errInvalidSyntax
			}
		case "mode":
			mode64, err := strconv.ParseUint(kv[1], 8, 32)
			if err != nil {
				return nil, nil, errInvalidSyntax
			}
			mode = uint32(mode64)
		case "uid":
			uid64, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return nil, nil, errInvalidSyntax
			}
			uid = uint32(uid64)
		case "gid":
			gid64, err := strconv.ParseUint(kv[1], 10, 32)
			if err != nil {
				return nil, nil, errInvalidSyntax
			}
			gid = uint32(gid64)
		default:
			return nil, nil, errInvalidSyntax
		}
	}

	if id == "" {
		id = "default"
	}
	// Default location for secretis is /run/buildkit/ssh_agent.{i}
	if target == "" {
		target = fmt.Sprintf("/run/buildkit/ssh_agent.%d", count)
	}

	sshsource, ok := sshsources[id]
	if !ok {
		if required {
			return nil, nil, errors.Errorf("ssh required but no ssh with id %s found", id)
		}
		return nil, nil, nil
	}
	// Create new agent from keys or socket
	fwdAgent, err := sshagent.NewAgentServer(sshsource)
	if err != nil {
		return nil, nil, err
	}
	// Start ssh server, and get the host sock we're mounting in the container
	hostSock, err := fwdAgent.Serve(processLabel)
	if err != nil {
		return nil, nil, err
	}

	if err := label.Relabel(filepath.Dir(hostSock), mountlabel, false); err != nil {
		if shutdownErr := fwdAgent.Shutdown(); shutdownErr != nil {
			b.Logger.Errorf("error shutting down agent: %v", shutdownErr)
		}
		return nil, nil, err
	}
	if err := label.Relabel(hostSock, mountlabel, false); err != nil {
		if shutdownErr := fwdAgent.Shutdown(); shutdownErr != nil {
			b.Logger.Errorf("error shutting down agent: %v", shutdownErr)
		}
		return nil, nil, err
	}

	hostUID, hostGID, err := util.GetHostIDs(uidmap, gidmap, uid, gid)
	if err != nil {
		if shutdownErr := fwdAgent.Shutdown(); shutdownErr != nil {
			b.Logger.Errorf("error shutting down agent: %v", shutdownErr)
		}
		return nil, nil, err
	}
	if err := os.Lchown(hostSock, int(hostUID), int(hostGID)); err != nil {
		if shutdownErr := fwdAgent.Shutdown(); shutdownErr != nil {
			b.Logger.Errorf("error shutting down agent: %v", shutdownErr)
		}
		return nil, nil, err
	}
	if err := os.Chmod(hostSock, os.FileMode(mode)); err != nil {
		if shutdownErr := fwdAgent.Shutdown(); shutdownErr != nil {
			b.Logger.Errorf("error shutting down agent: %v", shutdownErr)
		}
		return nil, nil, err
	}
	newMount := specs.Mount{
		Destination: target,
		Type:        "bind",
		Source:      hostSock,
		Options:     []string{"bind", "rprivate", "ro"},
	}
	return &newMount, fwdAgent, nil
}

// cleanupRunMounts cleans up run mounts so they only appear in this run.
func (b *Builder) cleanupRunMounts(context *imagetypes.SystemContext, mountpoint string, artifacts *runMountArtifacts) error {
	for _, agent := range artifacts.Agents {
		err := agent.Shutdown()
		if err != nil {
			return err
		}
	}

	//cleanup any mounted images for this run
	for _, image := range artifacts.MountedImages {
		if image != "" {
			// if flow hits here some image was mounted for this run
			i, err := internalUtil.LookupImage(context, b.store, image)
			if err == nil {
				// silently try to unmount and do nothing
				// if image is being used by something else
				_ = i.Unmount(false)
			}
			if errors.Cause(err) == storagetypes.ErrImageUnknown {
				// Ignore only if ErrImageUnknown
				// Reason: Image is already unmounted do nothing
				continue
			}
			return err
		}
	}

	opts := copier.RemoveOptions{
		All: true,
	}
	for _, path := range artifacts.RunMountTargets {
		err := copier.Remove(mountpoint, path, opts)
		if err != nil {
			return err
		}
	}
	var prevErr error
	for _, path := range artifacts.TmpFiles {
		err := os.Remove(path)
		if !os.IsNotExist(err) {
			if prevErr != nil {
				logrus.Error(prevErr)
			}
			prevErr = err
		}
	}
	// unlock if any locked files from this RUN statement
	for _, path := range artifacts.LockedTargets {
		_, err := os.Stat(path)
		if err != nil {
			// Lockfile not found this might be a problem,
			// since LockedTargets must contain list of all locked files
			// don't break here since we need to unlock other files but
			// log so user can take a look
			logrus.Warnf("Lockfile %q was expected here, stat failed with %v", path, err)
			continue
		}
		lockfile, err := lockfile.GetLockfile(path)
		if err != nil {
			// unable to get lockfile
			// lets log error and continue
			// unlocking other files
			logrus.Warn(err)
			continue
		}
		if lockfile.Locked() {
			lockfile.Unlock()
		} else {
			logrus.Warnf("Lockfile %q was expected to be locked, this is unexpected", path)
			continue
		}
	}
	return prevErr
}

// getNetworkInterface creates the network interface
func getNetworkInterface(store storage.Store, cniConfDir, cniPluginPath string) (nettypes.ContainerNetwork, error) {
	conf, err := config.Default()
	if err != nil {
		return nil, err
	}
	// copy the config to not modify the default by accident
	newconf := *conf
	if len(cniConfDir) > 0 {
		newconf.Network.NetworkConfigDir = cniConfDir
	}
	if len(cniPluginPath) > 0 {
		plugins := strings.Split(cniPluginPath, string(os.PathListSeparator))
		newconf.Network.CNIPluginDirs = plugins
	}

	_, netInt, err := network.NetworkBackend(store, &newconf, false)
	if err != nil {
		return nil, err
	}
	return netInt, nil
}

// setPdeathsig sets a parent-death signal for the process
func setPdeathsig(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
