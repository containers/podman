//go:build linux
// +build linux

package buildah

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/buildah/bind"
	"github.com/containers/buildah/chroot"
	"github.com/containers/buildah/copier"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/internal"
	internalParse "github.com/containers/buildah/internal/parse"
	"github.com/containers/buildah/pkg/overlay"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/util"
	"github.com/containers/common/libnetwork/pasta"
	"github.com/containers/common/libnetwork/resolvconf"
	"github.com/containers/common/libnetwork/slirp4netns"
	nettypes "github.com/containers/common/libnetwork/types"
	netUtil "github.com/containers/common/libnetwork/util"
	"github.com/containers/common/pkg/capabilities"
	"github.com/containers/common/pkg/chown"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/hooks"
	hooksExec "github.com/containers/common/pkg/hooks/exec"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/containers/storage/pkg/stringid"
	"github.com/containers/storage/pkg/unshare"
	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// ContainerDevices is an alias for a slice of github.com/opencontainers/runc/libcontainer/configs.Device structures.
type ContainerDevices define.ContainerDevices

var (
	// We dont want to remove destinations with /etc, /dev, /sys,
	// /proc as rootfs already contains these files and unionfs
	// will create a `whiteout` i.e `.wh` files on removal of
	// overlapping files from these directories.  everything other
	// than these will be cleaned up
	nonCleanablePrefixes = []string{
		"/etc", "/dev", "/sys", "/proc",
	}
)

func setChildProcess() error {
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, uintptr(1), 0, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "prctl(PR_SET_CHILD_SUBREAPER, 1): %v\n", err)
		return err
	}
	return nil
}

