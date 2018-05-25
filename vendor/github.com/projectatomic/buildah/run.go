package buildah

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/mount"
	"github.com/containers/storage/pkg/reexec"
	"github.com/docker/docker/profiles/seccomp"
	units "github.com/docker/go-units"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah/util"
	"github.com/projectatomic/libpod/pkg/secrets"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sys/unix"
)

const (
	// DefaultWorkingDir is used if none was specified.
	DefaultWorkingDir = "/"
	// DefaultRuntime is the default command to use to run the container.
	DefaultRuntime = "runc"
	// runUsingRuntimeCommand is a command we use as a key for reexec
	runUsingRuntimeCommand = Package + "-runtime"
)

// TerminalPolicy takes the value DefaultTerminal, WithoutTerminal, or WithTerminal.
type TerminalPolicy int

const (
	// DefaultTerminal indicates that this Run invocation should be
	// connected to a pseudoterminal if we're connected to a terminal.
	DefaultTerminal TerminalPolicy = iota
	// WithoutTerminal indicates that this Run invocation should NOT be
	// connected to a pseudoterminal.
	WithoutTerminal
	// WithTerminal indicates that this Run invocation should be connected
	// to a pseudoterminal.
	WithTerminal
)

// String converts a TerminalPoliicy into a string.
func (t TerminalPolicy) String() string {
	switch t {
	case DefaultTerminal:
		return "DefaultTerminal"
	case WithoutTerminal:
		return "WithoutTerminal"
	case WithTerminal:
		return "WithTerminal"
	}
	return fmt.Sprintf("unrecognized terminal setting %d", t)
}

// NamespaceOption controls how we set up a namespace when launching processes.
type NamespaceOption struct {
	// Name specifies the type of namespace, typically matching one of the
	// ...Namespace constants defined in
	// github.com/opencontainers/runtime-spec/specs-go.
	Name string
	// Host is used to force our processes to use the host's namespace of
	// this type.
	Host bool
	// Path is the path of the namespace to attach our process to, if Host
	// is not set.  If Host is not set and Path is also empty, a new
	// namespace will be created for the process that we're starting.
	// If Name is specs.NetworkNamespace, if Path doesn't look like an
	// absolute path, it is treated as a comma-separated list of CNI
	// configuration names which will be selected from among all of the CNI
	// network configurations which we find.
	Path string
}

// NamespaceOptions provides some helper methods for a slice of NamespaceOption
// structs.
type NamespaceOptions []NamespaceOption

// IDMappingOptions controls how we set up UID/GID mapping when we set up a
// user namespace.
type IDMappingOptions struct {
	HostUIDMapping bool
	HostGIDMapping bool
	UIDMap         []specs.LinuxIDMapping
	GIDMap         []specs.LinuxIDMapping
}

// RunOptions can be used to alter how a command is run in the container.
type RunOptions struct {
	// Hostname is the hostname we set for the running container.
	Hostname string
	// Runtime is the name of the command to run.  It should accept the same arguments
	// that runc does, and produce similar output.
	Runtime string
	// Args adds global arguments for the runtime.
	Args []string
	// Mounts are additional mount points which we want to provide.
	Mounts []specs.Mount
	// Env is additional environment variables to set.
	Env []string
	// User is the user as whom to run the command.
	User string
	// WorkingDir is an override for the working directory.
	WorkingDir string
	// Shell is default shell to run in a container.
	Shell string
	// Cmd is an override for the configured default command.
	Cmd []string
	// Entrypoint is an override for the configured entry point.
	Entrypoint []string
	// NamespaceOptions controls how we set up the namespaces for the process.
	NamespaceOptions NamespaceOptions
	// ConfigureNetwork controls whether or not network interfaces and
	// routing are configured for a new network namespace (i.e., when not
	// joining another's namespace and not just using the host's
	// namespace), effectively deciding whether or not the process has a
	// usable network.
	ConfigureNetwork NetworkConfigurationPolicy
	// CNIPluginPath is the location of CNI plugin helpers, if they should be
	// run from a location other than the default location.
	CNIPluginPath string
	// CNIConfigDir is the location of CNI configuration files, if the files in
	// the default configuration directory shouldn't be used.
	CNIConfigDir string
	// Terminal provides a way to specify whether or not the command should
	// be run with a pseudoterminal.  By default (DefaultTerminal), a
	// terminal is used if os.Stdout is connected to a terminal, but that
	// decision can be overridden by specifying either WithTerminal or
	// WithoutTerminal.
	Terminal TerminalPolicy
	// Quiet tells the run to turn off output to stdout.
	Quiet bool
}

// DefaultNamespaceOptions returns the default namespace settings from the
// runtime-tools generator library.
func DefaultNamespaceOptions() NamespaceOptions {
	options := NamespaceOptions{
		{Name: string(specs.CgroupNamespace), Host: true},
		{Name: string(specs.IPCNamespace), Host: true},
		{Name: string(specs.MountNamespace), Host: true},
		{Name: string(specs.NetworkNamespace), Host: true},
		{Name: string(specs.PIDNamespace), Host: true},
		{Name: string(specs.UserNamespace), Host: true},
		{Name: string(specs.UTSNamespace), Host: true},
	}
	g := generate.New()
	spec := g.Spec()
	if spec.Linux != nil {
		for _, ns := range spec.Linux.Namespaces {
			options.AddOrReplace(NamespaceOption{
				Name: string(ns.Type),
				Path: ns.Path,
			})
		}
	}
	return options
}

// Find the configuration for the namespace of the given type.  If there are
// duplicates, find the _last_ one of the type, since we assume it was appended
// more recently.
func (n *NamespaceOptions) Find(namespace string) *NamespaceOption {
	for i := range *n {
		j := len(*n) - 1 - i
		if (*n)[j].Name == namespace {
			return &((*n)[j])
		}
	}
	return nil
}

// AddOrReplace either adds or replaces the configuration for a given namespace.
func (n *NamespaceOptions) AddOrReplace(options ...NamespaceOption) {
nextOption:
	for _, option := range options {
		for i := range *n {
			j := len(*n) - 1 - i
			if (*n)[j].Name == option.Name {
				(*n)[j] = option
				continue nextOption
			}
		}
		*n = append(*n, option)
	}
}

func addRlimits(ulimit []string, g *generate.Generator) error {
	var (
		ul  *units.Ulimit
		err error
	)

	for _, u := range ulimit {
		if ul, err = units.ParseUlimit(u); err != nil {
			return errors.Wrapf(err, "ulimit option %q requires name=SOFT:HARD, failed to be parsed", u)
		}

		g.AddProcessRlimits("RLIMIT_"+strings.ToUpper(ul.Name), uint64(ul.Hard), uint64(ul.Soft))
	}
	return nil
}

func addHosts(hosts []string, w io.Writer) error {
	buf := bufio.NewWriter(w)
	for _, host := range hosts {
		fmt.Fprintln(buf, host)
	}
	return buf.Flush()
}

func addHostsToFile(hosts []string, filename string) error {
	if len(hosts) == 0 {
		return nil
	}
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer file.Close()
	return addHosts(hosts, file)
}

