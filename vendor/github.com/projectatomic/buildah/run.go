package buildah

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containers/storage/pkg/ioutils"
	"github.com/docker/docker/profiles/seccomp"
	units "github.com/docker/go-units"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/pkg/secrets"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	// DefaultWorkingDir is used if none was specified.
	DefaultWorkingDir = "/"
	// DefaultRuntime is the default command to use to run the container.
	DefaultRuntime = "runc"
)

const (
	// DefaultTerminal indicates that this Run invocation should be
	// connected to a pseudoterminal if we're connected to a terminal.
	DefaultTerminal = iota
	// WithoutTerminal indicates that this Run invocation should NOT be
	// connected to a pseudoterminal.
	WithoutTerminal
	// WithTerminal indicates that this Run invocation should be connected
	// to a pseudoterminal.
	WithTerminal
)

// RunOptions can be used to alter how a command is run in the container.
type RunOptions struct {
	// Hostname is the hostname we set for the running container.
	Hostname string
	// Runtime is the name of the command to run.  It should accept the same arguments that runc does.
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
	// NetworkDisabled puts the container into its own network namespace.
	NetworkDisabled bool
	// Terminal provides a way to specify whether or not the command should
	// be run with a pseudoterminal.  By default (DefaultTerminal), a
	// terminal is used if os.Stdout is connected to a terminal, but that
	// decision can be overridden by specifying either WithTerminal or
	// WithoutTerminal.
	Terminal int
	// Quiet tells the run to turn off output to stdout.
	Quiet bool
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
	// RESOURCES - CPU
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

	// RESOURCES - MEMORY
	if commonOpts.Memory != 0 {
		g.SetLinuxResourcesMemoryLimit(commonOpts.Memory)
	}
	if commonOpts.MemorySwap != 0 {
		g.SetLinuxResourcesMemorySwap(commonOpts.MemorySwap)
	}

	if commonOpts.CgroupParent != "" {
		g.SetLinuxCgroupsPath(commonOpts.CgroupParent)
	}

	if err := addRlimits(commonOpts.Ulimit, g); err != nil {
		return err
	}
	if err := addHostsToFile(commonOpts.AddHost, "/etc/hosts"); err != nil {
		return err
	}

	logrus.Debugln("Resources:", commonOpts)
	return nil
}