// Run runs the specified command in the container's root filesystem.
func (b *Builder) Run(command []string, options RunOptions) error {
	p, err := os.MkdirTemp("", define.Package)
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
		return fmt.Errorf("generating new 'linux' runtime spec: %w", err)
	}
	g := &gp

	isolation := options.Isolation
	if isolation == define.IsolationDefault {
		isolation = b.Isolation
		if isolation == define.IsolationDefault {
			isolation, err = parse.IsolationOption("")
			if err != nil {
				logrus.Debugf("got %v while trying to determine default isolation, guessing OCI", err)
				isolation = IsolationOCI
			} else if isolation == IsolationDefault {
				isolation = IsolationOCI
			}
		}
	}
	if err := checkAndOverrideIsolationOptions(isolation, &options); err != nil {
		return err
	}

	// hardwire the environment to match docker build to avoid subtle and hard-to-debug differences due to containers.conf
	b.configureEnvironment(g, options, []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"})

	if b.CommonBuildOpts == nil {
		return fmt.Errorf("invalid format on container you must recreate the container")
	}

	if err := addCommonOptsToSpec(b.CommonBuildOpts, g); err != nil {
		return err
	}

	workDir := b.WorkDir()
	if options.WorkingDir != "" {
		g.SetProcessCwd(options.WorkingDir)
		workDir = options.WorkingDir
	} else if b.WorkDir() != "" {
		g.SetProcessCwd(b.WorkDir())
	}
	setupSelinux(g, b.ProcessLabel, b.MountLabel)
	mountPoint, err := b.Mount(b.MountLabel)
	if err != nil {
		return fmt.Errorf("mounting container %q: %w", b.ContainerID, err)
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
		newMounts := []specs.Mount{}
		for _, d := range b.Devices {
			// Default permission is read-only.
			perm := "ro"
			// Get permission configured for this device but only process `write`
			// permission in rootless since `mknod` is not supported anyways.
			if strings.Contains(string(d.Rule.Permissions), "w") {
				perm = "rw"
			}
			devMnt := specs.Mount{
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
			sDev := specs.LinuxDevice{
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

	configureNetwork, networkString, err := b.configureNamespaces(g, &options)
	if err != nil {
		return err
	}

	homeDir, err := b.configureUIDGID(g, mountPoint, options)
	if err != nil {
		return err
	}

	g.SetProcessNoNewPrivileges(b.CommonBuildOpts.NoNewPrivileges)

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
	volumes := b.Volumes()

	// Figure out who owns files that will appear to be owned by UID/GID 0 in the container.
	rootUID, rootGID, err := util.GetHostRootIDs(spec)
	if err != nil {
		return err
	}
	rootIDPair := &idtools.IDPair{UID: int(rootUID), GID: int(rootGID)}

	hostFile := ""
	if !options.NoHosts && !contains(volumes, config.DefaultHostsFile) && options.ConfigureNetwork != define.NetworkDisabled {
		hostFile, err = b.generateHosts(path, rootIDPair, mountPoint, spec)
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

	if !contains(volumes, resolvconf.DefaultResolvConf) && options.ConfigureNetwork != define.NetworkDisabled && !(len(b.CommonBuildOpts.DNSServers) == 1 && strings.ToLower(b.CommonBuildOpts.DNSServers[0]) == "none") {
		resolvFile, err := b.addResolvConf(path, rootIDPair, b.CommonBuildOpts.DNSServers, b.CommonBuildOpts.DNSSearch, b.CommonBuildOpts.DNSOptions, spec.Linux.Namespaces)
		if err != nil {
			return err
		}
		bindFiles[resolvconf.DefaultResolvConf] = resolvFile
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

	// Setup OCI hooks
	_, err = b.setupOCIHooks(spec, (len(options.Mounts) > 0 || len(volumes) > 0))
	if err != nil {
		return fmt.Errorf("unable to setup OCI hooks: %w", err)
	}

	runMountInfo := runMountInfo{
		WorkDir:          workDir,
		ContextDir:       options.ContextDir,
		Secrets:          options.Secrets,
		SSHSources:       options.SSHSources,
		StageMountPoints: options.StageMountPoints,
		SystemContext:    options.SystemContext,
	}

	runArtifacts, err := b.setupMounts(mountPoint, spec, path, options.Mounts, bindFiles, volumes, b.CommonBuildOpts.Volumes, options.RunMounts, runMountInfo)
	if err != nil {
		return fmt.Errorf("resolving mountpoints for container %q: %w", b.ContainerID, err)
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
		err = b.runUsingRuntimeSubproc(isolation, options, configureNetwork, networkString, moreCreateArgs, spec,
			mountPoint, path, define.Package+"-"+filepath.Base(path), b.Container, hostFile)
	case IsolationChroot:
		err = chroot.RunUsingChroot(spec, path, homeDir, options.Stdin, options.Stdout, options.Stderr)
	case IsolationOCIRootless:
		moreCreateArgs := []string{"--no-new-keyring"}
		if options.NoPivot {
			moreCreateArgs = append(moreCreateArgs, "--no-pivot")
		}
		err = b.runUsingRuntimeSubproc(isolation, options, configureNetwork, networkString, moreCreateArgs, spec,
			mountPoint, path, define.Package+"-"+filepath.Base(path), b.Container, hostFile)
	default:
		err = errors.New("don't know how to run this command")
	}
	return err
}

func (b *Builder) setupOCIHooks(config *specs.Spec, hasVolumes bool) (map[string][]specs.Hook, error) {
	allHooks := make(map[string][]specs.Hook)
	if len(b.CommonBuildOpts.OCIHooksDir) == 0 {
		if unshare.IsRootless() {
			return nil, nil
		}
		for _, hDir := range []string{hooks.DefaultDir, hooks.OverrideDir} {
			manager, err := hooks.New(context.Background(), []string{hDir}, []string{})
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, err
			}
			ociHooks, err := manager.Hooks(config, b.ImageAnnotations, hasVolumes)
			if err != nil {
				return nil, err
			}
			if len(ociHooks) > 0 || config.Hooks != nil {
				logrus.Warnf("Implicit hook directories are deprecated; set --hooks-dir=%q explicitly to continue to load ociHooks from this directory", hDir)
			}
			for i, hook := range ociHooks {
				allHooks[i] = hook
			}
		}
	} else {
		manager, err := hooks.New(context.Background(), b.CommonBuildOpts.OCIHooksDir, []string{})
		if err != nil {
			return nil, err
		}

		allHooks, err = manager.Hooks(config, b.ImageAnnotations, hasVolumes)
		if err != nil {
			return nil, err
		}
	}

	hookErr, err := hooksExec.RuntimeConfigFilter(context.Background(), allHooks["precreate"], config, hooksExec.DefaultPostKillTimeout) //nolint:staticcheck
	if err != nil {
		logrus.Warnf("Container: precreate hook: %v", err)
		if hookErr != nil && hookErr != err {
			logrus.Debugf("container: precreate hook (hook error): %v", hookErr)
		}
		return nil, err
	}
	return allHooks, nil
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
		return fmt.Errorf("failed to get container config: %w", err)
	}
	// Other process resource limits
	if err := addRlimits(commonOpts.Ulimit, g, defaultContainerConfig.Containers.DefaultUlimits); err != nil {
		return err
	}

	logrus.Debugf("Resources: %#v", commonOpts)
	return nil
}

func setupSlirp4netnsNetwork(config *config.Config, netns, cid string, options []string) (func(), map[string]nettypes.StatusBlock, error) {
	// we need the TmpDir for the slirp4netns code
	if err := os.MkdirAll(config.Engine.TmpDir, 0o751); err != nil {
		return nil, nil, fmt.Errorf("failed to create tempdir: %w", err)
	}
	res, err := slirp4netns.Setup(&slirp4netns.SetupOptions{
		Config:       config,
		ContainerID:  cid,
		Netns:        netns,
		ExtraOptions: options,
		Pdeathsig:    syscall.SIGKILL,
	})
	if err != nil {
		return nil, nil, err
	}

	ip, err := slirp4netns.GetIP(res.Subnet)
	if err != nil {
		return nil, nil, fmt.Errorf("get slirp4netns ip: %w", err)
	}

	// create fake status to make sure we get the correct ip in hosts
	subnet := nettypes.IPNet{IPNet: net.IPNet{
		IP:   *ip,
		Mask: res.Subnet.Mask,
	}}
	netStatus := map[string]nettypes.StatusBlock{
		slirp4netns.BinaryName: nettypes.StatusBlock{
			Interfaces: map[string]nettypes.NetInterface{
				"tap0": {
					Subnets: []nettypes.NetAddress{{IPNet: subnet}},
				},
			},
		},
	}

	return func() {
		syscall.Kill(res.Pid, syscall.SIGKILL) // nolint:errcheck
		var status syscall.WaitStatus
		syscall.Wait4(res.Pid, &status, 0, nil) // nolint:errcheck
	}, netStatus, nil
}

func setupPasta(config *config.Config, netns string, options []string) (func(), map[string]nettypes.StatusBlock, error) {
	err := pasta.Setup(&pasta.SetupOptions{
		Config:       config,
		Netns:        netns,
		ExtraOptions: options,
	})
	if err != nil {
		return nil, nil, err
	}

	var ip string
	err = ns.WithNetNSPath(netns, func(_ ns.NetNS) error {
		// get the first ip in the netns and use this as our ip for /etc/hosts
		ip = netUtil.GetLocalIP()
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// create fake status to make sure we get the correct ip in hosts
	subnet := nettypes.IPNet{IPNet: net.IPNet{
		IP:   net.ParseIP(ip),
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}}
	netStatus := map[string]nettypes.StatusBlock{
		slirp4netns.BinaryName: nettypes.StatusBlock{
			Interfaces: map[string]nettypes.NetInterface{
				"tap0": {
					Subnets: []nettypes.NetAddress{{IPNet: subnet}},
				},
			},
		},
	}

	return nil, netStatus, nil
}

func (b *Builder) runConfigureNetwork(pid int, isolation define.Isolation, options RunOptions, network, containerName string) (teardown func(), netStatus map[string]nettypes.StatusBlock, err error) {
	netns := fmt.Sprintf("/proc/%d/ns/net", pid)
	var configureNetworks []string
	defConfig, err := config.Default()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get container config: %w", err)
	}

	name, networkOpts, hasOpts := strings.Cut(network, ":")
	var netOpts []string
	if hasOpts {
		netOpts = strings.Split(networkOpts, ",")
	}
	if isolation == IsolationOCIRootless && name == "" {
		switch defConfig.Network.DefaultRootlessNetworkCmd {
		case slirp4netns.BinaryName, "":
			name = slirp4netns.BinaryName
		case pasta.BinaryName:
			name = pasta.BinaryName
		default:
			return nil, nil, fmt.Errorf("invalid default_rootless_network_cmd option %q",
				defConfig.Network.DefaultRootlessNetworkCmd)
		}
	}

	switch {
	case name == slirp4netns.BinaryName:
		return setupSlirp4netnsNetwork(defConfig, netns, containerName, netOpts)
	case name == pasta.BinaryName:
		return setupPasta(defConfig, netns, netOpts)

	// Basically default case except we make sure to not split an empty
	// name as this would return a slice with one empty string which is
	// not a valid network name.
	case len(network) > 0:
		// old syntax allow comma separated network names
		configureNetworks = strings.Split(network, ",")
	}

	if isolation == IsolationOCIRootless {
		return nil, nil, errors.New("cannot use networks as rootless")
	}

	if len(configureNetworks) == 0 {
		configureNetworks = []string{b.NetworkInterface.DefaultNetworkName()}
	}

	// Make sure we can access the container's network namespace,
	// even after it exits, to successfully tear down the
	// interfaces.  Ensure this by opening a handle to the network
	// namespace, and using our copy to both configure and
	// deconfigure it.
	netFD, err := unix.Open(netns, unix.O_RDONLY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("opening network namespace: %w", err)
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

// Create pipes to use for relaying stdio.
func runMakeStdioPipe(uid, gid int) ([][]int, error) {
	stdioPipe := make([][]int, 3)
	for i := range stdioPipe {
		stdioPipe[i] = make([]int, 2)
		if err := unix.Pipe(stdioPipe[i]); err != nil {
			return nil, fmt.Errorf("creating pipe for container FD %d: %w", i, err)
		}
	}
	if err := unix.Fchown(stdioPipe[unix.Stdin][0], uid, gid); err != nil {
		return nil, fmt.Errorf("setting owner of stdin pipe descriptor: %w", err)
	}
	if err := unix.Fchown(stdioPipe[unix.Stdout][1], uid, gid); err != nil {
		return nil, fmt.Errorf("setting owner of stdout pipe descriptor: %w", err)
	}
	if err := unix.Fchown(stdioPipe[unix.Stderr][1], uid, gid); err != nil {
		return nil, fmt.Errorf("setting owner of stderr pipe descriptor: %w", err)
	}
	return stdioPipe, nil
}

func setupNamespaces(logger *logrus.Logger, g *generate.Generator, namespaceOptions define.NamespaceOptions, idmapOptions define.IDMappingOptions, policy define.NetworkConfigurationPolicy) (configureNetwork bool, networkString string, configureUTS bool, err error) {
	defaultContainerConfig, err := config.Default()
	if err != nil {
		return false, "", false, fmt.Errorf("failed to get container config: %w", err)
	}

	addSysctl := func(prefixes []string) error {
		for _, sysctl := range defaultContainerConfig.Sysctls() {
			splitn := strings.SplitN(sysctl, "=", 2)
			if len(splitn) > 2 {
				return fmt.Errorf("sysctl %q defined in containers.conf must be formatted name=value", sysctl)
			}
			for _, prefix := range prefixes {
				if strings.HasPrefix(splitn[0], prefix) {
					g.AddLinuxSysctl(splitn[0], splitn[1])
				}
			}
		}
		return nil
	}

	// Set namespace options in the container configuration.
	configureUserns := false
	specifiedNetwork := false
	for _, namespaceOption := range namespaceOptions {
		switch namespaceOption.Name {
		case string(specs.IPCNamespace):
			if !namespaceOption.Host {
				if err := addSysctl([]string{"fs.mqueue"}); err != nil {
					return false, "", false, err
				}
			}
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
					networkString = namespaceOption.Path
					namespaceOption.Path = ""
				}
				configureNetwork = (policy != define.NetworkDisabled)
			}
		case string(specs.UTSNamespace):
			configureUTS = false
			if !namespaceOption.Host {
				if namespaceOption.Path == "" {
					configureUTS = true
				}
				if err := addSysctl([]string{"kernel.hostname", "kernel.domainame"}); err != nil {
					return false, "", false, err
				}
			}
		}
		if namespaceOption.Host {
			if err := g.RemoveLinuxNamespace(namespaceOption.Name); err != nil {
				return false, "", false, fmt.Errorf("removing %q namespace for run: %w", namespaceOption.Name, err)
			}
		} else if err := g.AddOrReplaceLinuxNamespace(namespaceOption.Name, namespaceOption.Path); err != nil {
			if namespaceOption.Path == "" {
				return false, "", false, fmt.Errorf("adding new %q namespace for run: %w", namespaceOption.Name, err)
			}
			return false, "", false, fmt.Errorf("adding %q namespace %q for run: %w", namespaceOption.Name, namespaceOption.Path, err)
		}
	}

	// If we've got mappings, we're going to have to create a user namespace.
	if len(idmapOptions.UIDMap) > 0 || len(idmapOptions.GIDMap) > 0 || configureUserns {
		if err := g.AddOrReplaceLinuxNamespace(string(specs.UserNamespace), ""); err != nil {
			return false, "", false, fmt.Errorf("adding new %q namespace for run: %w", string(specs.UserNamespace), err)
		}
		hostUidmap, hostGidmap, err := unshare.GetHostIDMappings("")
		if err != nil {
			return false, "", false, err
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
				return false, "", false, fmt.Errorf("adding new %q namespace for run: %w", string(specs.NetworkNamespace), err)
			}
			configureNetwork = (policy != define.NetworkDisabled)
		}
	} else {
		if err := g.RemoveLinuxNamespace(string(specs.UserNamespace)); err != nil {
			return false, "", false, fmt.Errorf("removing %q namespace for run: %w", string(specs.UserNamespace), err)
		}
		if !specifiedNetwork {
			if err := g.RemoveLinuxNamespace(string(specs.NetworkNamespace)); err != nil {
				return false, "", false, fmt.Errorf("removing %q namespace for run: %w", string(specs.NetworkNamespace), err)
			}
		}
	}
	if configureNetwork {
		if err := addSysctl([]string{"net"}); err != nil {
			return false, "", false, err
		}
		for name, val := range define.DefaultNetworkSysctl {
			// Check that the sysctl we are adding is actually supported
			// by the kernel
			p := filepath.Join("/proc/sys", strings.Replace(name, ".", "/", -1))
			_, err := os.Stat(p)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return false, "", false, err
			}
			if err == nil {
				g.AddLinuxSysctl(name, val)
			} else {
				logger.Warnf("ignoring sysctl %s since %s doesn't exist", name, p)
			}
		}
	}
	return configureNetwork, networkString, configureUTS, nil
}

func (b *Builder) configureNamespaces(g *generate.Generator, options *RunOptions) (bool, string, error) {
	defaultNamespaceOptions, err := DefaultNamespaceOptions()
	if err != nil {
		return false, "", err
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
	if networkPolicy == NetworkDisabled {
		namespaceOptions.AddOrReplace(define.NamespaceOptions{{Name: string(specs.NetworkNamespace), Host: false}}...)
	}
	configureNetwork, networkString, configureUTS, err := setupNamespaces(options.Logger, g, namespaceOptions, b.IDMappingOptions, networkPolicy)
	if err != nil {
		return false, "", err
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

	return configureNetwork, networkString, nil
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
			return fmt.Errorf("ulimit option %q requires name=SOFT:HARD, failed to be parsed: %w", u, err)
		}

		g.AddProcessRlimits("RLIMIT_"+strings.ToUpper(ul.Name), uint64(ul.Hard), uint64(ul.Soft))
	}
	return nil
}

func (b *Builder) runSetupVolumeMounts(mountLabel string, volumeMounts []string, optionMounts []specs.Mount, idMaps IDMaps) (mounts []specs.Mount, Err error) {
	// Make sure the overlay directory is clean before running
	containerDir, err := b.store.ContainerDirectory(b.ContainerID)
	if err != nil {
		return nil, fmt.Errorf("looking up container directory for %s: %w", b.ContainerID, err)
	}
	if err := overlay.CleanupContent(containerDir); err != nil {
		return nil, fmt.Errorf("cleaning up overlay content for %s: %w", b.ContainerID, err)
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
			if err := chown.ChangeHostPathOwnership(host, true, idMaps.processUID, idMaps.processGID); err != nil {
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

			contentDir, err := overlay.TempDir(containerDir, idMaps.rootUID, idMaps.rootGID)
			if err != nil {
				return specs.Mount{}, fmt.Errorf("failed to create TempDir in the %s directory: %w", containerDir, err)
			}

			overlayOpts := overlay.Options{
				RootUID:                idMaps.rootUID,
				RootGID:                idMaps.rootGID,
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
				if err := chown.ChangeHostPathOwnership(contentDir, true, idMaps.processUID, idMaps.processGID); err != nil {
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
			return fmt.Errorf("adding %q to the bounding capability set: %w", cap, err)
		}
		if err := g.AddProcessCapabilityEffective(cap); err != nil {
			return fmt.Errorf("adding %q to the effective capability set: %w", cap, err)
		}
		if err := g.AddProcessCapabilityPermitted(cap); err != nil {
			return fmt.Errorf("adding %q to the permitted capability set: %w", cap, err)
		}
		if err := g.AddProcessCapabilityAmbient(cap); err != nil {
			return fmt.Errorf("adding %q to the ambient capability set: %w", cap, err)
		}
	}
	return nil
}

func setupCapDrop(g *generate.Generator, caps ...string) error {
	for _, cap := range caps {
		if err := g.DropProcessCapabilityBounding(cap); err != nil {
			return fmt.Errorf("removing %q from the bounding capability set: %w", cap, err)
		}
		if err := g.DropProcessCapabilityEffective(cap); err != nil {
			return fmt.Errorf("removing %q from the effective capability set: %w", cap, err)
		}
		if err := g.DropProcessCapabilityPermitted(cap); err != nil {
			return fmt.Errorf("removing %q from the permitted capability set: %w", cap, err)
		}
		if err := g.DropProcessCapabilityAmbient(cap); err != nil {
			return fmt.Errorf("removing %q from the ambient capability set: %w", cap, err)
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

func addOrReplaceMount(mounts []specs.Mount, mount specs.Mount) []specs.Mount {
	for i := range mounts {
		if mounts[i].Destination == mount.Destination {
			mounts[i] = mount
			return mounts
		}
	}
	return append(mounts, mount)
}

// setupSpecialMountSpecChanges creates special mounts for depending on the namespaces
// logic taken from podman and adapted for buildah
// https://github.com/containers/podman/blob/4ba71f955a944790edda6e007e6d074009d437a7/pkg/specgen/generate/oci.go#L178
func setupSpecialMountSpecChanges(spec *specs.Spec, shmSize string) ([]specs.Mount, error) {
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
	// mount sys when root and no userns or when a new netns is created
	canMountSys := (!isRootless && !isNewUserns) || isNetns
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

func checkIdsGreaterThan5(ids []specs.LinuxIDMapping) bool {
	for _, r := range ids {
		if r.ContainerID <= 5 && 5 < r.ContainerID+r.Size {
			return true
		}
	}
	return false
}

// If this function succeeds and returns a non-nil *lockfile.LockFile, the caller must unlock it (when??).
func (b *Builder) getCacheMount(tokens []string, stageMountPoints map[string]internal.StageMountDetails, idMaps IDMaps, workDir string) (*specs.Mount, *lockfile.LockFile, error) {
	var optionMounts []specs.Mount
	mount, targetLock, err := internalParse.GetCacheMount(tokens, b.store, b.MountLabel, stageMountPoints, workDir)
	if err != nil {
		return nil, nil, err
	}
	succeeded := false
	defer func() {
		if !succeeded && targetLock != nil {
			targetLock.Unlock()
		}
	}()
	optionMounts = append(optionMounts, mount)
	volumes, err := b.runSetupVolumeMounts(b.MountLabel, nil, optionMounts, idMaps)
	if err != nil {
		return nil, nil, err
	}
	succeeded = true
	return &volumes[0], targetLock, nil
}