func addCommonOptsToSpec(commonOpts *CommonBuildOptions, g *generate.Generator) error {
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

	// Other process resource limits
	if err := addRlimits(commonOpts.Ulimit, g); err != nil {
		return err
	}

	logrus.Debugf("Resources: %#v", commonOpts)
	return nil
}

func (b *Builder) setupMounts(mountPoint string, spec *specs.Spec, optionMounts []specs.Mount, bindFiles map[string]string, builtinVolumes, volumeMounts []string, shmSize string, namespaceOptions NamespaceOptions) error {
	// Start building a new list of mounts.
	mounts := make([]specs.Mount, len(optionMounts))
	copy(mounts, optionMounts)
	haveMount := func(destination string) bool {
		for _, mount := range mounts {
			if mount.Destination == destination {
				// Already have something to mount there.
				return true
			}
		}
		return false
	}

	// Copy mounts from the generated list.
	mountCgroups := true
	specMounts := []specs.Mount{}
	for _, specMount := range spec.Mounts {
		// Override some of the mounts from the generated list if we're doing different things with namespaces.
		if specMount.Destination == "/dev/shm" {
			specMount.Options = []string{"nosuid", "noexec", "nodev", "mode=1777", "size=" + shmSize}
			user := namespaceOptions.Find(string(specs.UserNamespace))
			ipc := namespaceOptions.Find(string(specs.IPCNamespace))
			if (ipc == nil || ipc.Host) && (user != nil && !user.Host) {
				if _, err := os.Stat("/dev/shm"); err != nil && os.IsNotExist(err) {
					continue
				}
				specMount = specs.Mount{
					Source:      "/dev/shm",
					Type:        "bind",
					Destination: "/dev/shm",
					Options:     []string{"nobuildahbind", "rbind", "nosuid", "noexec", "nodev"},
				}
			}
		}
		if specMount.Destination == "/dev/mqueue" {
			user := namespaceOptions.Find(string(specs.UserNamespace))
			ipc := namespaceOptions.Find(string(specs.IPCNamespace))
			if (ipc == nil || ipc.Host) && (user != nil && !user.Host) {
				if _, err := os.Stat("/dev/mqueue"); err != nil && os.IsNotExist(err) {
					continue
				}
				specMount = specs.Mount{
					Source:      "/dev/mqueue",
					Type:        "bind",
					Destination: "/dev/mqueue",
					Options:     []string{"nobuildahbind", "rbind", "nosuid", "noexec", "nodev"},
				}
			}
		}
		if specMount.Destination == "/sys" {
			user := namespaceOptions.Find(string(specs.UserNamespace))
			net := namespaceOptions.Find(string(specs.NetworkNamespace))
			if (net == nil || net.Host) && (user != nil && !user.Host) {
				mountCgroups = false
				if _, err := os.Stat("/sys"); err != nil && os.IsNotExist(err) {
					continue
				}
				specMount = specs.Mount{
					Source:      "/sys",
					Type:        "bind",
					Destination: "/sys",
					Options:     []string{"nobuildahbind", "rbind", "nosuid", "noexec", "nodev", "ro"},
				}
			}
		}
		specMounts = append(specMounts, specMount)
	}

	// Add a mount for the cgroups filesystem, unless we're already
	// recursively bind mounting all of /sys, in which case we shouldn't
	// bother with it.
	sysfsMount := []specs.Mount{}
	if mountCgroups {
		sysfsMount = []specs.Mount{{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"nobuildahbind", "nosuid", "noexec", "nodev", "relatime", "ro"},
		}}
	}

	// Get the list of files we need to bind into the container.
	bindFileMounts, err := runSetupBoundFiles(bindFiles)
	if err != nil {
		return err
	}

	// After this point we need to know the per-container persistent storage directory.
	cdir, err := b.store.ContainerDirectory(b.ContainerID)
	if err != nil {
		return errors.Wrapf(err, "error determining work directory for container %q", b.ContainerID)
	}

	// Figure out which UID and GID to tell the secrets package to use
	// for files that it creates.
	rootUID, rootGID, err := getHostRootIDs(spec)
	if err != nil {
		return err
	}

	// Get the list of secrets mounts.
	secretMounts := secrets.SecretMountsWithUIDGID(b.MountLabel, cdir, b.DefaultMountsFilePath, cdir, int(rootUID), int(rootGID))

	// Add temporary copies of the contents of volume locations at the
	// volume locations, unless we already have something there.
	copyWithTar := b.copyWithTar(nil)
	builtins, err := runSetupBuiltinVolumes(b.MountLabel, mountPoint, cdir, copyWithTar, builtinVolumes)
	if err != nil {
		return err
	}

	// Get the list of explicitly-specified volume mounts.
	volumes, err := runSetupVolumeMounts(spec.Linux.MountLabel, volumeMounts)
	if err != nil {
		return err
	}

	// Add them all, in the preferred order, except where they conflict with something that was previously added.
	for _, mount := range append(append(append(append(append(volumes, builtins...), secretMounts...), bindFileMounts...), specMounts...), sysfsMount...) {
		if haveMount(mount.Destination) {
			// Already mounting something there, no need to bother with this one.
			continue
		}
		// Add the mount.
		mounts = append(mounts, mount)
	}

	// Set the list in the spec.
	spec.Mounts = mounts
	return nil
}

func runSetupBoundFiles(bindFiles map[string]string) (mounts []specs.Mount, err error) {
	for dest, src := range bindFiles {
		mounts = append(mounts, specs.Mount{
			Source:      src,
			Destination: dest,
			Type:        "bind",
			Options:     []string{"rbind"},
		})
	}
	return mounts, nil
}