func (b *Builder) setupMounts(mountPoint string, spec *specs.Spec, optionMounts []specs.Mount, bindFiles, builtinVolumes, volumeMounts []string, shmSize string) error {
	// The passed-in mounts matter the most to us.
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
	// Add mounts from the generated list, unless they conflict.
	for _, specMount := range spec.Mounts {
		if specMount.Destination == "/dev/shm" {
			specMount.Options = []string{"nosuid", "noexec", "nodev", "mode=1777", "size=" + shmSize}
		}
		if haveMount(specMount.Destination) {
			// Already have something to mount there, so skip this one.
			continue
		}
		mounts = append(mounts, specMount)
	}
	// Add bind mounts for important files, unless they conflict.
	for _, boundFile := range bindFiles {
		if haveMount(boundFile) {
			// Already have something to mount there, so skip this one.
			continue
		}
		mounts = append(mounts, specs.Mount{
			Source:      boundFile,
			Destination: boundFile,
			Type:        "bind",
			Options:     []string{"rbind", "ro"},
		})
	}

	cdir, err := b.store.ContainerDirectory(b.ContainerID)
	if err != nil {
		return errors.Wrapf(err, "error determining work directory for container %q", b.ContainerID)
	}

	// Add secrets mounts
	secretMounts := secrets.SecretMounts(b.MountLabel, cdir, b.DefaultMountsFilePath)
	for _, mount := range secretMounts {
		if haveMount(mount.Destination) {
			continue
		}
		mounts = append(mounts, mount)
	}

	// Add temporary copies of the contents of volume locations at the
	// volume locations, unless we already have something there.
	for _, volume := range builtinVolumes {
		if haveMount(volume) {
			// Already mounting something there, no need to bother.
			continue
		}
		subdir := digest.Canonical.FromString(volume).Hex()
		volumePath := filepath.Join(cdir, "buildah-volumes", subdir)
		// If we need to, initialize the volume path's initial contents.
		if _, err = os.Stat(volumePath); os.IsNotExist(err) {
			if err = os.MkdirAll(volumePath, 0755); err != nil {
				return errors.Wrapf(err, "error creating directory %q for volume %q in container %q", volumePath, volume, b.ContainerID)
			}
			if err = label.Relabel(volumePath, b.MountLabel, false); err != nil {
				return errors.Wrapf(err, "error relabeling directory %q for volume %q in container %q", volumePath, volume, b.ContainerID)
			}
			srcPath := filepath.Join(mountPoint, volume)
			if err = copyWithTar(srcPath, volumePath); err != nil && !os.IsNotExist(err) {
				return errors.Wrapf(err, "error populating directory %q for volume %q in container %q using contents of %q", volumePath, volume, b.ContainerID, srcPath)
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
	// Bind mount volumes given by the user at execution
	var options []string
	for _, i := range volumeMounts {
		spliti := strings.Split(i, ":")
		if len(spliti) > 2 {
			options = strings.Split(spliti[2], ",")
		}
		if haveMount(spliti[1]) {
			continue
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
			if err := label.Relabel(spliti[0], spec.Linux.MountLabel, true); err != nil {
				return errors.Wrapf(err, "relabel failed %q", spliti[0])
			}
		}
		if foundZ {
			if err := label.Relabel(spliti[0], spec.Linux.MountLabel, false); err != nil {
				return errors.Wrapf(err, "relabel failed %q", spliti[0])
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
	// Set the list in the spec.
	spec.Mounts = mounts
	return nil
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
	g := generate.New()

	for _, envSpec := range append(b.Env(), options.Env...) {
		env := strings.SplitN(envSpec, "=", 2)
		if len(env) > 1 {
			g.AddProcessEnv(env[0], env[1])
		}
	}

	if b.CommonBuildOpts == nil {
		return errors.Errorf("Invalid format on container you must recreate the container")
	}

	if err := addCommonOptsToSpec(b.CommonBuildOpts, &g); err != nil {
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
	if options.Hostname != "" {
		g.SetHostname(options.Hostname)
	} else if b.Hostname() != "" {
		g.SetHostname(b.Hostname())
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
	g.SetRootPath(mountPoint)
	switch options.Terminal {
	case DefaultTerminal:
		g.SetProcessTerminal(terminal.IsTerminal(int(os.Stdout.Fd())))
	case WithTerminal:
		g.SetProcessTerminal(true)
	case WithoutTerminal:
		g.SetProcessTerminal(false)
	}
	if !options.NetworkDisabled {
		if err = g.RemoveLinuxNamespace("network"); err != nil {
			return errors.Wrapf(err, "error removing network namespace for run")
		}
	}
	user, err = b.user(mountPoint, options.User)
	if err != nil {
		return err
	}
	g.SetProcessUID(user.UID)
	g.SetProcessGID(user.GID)
	spec := g.Spec()
	if spec.Process.Cwd == "" {
		spec.Process.Cwd = DefaultWorkingDir
	}
	if err = os.MkdirAll(filepath.Join(mountPoint, spec.Process.Cwd), 0755); err != nil {
		return errors.Wrapf(err, "error ensuring working directory %q exists", spec.Process.Cwd)
	}

	//Security Opts
	g.SetProcessApparmorProfile(b.CommonBuildOpts.ApparmorProfile)

	// HANDLE SECCOMP
	if b.CommonBuildOpts.SeccompProfilePath != "unconfined" {
		if b.CommonBuildOpts.SeccompProfilePath != "" {
			seccompProfile, err := ioutil.ReadFile(b.CommonBuildOpts.SeccompProfilePath)
			if err != nil {
				return errors.Wrapf(err, "opening seccomp profile (%s) failed", b.CommonBuildOpts.SeccompProfilePath)
			}
			seccompConfig, err := seccomp.LoadProfile(string(seccompProfile), spec)
			if err != nil {
				return errors.Wrapf(err, "loading seccomp profile (%s) failed", b.CommonBuildOpts.SeccompProfilePath)
			}
			spec.Linux.Seccomp = seccompConfig
		} else {
			seccompConfig, err := seccomp.GetDefaultProfile(spec)
			if err != nil {
				return errors.Wrapf(err, "loading seccomp profile (%s) failed", b.CommonBuildOpts.SeccompProfilePath)
			}
			spec.Linux.Seccomp = seccompConfig
		}
	}

	cgroupMnt := specs.Mount{
		Destination: "/sys/fs/cgroup",
		Type:        "cgroup",
		Source:      "cgroup",
		Options:     []string{"nosuid", "noexec", "nodev", "relatime", "ro"},
	}
	g.AddMount(cgroupMnt)

	bindFiles := []string{"/etc/hosts", "/etc/resolv.conf"}
	err = b.setupMounts(mountPoint, spec, options.Mounts, bindFiles, b.Volumes(), b.CommonBuildOpts.Volumes, b.CommonBuildOpts.ShmSize)
	if err != nil {
		return errors.Wrapf(err, "error resolving mountpoints for container")
	}
	specbytes, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	err = ioutils.AtomicWriteFile(filepath.Join(path, "config.json"), specbytes, 0600)
	if err != nil {
		return errors.Wrapf(err, "error storing runtime configuration")
	}
	logrus.Debugf("config = %v", string(specbytes))
	runtime := options.Runtime
	if runtime == "" {
		runtime = DefaultRuntime
	}
	args := append(options.Args, "run", "-b", path, Package+"-"+b.ContainerID)
	cmd := exec.Command(runtime, args...)
	cmd.Dir = mountPoint
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	if options.Quiet {
		cmd.Stdout = nil
	}
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		logrus.Debugf("error running runc %v: %v", spec.Process.Args, err)
	}
	return err
}
