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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/storage/pkg/ioutils"
	"github.com/containers/storage/pkg/reexec"
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
	Terminal TerminalPolicy
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

func (b *Builder) setupMounts(mountPoint string, spec *specs.Spec, optionMounts []specs.Mount, bindFiles map[string]string, builtinVolumes, volumeMounts []string, shmSize string) error {
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
	for dest, src := range bindFiles {
		if haveMount(dest) {
			// Already have something to mount there, so skip this one.
			continue
		}
		mounts = append(mounts, specs.Mount{
			Source:      src,
			Destination: dest,
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

// addNetworkConfig copies files from host and sets them up to bind mount into container
func (b *Builder) addNetworkConfig(rdir, hostPath string) (string, error) {
	stat, err := os.Stat(hostPath)
	if err != nil {
		return "", errors.Wrapf(err, "stat %q failed", hostPath)
	}

	buf, err := ioutil.ReadFile(hostPath)
	if err != nil {
		return "", errors.Wrapf(err, "opening %q failed", hostPath)
	}
	cfile := filepath.Join(rdir, filepath.Base(hostPath))
	if err := ioutil.WriteFile(cfile, buf, stat.Mode()); err != nil {
		return "", errors.Wrapf(err, "opening %q failed", cfile)
	}
	if err = label.Relabel(cfile, b.MountLabel, false); err != nil {
		return "", errors.Wrapf(err, "error relabeling %q in container %q", cfile, b.ContainerID)
	}

	return cfile, nil
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

	// Set the apparmor profile name.
	g.SetProcessApparmorProfile(b.CommonBuildOpts.ApparmorProfile)

	// Set the seccomp configuration using the specified profile name.
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
	err = b.setupMounts(mountPoint, spec, options.Mounts, bindFiles, b.Volumes(), b.CommonBuildOpts.Volumes, b.CommonBuildOpts.ShmSize)
	if err != nil {
		return errors.Wrapf(err, "error resolving mountpoints for container")
	}
	return b.runUsingRuntimeSubproc(options, spec, mountPoint, path, Package+"-"+filepath.Base(path))
}

type runUsingRuntimeSubprocOptions struct {
	Options       RunOptions
	Spec          *specs.Spec
	RootPath      string
	BundlePath    string
	ContainerName string
}

func (b *Builder) runUsingRuntimeSubproc(options RunOptions, spec *specs.Spec, rootPath, bundlePath, containerName string) (err error) {
	var confwg sync.WaitGroup
	config, conferr := json.Marshal(runUsingRuntimeSubprocOptions{
		Options:       options,
		Spec:          spec,
		RootPath:      rootPath,
		BundlePath:    bundlePath,
		ContainerName: containerName,
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
	status, err := runUsingRuntime(options.Options, options.Spec, options.RootPath, options.BundlePath, options.ContainerName)
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

func runUsingRuntime(options RunOptions, spec *specs.Spec, rootPath, bundlePath, containerName string) (wstatus unix.WaitStatus, err error) {
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
			// Create pipes to use for relaying stdio.
			for i := range stdioPipe {
				stdioPipe[i] = make([]int, 2)
				if err = unix.Pipe(stdioPipe[i]); err != nil {
					return 1, errors.Wrapf(err, "error creating pipe for container FD %d", i)
				}
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
		return 1, errors.Wrapf(err, "error creating container for %v", spec.Process.Args)
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
			logrus.Errorf("error waiting for container child process: %v\n", err)
		}
	}()

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