func runSetupBuiltinVolumes(mountLabel, mountPoint, containerDir string, copyWithTar func(srcPath, dstPath string) error, builtinVolumes []string) ([]specs.Mount, error) {
	var mounts []specs.Mount
	// Add temporary copies of the contents of volume locations at the
	// volume locations, unless we already have something there.
	for _, volume := range builtinVolumes {
		subdir := digest.Canonical.FromString(volume).Hex()
		volumePath := filepath.Join(containerDir, "buildah-volumes", subdir)
		// If we need to, initialize the volume path's initial contents.
		if _, err := os.Stat(volumePath); os.IsNotExist(err) {
			if err = os.MkdirAll(volumePath, 0755); err != nil {
				return nil, errors.Wrapf(err, "error creating directory %q for volume %q", volumePath, volume)
			}
			if err = label.Relabel(volumePath, mountLabel, false); err != nil {
				return nil, errors.Wrapf(err, "error relabeling directory %q for volume %q", volumePath, volume)
			}
			srcPath := filepath.Join(mountPoint, volume)
			if err = copyWithTar(srcPath, volumePath); err != nil && !os.IsNotExist(err) {
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

func runSetupVolumeMounts(mountLabel string, volumeMounts []string) ([]specs.Mount, error) {
	var mounts []specs.Mount

	// Bind mount volumes given by the user at execution
	for _, i := range volumeMounts {
		var options []string
		spliti := strings.Split(i, ":")
		if len(spliti) > 2 {
			options = strings.Split(spliti[2], ",")
		}
		options = append(options, "rbind")
		var foundrw, foundro, foundz, foundZ bool
		var rootProp string
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
			case "private", "rprivate", "slave", "rslave", "shared", "rshared":
				rootProp = opt
			}
		}
		if !foundrw && !foundro {
			options = append(options, "rw")
		}
		if foundz {
			if err := label.Relabel(spliti[0], mountLabel, true); err != nil {
				return nil, errors.Wrapf(err, "relabeling %q failed", spliti[0])
			}
		}
		if foundZ {
			if err := label.Relabel(spliti[0], mountLabel, false); err != nil {
				return nil, errors.Wrapf(err, "relabeling %q failed", spliti[0])
			}
		}
		if rootProp == "" {
			options = append(options, "private")
		}

		mounts = append(mounts, specs.Mount{
			Destination: spliti[1],
			Type:        "bind",
			Source:      spliti[0],
			Options:     options,
		})
	}
	return mounts, nil
}

// addNetworkConfig copies files from host and sets them up to bind mount into container
func (b *Builder) addNetworkConfig(rdir, hostPath string) (string, error) {
	copyFileWithTar := b.copyFileWithTar(nil)

	cfile := filepath.Join(rdir, filepath.Base(hostPath))

	if err := copyFileWithTar(hostPath, cfile); err != nil {
		return "", errors.Wrapf(err, "error copying %q for container %q", cfile, b.ContainerID)
	}

	if err := label.Relabel(cfile, b.MountLabel, false); err != nil {
		return "", errors.Wrapf(err, "error relabeling %q in container %q", cfile, b.ContainerID)
	}

	return cfile, nil
}

func setupMaskedPaths(g *generate.Generator) {
	for _, mp := range []string{
		"/proc/kcore",
		"/proc/latency_stats",
		"/proc/timer_list",
		"/proc/timer_stats",
		"/proc/sched_debug",
		"/proc/scsi",
		"/sys/firmware",
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

func setupSeccomp(spec *specs.Spec, seccompProfilePath string) error {
	if seccompProfilePath != "unconfined" {
		if seccompProfilePath != "" {
			seccompProfile, err := ioutil.ReadFile(seccompProfilePath)
			if err != nil {
				return errors.Wrapf(err, "opening seccomp profile (%s) failed", seccompProfilePath)
			}
			seccompConfig, err := seccomp.LoadProfile(string(seccompProfile), spec)
			if err != nil {
				return errors.Wrapf(err, "loading seccomp profile (%s) failed", seccompProfilePath)
			}
			spec.Linux.Seccomp = seccompConfig
		} else {
			seccompConfig, err := seccomp.GetDefaultProfile(spec)
			if err != nil {
				return errors.Wrapf(err, "loading seccomp profile (%s) failed", seccompProfilePath)
			}
			spec.Linux.Seccomp = seccompConfig
		}
	}
	return nil
}

func setupApparmor(spec *specs.Spec, apparmorProfile string) error {
	spec.Process.ApparmorProfile = apparmorProfile
	return nil
}

func setupTerminal(g *generate.Generator, terminalPolicy TerminalPolicy) {
	switch terminalPolicy {
	case DefaultTerminal:
		g.SetProcessTerminal(terminal.IsTerminal(int(os.Stdout.Fd())))
	case WithTerminal:
		g.SetProcessTerminal(true)
	case WithoutTerminal:
		g.SetProcessTerminal(false)
	}
}

func setupNamespaces(g *generate.Generator, namespaceOptions NamespaceOptions, idmapOptions IDMappingOptions, policy NetworkConfigurationPolicy) (configureNetwork bool, configureNetworks []string, configureUTS bool, err error) {
	// Set namespace options in the container configuration.
	hostPidns := false
	configureUserns := false
	specifiedNetwork := false
	for _, namespaceOption := range namespaceOptions {
		switch namespaceOption.Name {
		case string(specs.UserNamespace):
			configureUserns = false
			if !namespaceOption.Host && namespaceOption.Path == "" {
				configureUserns = true
			}
		case string(specs.PIDNamespace):
			hostPidns = namespaceOption.Host
		case string(specs.NetworkNamespace):
			specifiedNetwork = true
			configureNetwork = false
			if !namespaceOption.Host && (namespaceOption.Path == "" || !filepath.IsAbs(namespaceOption.Path)) {
				if namespaceOption.Path != "" && !filepath.IsAbs(namespaceOption.Path) {
					configureNetworks = strings.Split(namespaceOption.Path, ",")
					namespaceOption.Path = ""
				}
				configureNetwork = (policy != NetworkDisabled)
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
		if hostPidns {
			return false, nil, false, errors.Wrapf(err, "unable to mix host PID namespace with user namespace")
		}
		if err := g.AddOrReplaceLinuxNamespace(specs.UserNamespace, ""); err != nil {
			return false, nil, false, errors.Wrapf(err, "error adding new %q namespace for run", string(specs.UserNamespace))
		}
		for _, m := range idmapOptions.UIDMap {
			g.AddLinuxUIDMapping(m.HostID, m.ContainerID, m.Size)
		}
		if len(idmapOptions.UIDMap) == 0 {
			mappings, err := getProcIDMappings("/proc/self/uid_map")
			if err != nil {
				return false, nil, false, err
			}
			for _, m := range mappings {
				g.AddLinuxUIDMapping(m.ContainerID, m.ContainerID, m.Size)
			}
		}
		for _, m := range idmapOptions.GIDMap {
			g.AddLinuxGIDMapping(m.HostID, m.ContainerID, m.Size)
		}
		if len(idmapOptions.GIDMap) == 0 {
			mappings, err := getProcIDMappings("/proc/self/gid_map")
			if err != nil {
				return false, nil, false, err
			}
			for _, m := range mappings {
				g.AddLinuxGIDMapping(m.ContainerID, m.ContainerID, m.Size)
			}
		}
		if !specifiedNetwork {
			if err := g.AddOrReplaceLinuxNamespace(specs.NetworkNamespace, ""); err != nil {
				return false, nil, false, errors.Wrapf(err, "error adding new %q namespace for run", string(specs.NetworkNamespace))
			}
			configureNetwork = (policy != NetworkDisabled)
		}
	} else {
		if err := g.RemoveLinuxNamespace(specs.UserNamespace); err != nil {
			return false, nil, false, errors.Wrapf(err, "error removing %q namespace for run", string(specs.UserNamespace))
		}
		if !specifiedNetwork {
			if err := g.RemoveLinuxNamespace(specs.NetworkNamespace); err != nil {
				return false, nil, false, errors.Wrapf(err, "error removing %q namespace for run", string(specs.NetworkNamespace))
			}
		}
	}
	return configureNetwork, configureNetworks, configureUTS, nil
}

// Run runs the specified command in the container's root filesystem.
func (b *Builder) Run(command []string, options RunOptions) error {
	var user specs.User
	path, err := ioutil.TempDir(os.TempDir(), Package)
	if err != nil {
		return err
	}
	logrus.Debugf("using %q to hold bundle data", path)
	defer func() {
		if err2 := os.RemoveAll(path); err2 != nil {
			logrus.Errorf("error removing %q: %v", path, err2)
		}
	}()
	gp := generate.New()
	g := &gp

	for _, envSpec := range append(b.Env(), options.Env...) {
		env := strings.SplitN(envSpec, "=", 2)
		if len(env) > 1 {
			g.AddProcessEnv(env[0], env[1])
		}
	}

	if b.CommonBuildOpts == nil {
		return errors.Errorf("Invalid format on container you must recreate the container")
	}

	if err := addCommonOptsToSpec(b.CommonBuildOpts, g); err != nil {
		return err
	}

	if len(command) > 0 {
		g.SetProcessArgs(command)
	} else {
		g.SetProcessArgs(nil)
	}
	if options.WorkingDir != "" {
		g.SetProcessCwd(options.WorkingDir)
	} else if b.WorkDir() != "" {
		g.SetProcessCwd(b.WorkDir())
	}
	g.SetProcessSelinuxLabel(b.ProcessLabel)
	g.SetLinuxMountLabel(b.MountLabel)
	mountPoint, err := b.Mount(b.MountLabel)
	if err != nil {
		return err
	}
	defer func() {
		if err2 := b.Unmount(); err2 != nil {
			logrus.Errorf("error unmounting container: %v", err2)
		}
	}()

	setupMaskedPaths(g)
	setupReadOnlyPaths(g)

	g.SetRootPath(mountPoint)

	setupTerminal(g, options.Terminal)

	namespaceOptions := DefaultNamespaceOptions()
	namespaceOptions.AddOrReplace(b.NamespaceOptions...)
	namespaceOptions.AddOrReplace(options.NamespaceOptions...)

	networkPolicy := options.ConfigureNetwork
	if networkPolicy == NetworkDefault {
		networkPolicy = b.ConfigureNetwork
	}

	configureNetwork, configureNetworks, configureUTS, err := setupNamespaces(g, namespaceOptions, b.IDMappingOptions, networkPolicy)
	if err != nil {
		return err
	}

	if configureUTS {
		if options.Hostname != "" {
			g.SetHostname(options.Hostname)
		} else if b.Hostname() != "" {
			g.SetHostname(b.Hostname())
		}
	} else {
		g.SetHostname("")
	}

	if user, err = b.user(mountPoint, options.User); err != nil {
		return err
	}
	g.SetProcessUID(user.UID)
	g.SetProcessGID(user.GID)
	for _, gid := range user.AdditionalGids {
		g.AddProcessAdditionalGid(gid)
	}

	// Now grab the spec from the generator.  Set the generator to nil so that future contributors
	// will quickly be able to tell that they're supposed to be modifying the spec directly from here.
	spec := g.Spec()
	g = nil
	if spec.Process.Cwd == "" {
		spec.Process.Cwd = DefaultWorkingDir
	}
	if err = os.MkdirAll(filepath.Join(mountPoint, spec.Process.Cwd), 0755); err != nil {
		return errors.Wrapf(err, "error ensuring working directory %q exists", spec.Process.Cwd)
	}

	// Set the apparmor profile name.
	if err = setupApparmor(spec, b.CommonBuildOpts.ApparmorProfile); err != nil {
		return err
	}

	// Set the seccomp configuration using the specified profile name.
	if err = setupSeccomp(spec, b.CommonBuildOpts.SeccompProfilePath); err != nil {
		return err
	}

	hostFile, err := b.addNetworkConfig(path, "/etc/hosts")
	if err != nil {
		return err
	}
	resolvFile, err := b.addNetworkConfig(path, "/etc/resolv.conf")
	if err != nil {
		return err
	}

	if err := addHostsToFile(b.CommonBuildOpts.AddHost, hostFile); err != nil {
		return err
	}

	bindFiles := map[string]string{
		"/etc/hosts":       hostFile,
		"/etc/resolv.conf": resolvFile,
	}
	err = b.setupMounts(mountPoint, spec, options.Mounts, bindFiles, b.Volumes(), b.CommonBuildOpts.Volumes, b.CommonBuildOpts.ShmSize, append(b.NamespaceOptions, options.NamespaceOptions...))
	if err != nil {
		return errors.Wrapf(err, "error resolving mountpoints for container")
	}

	if options.CNIConfigDir == "" {
		options.CNIConfigDir = b.CNIConfigDir
		if b.CNIConfigDir == "" {
			options.CNIConfigDir = util.DefaultCNIConfigDir
		}
	}
	if options.CNIPluginPath == "" {
		options.CNIPluginPath = b.CNIPluginPath
		if b.CNIPluginPath == "" {
			options.CNIPluginPath = util.DefaultCNIPluginPath
		}
	}

	return b.runUsingRuntimeSubproc(options, configureNetwork, configureNetworks, spec, mountPoint, path, Package+"-"+filepath.Base(path))
}

type runUsingRuntimeSubprocOptions struct {
	Options           RunOptions
	Spec              *specs.Spec
	RootPath          string
	BundlePath        string
	ConfigureNetwork  bool
	ConfigureNetworks []string
	ContainerName     string
}

func (b *Builder) runUsingRuntimeSubproc(options RunOptions, configureNetwork bool, configureNetworks []string, spec *specs.Spec, rootPath, bundlePath, containerName string) (err error) {
	var confwg sync.WaitGroup
	config, conferr := json.Marshal(runUsingRuntimeSubprocOptions{
		Options:           options,
		Spec:              spec,
		RootPath:          rootPath,
		BundlePath:        bundlePath,
		ConfigureNetwork:  configureNetwork,
		ConfigureNetworks: configureNetworks,
		ContainerName:     containerName,
	})
	if conferr != nil {
		return errors.Wrapf(conferr, "error encoding configuration for %q", runUsingRuntimeCommand)
	}
	cmd := reexec.Command(runUsingRuntimeCommand)
	cmd.Dir = bundlePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), fmt.Sprintf("LOGLEVEL=%d", logrus.GetLevel()))
	preader, pwriter, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "error creating configuration pipe")
	}
	confwg.Add(1)
	go func() {
		_, conferr = io.Copy(pwriter, bytes.NewReader(config))
		confwg.Done()
	}()
	cmd.ExtraFiles = append([]*os.File{preader}, cmd.ExtraFiles...)
	defer preader.Close()
	defer pwriter.Close()
	err = cmd.Run()
	confwg.Wait()
	if err == nil {
		return conferr
	}
	return err
}

func init() {
	reexec.Register(runUsingRuntimeCommand, runUsingRuntimeMain)
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
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, uintptr(1), 0, 0, 0); err != nil {
		fmt.Fprintf(os.Stderr, "prctl(PR_SET_CHILD_SUBREAPER, 1): %v\n", err)
		os.Exit(1)
	}
	// Run the container, start to finish.
	status, err := runUsingRuntime(options.Options, options.ConfigureNetwork, options.ConfigureNetworks, options.Spec, options.RootPath, options.BundlePath, options.ContainerName)
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

func runUsingRuntime(options RunOptions, configureNetwork bool, configureNetworks []string, spec *specs.Spec, rootPath, bundlePath, containerName string) (wstatus unix.WaitStatus, err error) {
	// Set up bind mounts for things that a namespaced user might not be able to get to directly.
	runtime.LockOSThread()
	unmountAll, err := runSetupIntermediateMountNamespace(spec, bundlePath)
	if unmountAll != nil {
		defer func() {
			if err := unmountAll(); err != nil {
				logrus.Error(err)
			}
		}()
	}
	if err != nil {
		return 1, err
	}

	// Write the runtime configuration.
	specbytes, err := json.Marshal(spec)
	if err != nil {
		return 1, err
	}
	if err = ioutils.AtomicWriteFile(filepath.Join(bundlePath, "config.json"), specbytes, 0600); err != nil {
		return 1, errors.Wrapf(err, "error storing runtime configuration")
	}

	logrus.Debugf("config = %v", string(specbytes))

	// Decide which runtime to use.
	runtime := options.Runtime
	if runtime == "" {
		runtime = DefaultRuntime
	}

	// Default to not specifying a console socket location.
	moreCreateArgs := func() []string { return nil }
	// Default to just passing down our stdio.
	getCreateStdio := func() (*os.File, *os.File, *os.File) { return os.Stdin, os.Stdout, os.Stderr }

	// Figure out how we're doing stdio handling, and create pipes and sockets.
	var stdio sync.WaitGroup
	var consoleListener *net.UnixListener
	stdioPipe := make([][]int, 3)
	copyConsole := false
	copyStdio := false
	finishCopy := make([]int, 2)
	if err = unix.Pipe(finishCopy); err != nil {
		return 1, errors.Wrapf(err, "error creating pipe for notifying to stop stdio")
	}
	finishedCopy := make(chan struct{})
	if spec.Process != nil {
		if spec.Process.Terminal {
			copyConsole = true
			// Create a listening socket for accepting the container's terminal's PTY master.
			socketPath := filepath.Join(bundlePath, "console.sock")
			consoleListener, err = net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
			if err != nil {
				return 1, errors.Wrapf(err, "error creating socket to receive terminal descriptor")
			}
			// Add console socket arguments.
			moreCreateArgs = func() []string { return []string{"--console-socket", socketPath} }
		} else {
			copyStdio = true
			// Figure out who should own the pipes.
			uid, gid, err := getHostRootIDs(spec)
			if err != nil {
				return 1, err
			}
			// Create stdio pipes.
			if stdioPipe, err = runMakeStdioPipe(int(uid), int(gid)); err != nil {
				return 1, err
			}
			// Set stdio to our pipes.
			getCreateStdio = func() (*os.File, *os.File, *os.File) {
				stdin := os.NewFile(uintptr(stdioPipe[unix.Stdin][0]), "/dev/stdin")
				stdout := os.NewFile(uintptr(stdioPipe[unix.Stdout][1]), "/dev/stdout")
				stderr := os.NewFile(uintptr(stdioPipe[unix.Stderr][1]), "/dev/stderr")
				return stdin, stdout, stderr
			}
		}
	} else {
		if options.Quiet {
			// Discard stdout.
			getCreateStdio = func() (*os.File, *os.File, *os.File) {
				return os.Stdin, nil, os.Stderr
			}
		}
	}

	// Build the commands that we'll execute.
	pidFile := filepath.Join(bundlePath, "pid")
	args := append(append(append(options.Args, "create", "--bundle", bundlePath, "--pid-file", pidFile), moreCreateArgs()...), containerName)
	create := exec.Command(runtime, args...)
	create.Dir = bundlePath
	stdin, stdout, stderr := getCreateStdio()
	create.Stdin, create.Stdout, create.Stderr = stdin, stdout, stderr
	if create.SysProcAttr == nil {
		create.SysProcAttr = &syscall.SysProcAttr{}
	}
	runSetDeathSig(create)

	args = append(options.Args, "start", containerName)
	start := exec.Command(runtime, args...)
	start.Dir = bundlePath
	start.Stderr = os.Stderr
	runSetDeathSig(start)

	args = append(options.Args, "kill", containerName)
	kill := exec.Command(runtime, args...)
	kill.Dir = bundlePath
	kill.Stderr = os.Stderr
	runSetDeathSig(kill)

	args = append(options.Args, "delete", containerName)
	del := exec.Command(runtime, args...)
	del.Dir = bundlePath
	del.Stderr = os.Stderr
	runSetDeathSig(del)

	// Actually create the container.
	err = create.Run()
	if err != nil {
		return 1, errors.Wrapf(err, "error creating container for %v: %s", spec.Process.Args, runCollectOutput(stdioPipe[unix.Stdout][0], stdioPipe[unix.Stderr][0]))
	}
	defer func() {
		err2 := del.Run()
		if err2 != nil {
			if err == nil {
				err = errors.Wrapf(err2, "error deleting container")
			} else {
				logrus.Infof("error deleting container: %v", err2)
			}
		}
	}()

	// Make sure we read the container's exit status when it exits.
	pidValue, err := ioutil.ReadFile(pidFile)
	if err != nil {
		return 1, errors.Wrapf(err, "error reading pid from %q", pidFile)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidValue)))
	if err != nil {
		return 1, errors.Wrapf(err, "error parsing pid %s as a number", string(pidValue))
	}
	var reaping sync.WaitGroup
	reaping.Add(1)
	go func() {
		defer reaping.Done()
		var err error
		_, err = unix.Wait4(pid, &wstatus, 0, nil)
		if err != nil {
			wstatus = 0
			logrus.Errorf("error waiting for container child process %d: %v\n", pid, err)
		}
	}()

	if configureNetwork {
		teardown, err := runConfigureNetwork(options, configureNetwork, configureNetworks, pid, containerName, spec.Process.Args)
		if teardown != nil {
			defer teardown()
		}
		if err != nil {
			return 1, err
		}
	}

	if copyStdio {
		// We don't need the ends of the pipes that belong to the container.
		stdin.Close()
		if stdout != nil {
			stdout.Close()
		}
		stderr.Close()
	}

	// Handle stdio for the container in the background.
	stdio.Add(1)
	go runCopyStdio(&stdio, copyStdio, stdioPipe, copyConsole, consoleListener, finishCopy, finishedCopy)

	// Start the container.
	err = start.Run()
	if err != nil {
		return 1, errors.Wrapf(err, "error starting container")
	}
	stopped := false
	defer func() {
		if !stopped {
			err2 := kill.Run()
			if err2 != nil {
				if err == nil {
					err = errors.Wrapf(err2, "error stopping container")
				} else {
					logrus.Infof("error stopping container: %v", err2)
				}
			}
		}
	}()

	// Wait for the container to exit.
	for {
		now := time.Now()
		var state specs.State
		args = append(options.Args, "state", containerName)
		stat := exec.Command(runtime, args...)
		stat.Dir = bundlePath
		stat.Stderr = os.Stderr
		stateOutput, stateErr := stat.Output()
		if stateErr != nil {
			return 1, errors.Wrapf(stateErr, "error reading container state")
		}
		if err = json.Unmarshal(stateOutput, &state); err != nil {
			return 1, errors.Wrapf(stateErr, "error parsing container state %q", string(stateOutput))
		}
		switch state.Status {
		case "running":
		case "stopped":
			stopped = true
		default:
			return 1, errors.Errorf("container status unexpectedly changed to %q", state.Status)
		}
		if stopped {
			break
		}
		select {
		case <-finishedCopy:
			stopped = true
		case <-time.After(time.Until(now.Add(100 * time.Millisecond))):
			continue
		}
		if stopped {
			break
		}
	}

	// Close the writing end of the stop-handling-stdio notification pipe.
	unix.Close(finishCopy[1])
	// Wait for the stdio copy goroutine to flush.
	stdio.Wait()
	// Wait until we finish reading the exit status.
	reaping.Wait()

	return wstatus, nil
}

func runCollectOutput(fds ...int) string {
	var b bytes.Buffer
	buf := make([]byte, 8192)
	for _, fd := range fds {
		if err := unix.SetNonblock(fd, true); err != nil {
			logrus.Errorf("error setting pipe descriptor %d nonblocking: %v", fd, err)
			continue
		}
		nread, err := unix.Read(fd, buf)
		if err != nil {
			logrus.Errorf("error reading from pipe %d: %v", fd, err)
			break
		}
		for nread > 0 {
			r := buf[:nread]
			if nwritten, err := b.Write(r); err != nil || nwritten != len(r) {
				if nwritten != len(r) {
					logrus.Errorf("error buffering data from pipe %d: %v", fd, err)
					break
				}
			}
			nread, err = unix.Read(fd, buf)
			if err != nil {
				logrus.Errorf("error reading from pipe %d: %v", fd, err)
				break
			}
		}
	}
	return b.String()
}

func runConfigureNetwork(options RunOptions, configureNetwork bool, configureNetworks []string, pid int, containerName string, command []string) (teardown func(), err error) {
	var netconf, undo []*libcni.NetworkConfigList
	// Scan for CNI configuration files.
	confdir := options.CNIConfigDir
	files, err := libcni.ConfFiles(confdir, []string{".conf"})
	if err != nil {
		return nil, errors.Wrapf(err, "error finding CNI networking configuration files named *.conf in directory %q", confdir)
	}
	lists, err := libcni.ConfFiles(confdir, []string{".conflist"})
	if err != nil {
		return nil, errors.Wrapf(err, "error finding CNI networking configuration list files named *.conflist in directory %q", confdir)
	}
	logrus.Debugf("CNI network configuration file list: %#v", append(files, lists...))
	// Read the CNI configuration files.
	for _, file := range files {
		nc, err := libcni.ConfFromFile(file)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading networking configuration from file %q for %v", file, command)
		}
		if len(configureNetworks) > 0 && nc.Network != nil && (nc.Network.Name == "" || !stringInSlice(nc.Network.Name, configureNetworks)) {
			if nc.Network.Name == "" {
				logrus.Debugf("configuration in %q has no name, skipping it", file)
			} else {
				logrus.Debugf("configuration in %q has name %q, skipping it", file, nc.Network.Name)
			}
			continue
		}
		cl, err := libcni.ConfListFromConf(nc)
		if err != nil {
			return nil, errors.Wrapf(err, "error converting networking configuration from file %q for %v", file, command)
		}
		logrus.Debugf("using network configuration from %q", file)
		netconf = append(netconf, cl)
	}
	for _, list := range lists {
		cl, err := libcni.ConfListFromFile(list)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading networking configuration list from file %q for %v", list, command)
		}
		if len(configureNetworks) > 0 && (cl.Name == "" || !stringInSlice(cl.Name, configureNetworks)) {
			if cl.Name == "" {
				logrus.Debugf("configuration list in %q has no name, skipping it", list)
			} else {
				logrus.Debugf("configuration list in %q has name %q, skipping it", list, cl.Name)
			}
			continue
		}
		logrus.Debugf("using network configuration list from %q", list)
		netconf = append(netconf, cl)
	}
	// Make sure we can access the container's network namespace,
	// even after it exits, to successfully tear down the
	// interfaces.  Ensure this by opening a handle to the network
	// namespace, and using our copy to both configure and
	// deconfigure it.
	netns := fmt.Sprintf("/proc/%d/ns/net", pid)
	netFD, err := unix.Open(netns, unix.O_RDONLY, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening network namespace for %v", command)
	}
	mynetns := fmt.Sprintf("/proc/%d/fd/%d", unix.Getpid(), netFD)
	// Build our search path for the plugins.
	pluginPaths := strings.Split(options.CNIPluginPath, string(os.PathListSeparator))
	cni := libcni.CNIConfig{Path: pluginPaths}
	// Configure the interfaces.
	rtconf := make(map[*libcni.NetworkConfigList]*libcni.RuntimeConf)
	teardown = func() {
		for _, nc := range undo {
			if err = cni.DelNetworkList(nc, rtconf[nc]); err != nil {
				logrus.Errorf("error cleaning up network %v for %v: %v", rtconf[nc].IfName, command, err)
			}
		}
		unix.Close(netFD)
	}
	for i, nc := range netconf {
		// Build the runtime config for use with this network configuration.
		rtconf[nc] = &libcni.RuntimeConf{
			ContainerID:    containerName,
			NetNS:          mynetns,
			IfName:         fmt.Sprintf("if%d", i),
			Args:           [][2]string{},
			CapabilityArgs: map[string]interface{}{},
		}
		// Bring it up.
		_, err := cni.AddNetworkList(nc, rtconf[nc])
		if err != nil {
			return teardown, errors.Wrapf(err, "error configuring network list %v for %v", rtconf[nc].IfName, command)
		}
		// Add it to the list of networks to take down when the container process exits.
		undo = append([]*libcni.NetworkConfigList{nc}, undo...)
	}
	return teardown, nil
}

func runCopyStdio(stdio *sync.WaitGroup, copyStdio bool, stdioPipe [][]int, copyConsole bool, consoleListener *net.UnixListener, finishCopy []int, finishedCopy chan struct{}) {
	defer func() {
		unix.Close(finishCopy[0])
		if copyStdio {
			unix.Close(stdioPipe[unix.Stdin][1])
			unix.Close(stdioPipe[unix.Stdout][0])
			unix.Close(stdioPipe[unix.Stderr][0])
		}
		stdio.Done()
		finishedCopy <- struct{}{}
	}()
	// If we're not doing I/O handling, we're done.
	if !copyConsole && !copyStdio {
		return
	}
	terminalFD := -1
	if copyConsole {
		// Accept a connection over our listening socket.
		fd, err := runAcceptTerminal(consoleListener)
		if err != nil {
			logrus.Errorf("%v", err)
			return
		}
		terminalFD = fd
		// Set our terminal's mode to raw, to pass handling of special
		// terminal input to the terminal in the container.
		state, err := terminal.MakeRaw(unix.Stdin)
		if err != nil {
			logrus.Warnf("error setting terminal state: %v", err)
		} else {
			defer func() {
				if err = terminal.Restore(unix.Stdin, state); err != nil {
					logrus.Errorf("unable to restore terminal state: %v", err)
				}
			}()
			// FIXME - if we're connected to a terminal, we should be
			// passing the updated terminal size down when we receive a
			// SIGWINCH.
		}
	}
	// Track how many descriptors we're expecting data from.
	reading := 0
	// Map describing where data on an incoming descriptor should go.
	relayMap := make(map[int]int)
	// Map describing incoming descriptors.
	relayDesc := make(map[int]string)
	// Buffers.
	relayBuffer := make(map[int]*bytes.Buffer)
	if copyConsole {
		// Input from our stdin, output from the terminal descriptor.
		relayMap[unix.Stdin] = terminalFD
		relayDesc[unix.Stdin] = "stdin"
		relayBuffer[unix.Stdin] = new(bytes.Buffer)
		relayMap[terminalFD] = unix.Stdout
		relayDesc[terminalFD] = "container terminal output"
		relayBuffer[terminalFD] = new(bytes.Buffer)
		reading = 2
	}
	if copyStdio {
		// Input from our stdin, output from the stdout and stderr pipes.
		relayMap[unix.Stdin] = stdioPipe[unix.Stdin][1]
		relayDesc[unix.Stdin] = "stdin"
		relayBuffer[unix.Stdin] = new(bytes.Buffer)
		relayMap[stdioPipe[unix.Stdout][0]] = unix.Stdout
		relayDesc[stdioPipe[unix.Stdout][0]] = "container stdout"
		relayBuffer[stdioPipe[unix.Stdout][0]] = new(bytes.Buffer)
		relayMap[stdioPipe[unix.Stderr][0]] = unix.Stderr
		relayDesc[stdioPipe[unix.Stderr][0]] = "container stderr"
		relayBuffer[stdioPipe[unix.Stderr][0]] = new(bytes.Buffer)
		reading = 3
	}
	// Set our reading descriptors to non-blocking.
	for fd := range relayMap {
		if err := unix.SetNonblock(fd, true); err != nil {
			logrus.Errorf("error setting %s to nonblocking: %v", relayDesc[fd], err)
			return
		}
	}
	// Pass data back and forth.
	for {
		// Start building the list of descriptors to poll.
		pollFds := make([]unix.PollFd, 0, reading+1)
		// Poll for a notification that we should stop handling stdio.
		pollFds = append(pollFds, unix.PollFd{Fd: int32(finishCopy[0]), Events: unix.POLLIN | unix.POLLHUP})
		// Poll on our reading descriptors.
		for rfd := range relayMap {
			pollFds = append(pollFds, unix.PollFd{Fd: int32(rfd), Events: unix.POLLIN | unix.POLLHUP})
		}
		buf := make([]byte, 8192)
		// Wait for new data from any input descriptor, or a notification that we're done.
		nevents, err := unix.Poll(pollFds, -1)
		if err != nil {
			if errno, isErrno := err.(syscall.Errno); isErrno {
				switch errno {
				case syscall.EINTR:
					continue
				default:
					logrus.Errorf("unable to wait for stdio/terminal data to relay: %v", err)
					return
				}
			} else {
				logrus.Errorf("unable to wait for stdio/terminal data to relay: %v", err)
				return
			}
		}
		if nevents == 0 {
			logrus.Errorf("unexpected no data, no error waiting for terminal data to relay")
			return
		}
		var removes []int
		for _, pollFd := range pollFds {
			// If this descriptor's just been closed from the other end, mark it for
			// removal from the set that we're checking for.
			if pollFd.Revents&unix.POLLHUP == unix.POLLHUP {
				removes = append(removes, int(pollFd.Fd))
			}
			// If the EPOLLIN flag isn't set, then there's no data to be read from this descriptor.
			if pollFd.Revents&unix.POLLIN == 0 {
				// If we're using pipes and it's our stdin, close the writing end
				// of the corresponding pipe.
				if copyStdio && int(pollFd.Fd) == unix.Stdin {
					unix.Close(stdioPipe[unix.Stdin][1])
					stdioPipe[unix.Stdin][1] = -1
				}
				continue
			}
			// Copy whatever we read to wherever it needs to be sent.
			readFD := int(pollFd.Fd)
			writeFD, needToRelay := relayMap[readFD]
			if needToRelay {
				n, err := unix.Read(readFD, buf)
				if err != nil {
					if errno, isErrno := err.(syscall.Errno); isErrno {
						switch errno {
						default:
							logrus.Errorf("unable to read %s: %v", relayDesc[readFD], err)
						case syscall.EINTR, syscall.EAGAIN:
						}
					} else {
						logrus.Errorf("unable to wait for %s data to relay: %v", relayDesc[readFD], err)
					}
					continue
				}
				// If it's zero-length on our stdin and we're
				// using pipes, it's an EOF, so close the stdin
				// pipe's writing end.
				if n == 0 && copyStdio && int(pollFd.Fd) == unix.Stdin {
					unix.Close(stdioPipe[unix.Stdin][1])
					stdioPipe[unix.Stdin][1] = -1
				}
				if n > 0 {
					// Buffer the data in case we're blocked on where they need to go.
					relayBuffer[readFD].Write(buf[:n])
					// Try to drain the buffer.
					n, err = unix.Write(writeFD, relayBuffer[readFD].Bytes())
					if err != nil {
						logrus.Errorf("unable to write %s: %v", relayDesc[readFD], err)
						return
					}
					relayBuffer[readFD].Next(n)
				}
			}
		}
		// Remove any descriptors which we don't need to poll any more from the poll descriptor list.
		for _, remove := range removes {
			delete(relayMap, remove)
			reading--
		}
		if reading == 0 {
			// We have no more open descriptors to read, so we can stop now.
			return
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

func runAcceptTerminal(consoleListener *net.UnixListener) (int, error) {
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
		return -1, errors.Wrapf(err, "error reading socket descriptor: %v")
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
			return -1, errors.Wrapf(err, "error parsing unix rights control message: %v")
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
	// Set the pseudoterminal's size to match our own.
	winsize, err := unix.IoctlGetWinsize(unix.Stdin, unix.TIOCGWINSZ)
	if err != nil {
		logrus.Warnf("error reading size of controlling terminal: %v", err)
		return terminalFD, nil
	}
	err = unix.IoctlSetWinsize(terminalFD, unix.TIOCSWINSZ, winsize)
	if err != nil {
		logrus.Warnf("error setting size of container pseudoterminal: %v", err)
	}
	return terminalFD, nil
}

func runSetDeathSig(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if cmd.SysProcAttr.Pdeathsig == 0 {
		cmd.SysProcAttr.Pdeathsig = syscall.SIGTERM
	}
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

// Create and bind mount all bind-mount sources into a subdirectory of
// bundlePath that can only be reached by the root user of the container's user
// namespace.
func runSetupIntermediateMountNamespace(spec *specs.Spec, bundlePath string) (unmountAll func() error, err error) {
	defer func() {
		// Strip "nobuildahbind" options out of the spec, at least. */
		for i := range spec.Mounts {
			if stringInSlice("nobuildahbind", spec.Mounts[i].Options) {
				prunedOptions := make([]string, 0, len(spec.Mounts[i].Options))
				for _, option := range spec.Mounts[i].Options {
					if option != "nobuildahbind" {
						prunedOptions = append(prunedOptions, option)
					}
				}
				spec.Mounts[i].Options = prunedOptions
			}
		}
	}()

	// Create a new mount namespace in which to do the things we're doing.
	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return nil, errors.Wrapf(err, "error creating new mount namespace for %v", spec.Process.Args)
	}

	// Make all of our mounts private to our namespace.
	if err := mount.MakePrivate("/"); err != nil {
		return nil, errors.Wrapf(err, "error making mounts private to mount namespace for %v", spec.Process.Args)
	}

	// We expect a root directory to be defined.
	if spec.Root == nil {
		return nil, errors.Errorf("configuration has no root filesystem?")
	}
	rootPath := spec.Root.Path

	// Make sure the bundle directory is searchable.  We created it with
	// TempDir(), so it should have started with permissions set to 0700.
	info, err := os.Stat(bundlePath)
	if err != nil {
		return nil, errors.Wrapf(err, "error checking permissions on %q", bundlePath)
	}
	if err = os.Chmod(bundlePath, info.Mode()|0111); err != nil {
		return nil, errors.Wrapf(err, "error loosening permissions on %q", bundlePath)
	}

	// Figure out who needs to be able to reach these bind mounts in order
	// for the container to be started.
	rootUID, rootGID, err := getHostRootIDs(spec)
	if err != nil {
		return nil, err
	}

	// Hand back a callback that the caller can use to clean up everything
	// we're doing here.
	unmount := []string{}
	unmountAll = func() (err error) {
		for _, mountpoint := range unmount {
			subdirs := []string{mountpoint}
			var infos []*mount.Info
			infos, err = mount.GetMounts()
			// Gather up mountpoints below this one, since we did
			// some recursive mounting.
			if err == nil {
				for _, info := range infos {
					if strings.HasPrefix(info.Mountpoint, mountpoint) {
						subdirs = append(subdirs, info.Mountpoint)
					}
				}
			}
			// Unmount all of the lower mountpoints, then the
			// mountpoint itself.
			sort.Strings(subdirs)
			for i := range subdirs {
				var err2 error
				subdir := subdirs[len(subdirs)-i-1]
				for err2 == nil {
					err2 = unix.Unmount(subdir, 0)
				}
				if errno, ok := err2.(syscall.Errno); !ok || errno != unix.EINVAL {
					logrus.Warnf("error unmounting %q: %v", mountpoint, err2)
					if err == nil {
						err = err2
					}
				}
			}
			// Remove just the mountpoint.
			if err2 := os.Remove(mountpoint); err2 != nil {
				logrus.Warnf("error removing %q: %v", mountpoint, err2)
				if err == nil {
					err = err2
				}
			}
		}
		return err
	}

	// Create a top-level directory that the "root" user will be able to
	// access, that "root" from containers which use different mappings, or
	// other unprivileged users outside of containers, shouldn't be able to
	// access.
	mnt := filepath.Join(bundlePath, "mnt")
	if err = idtools.MkdirAndChown(mnt, 0100, idtools.IDPair{UID: int(rootUID), GID: int(rootGID)}); err != nil {
		return unmountAll, errors.Wrapf(err, "error creating %q owned by the container's root user", mnt)
	}

	// Make that directory private, and add it to the list of locations we
	// unmount at cleanup time.
	if err = mount.MakeRPrivate(mnt); err != nil {
		return unmountAll, errors.Wrapf(err, "error marking filesystem at %q as private", mnt)
	}
	unmount = append([]string{mnt}, unmount...)

	// Create a bind mount for the root filesystem and add it to the list.
	rootfs := filepath.Join(mnt, "rootfs")
	if err = os.Mkdir(rootfs, 0000); err != nil {
		return unmountAll, errors.Wrapf(err, "error creating directory %q", rootfs)
	}
	if err = unix.Mount(rootPath, rootfs, "", unix.MS_BIND|unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
		return unmountAll, errors.Wrapf(err, "error bind mounting root filesystem from %q to %q", rootPath, rootfs)
	}
	unmount = append([]string{rootfs}, unmount...)
	spec.Root.Path = rootfs

	// Do the same for everything we're binding in.
	mounts := make([]specs.Mount, 0, len(spec.Mounts))
	for i := range spec.Mounts {
		// If we're not using an intermediate, leave it in the list.
		if runLeaveBindMountAlone(spec.Mounts[i]) {
			mounts = append(mounts, spec.Mounts[i])
			continue
		}
		// Check if the source is a directory or something else.
		info, err := os.Stat(spec.Mounts[i].Source)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Warnf("couldn't find %q on host to bind mount into container", spec.Mounts[i].Source)
				continue
			}
			return unmountAll, errors.Wrapf(err, "error checking if %q is a directory", spec.Mounts[i].Source)
		}
		stage := filepath.Join(mnt, fmt.Sprintf("buildah-bind-target-%d", i))
		if info.IsDir() {
			// If the source is a directory, make one to use as the
			// mount target.
			if err = os.Mkdir(stage, 0000); err != nil {
				return unmountAll, errors.Wrapf(err, "error creating directory %q", stage)
			}
		} else {
			// If the source is not a directory, create an empty
			// file to use as the mount target.
			file, err := os.OpenFile(stage, os.O_WRONLY|os.O_CREATE, 0000)
			if err != nil {
				return unmountAll, errors.Wrapf(err, "error creating file %q", stage)
			}
			file.Close()
		}
		// Bind mount the source from wherever it is to a place where
		// we know the runtime helper will be able to get to it...
		if err = unix.Mount(spec.Mounts[i].Source, stage, "", unix.MS_BIND|unix.MS_REC|unix.MS_PRIVATE, ""); err != nil {
			return unmountAll, errors.Wrapf(err, "error bind mounting bind object from %q to %q", spec.Mounts[i].Source, stage)
		}
		spec.Mounts[i].Source = stage
		// ... and update the source location that we'll pass to the
		// runtime to our intermediate location.
		mounts = append(mounts, spec.Mounts[i])
		unmount = append([]string{stage}, unmount...)
	}
	spec.Mounts = mounts

	return unmountAll, nil
}

// Decide if the mount should not be redirected to an intermediate location first.
func runLeaveBindMountAlone(mount specs.Mount) bool {
	// If we know we shouldn't do a redirection for this mount, skip it.
	if stringInSlice("nobuildahbind", mount.Options) {
		return true
	}
	// If we're not bind mounting it in, we don't need to do anything for it.
	if mount.Type != "bind" && !stringInSlice("bind", mount.Options) && !stringInSlice("rbind", mount.Options) {
		return true
	}
	return false
}
