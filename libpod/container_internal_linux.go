// +build linux

package libpod

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/buildah/pkg/secrets"
	"github.com/containers/common/pkg/apparmor"
	"github.com/containers/common/pkg/config"
	"github.com/containers/libpod/libpod/define"
	"github.com/containers/libpod/libpod/events"
	"github.com/containers/libpod/pkg/annotations"
	"github.com/containers/libpod/pkg/cgroups"
	"github.com/containers/libpod/pkg/criu"
	"github.com/containers/libpod/pkg/lookup"
	"github.com/containers/libpod/pkg/resolvconf"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/containers/libpod/pkg/util"
	"github.com/containers/storage/pkg/archive"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/runc/libcontainer/user"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func (c *Container) mountSHM(shmOptions string) error {
	if err := unix.Mount("shm", c.config.ShmDir, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV,
		label.FormatMountLabel(shmOptions, c.config.MountLabel)); err != nil {
		return errors.Wrapf(err, "failed to mount shm tmpfs %q", c.config.ShmDir)
	}
	return nil
}

func (c *Container) unmountSHM(mount string) error {
	if err := unix.Unmount(mount, 0); err != nil {
		if err != syscall.EINVAL && err != syscall.ENOENT {
			return errors.Wrapf(err, "error unmounting container %s SHM mount %s", c.ID(), mount)
		}
		// If it's just an EINVAL or ENOENT, debug logs only
		logrus.Debugf("container %s failed to unmount %s : %v", c.ID(), mount, err)
	}
	return nil
}

// prepare mounts the container and sets up other required resources like net
// namespaces
func (c *Container) prepare() error {
	var (
		wg                              sync.WaitGroup
		netNS                           ns.NetNS
		networkStatus                   []*cnitypes.Result
		createNetNSErr, mountStorageErr error
		mountPoint                      string
		tmpStateLock                    sync.Mutex
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		// Set up network namespace if not already set up
		if c.config.CreateNetNS && c.state.NetNS == nil && !c.config.PostConfigureNetNS {
			netNS, networkStatus, createNetNSErr = c.runtime.createNetNS(c)
			if createNetNSErr != nil {
				return
			}

			tmpStateLock.Lock()
			defer tmpStateLock.Unlock()

			// Assign NetNS attributes to container
			c.state.NetNS = netNS
			c.state.NetworkStatus = networkStatus
		}

		// handle rootless network namespace setup
		if c.state.NetNS != nil && c.config.NetMode.IsSlirp4netns() && !c.config.PostConfigureNetNS {
			createNetNSErr = c.runtime.setupRootlessNetNS(c)
		}
	}()
	// Mount storage if not mounted
	go func() {
		defer wg.Done()
		mountPoint, mountStorageErr = c.mountStorage()

		if mountStorageErr != nil {
			return
		}

		tmpStateLock.Lock()
		defer tmpStateLock.Unlock()

		// Finish up mountStorage
		c.state.Mounted = true
		c.state.Mountpoint = mountPoint

		logrus.Debugf("Created root filesystem for container %s at %s", c.ID(), c.state.Mountpoint)
	}()

	wg.Wait()

	var createErr error
	if createNetNSErr != nil {
		createErr = createNetNSErr
	}
	if mountStorageErr != nil {
		if createErr != nil {
			logrus.Errorf("Error preparing container %s: %v", c.ID(), createErr)
		}
		createErr = mountStorageErr
	}

	// Only trigger storage cleanup if mountStorage was successful.
	// Otherwise, we may mess up mount counters.
	if createNetNSErr != nil && mountStorageErr == nil {
		if err := c.cleanupStorage(); err != nil {
			// createErr is guaranteed non-nil, so print
			// unconditionally
			logrus.Errorf("Error preparing container %s: %v", c.ID(), createErr)
			createErr = errors.Wrapf(err, "error unmounting storage for container %s after network create failure", c.ID())
		}
	}

	// It's OK to unconditionally trigger network cleanup. If the network
	// isn't ready it will do nothing.
	if createErr != nil {
		if err := c.cleanupNetwork(); err != nil {
			logrus.Errorf("Error preparing container %s: %v", c.ID(), createErr)
			createErr = errors.Wrapf(err, "error cleaning up container %s network after setup failure", c.ID())
		}
	}

	if createErr != nil {
		return createErr
	}

	// Save changes to container state
	return c.save()
}

// cleanupNetwork unmounts and cleans up the container's network
func (c *Container) cleanupNetwork() error {
	if c.config.NetNsCtr != "" {
		return nil
	}
	netDisabled, err := c.NetworkDisabled()
	if err != nil {
		return err
	}
	if netDisabled {
		return nil
	}
	if c.state.NetNS == nil {
		logrus.Debugf("Network is already cleaned up, skipping...")
		return nil
	}

	// Stop the container's network namespace (if it has one)
	if err := c.runtime.teardownNetNS(c); err != nil {
		logrus.Errorf("unable to cleanup network for container %s: %q", c.ID(), err)
	}

	c.state.NetNS = nil
	c.state.NetworkStatus = nil

	if c.valid {
		return c.save()
	}

	return nil
}

func (c *Container) getUserOverrides() *lookup.Overrides {
	var hasPasswdFile, hasGroupFile bool
	overrides := lookup.Overrides{}
	for _, m := range c.config.Spec.Mounts {
		if m.Destination == "/etc/passwd" {
			overrides.ContainerEtcPasswdPath = m.Source
			hasPasswdFile = true
		}
		if m.Destination == "/etc/group" {
			overrides.ContainerEtcGroupPath = m.Source
			hasGroupFile = true
		}
		if m.Destination == "/etc" {
			if !hasPasswdFile {
				overrides.ContainerEtcPasswdPath = filepath.Join(m.Source, "passwd")
			}
			if !hasGroupFile {
				overrides.ContainerEtcGroupPath = filepath.Join(m.Source, "group")
			}
		}
	}
	return &overrides
}

// Generate spec for a container
// Accepts a map of the container's dependencies
func (c *Container) generateSpec(ctx context.Context) (*spec.Spec, error) {
	span, _ := opentracing.StartSpanFromContext(ctx, "generateSpec")
	span.SetTag("type", "container")
	defer span.Finish()

	overrides := c.getUserOverrides()
	execUser, err := lookup.GetUserGroupInfo(c.state.Mountpoint, c.config.User, overrides)
	if err != nil {
		return nil, err
	}

	g := generate.NewFromSpec(c.config.Spec)

	// If network namespace was requested, add it now
	if c.config.CreateNetNS {
		if c.config.PostConfigureNetNS {
			if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), ""); err != nil {
				return nil, err
			}
		} else {
			if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), c.state.NetNS.Path()); err != nil {
				return nil, err
			}
		}
	}

	// Apply AppArmor checks and load the default profile if needed.
	if !c.config.Privileged {
		updatedProfile, err := apparmor.CheckProfileAndLoadDefault(c.config.Spec.Process.ApparmorProfile)
		if err != nil {
			return nil, err
		}
		g.SetProcessApparmorProfile(updatedProfile)
	}

	if err := c.makeBindMounts(); err != nil {
		return nil, err
	}

	// Check if the spec file mounts contain the label Relabel flags z or Z.
	// If they do, relabel the source directory and then remove the option.
	for i := range g.Config.Mounts {
		m := &g.Config.Mounts[i]
		var options []string
		for _, o := range m.Options {
			switch o {
			case "z":
				fallthrough
			case "Z":
				if err := label.Relabel(m.Source, c.MountLabel(), label.IsShared(o)); err != nil {
					return nil, errors.Wrapf(err, "relabel failed %q", m.Source)
				}

			default:
				options = append(options, o)
			}
		}
		m.Options = options
	}

	g.SetProcessSelinuxLabel(c.ProcessLabel())
	g.SetLinuxMountLabel(c.MountLabel())

	// Add named volumes
	for _, namedVol := range c.config.NamedVolumes {
		volume, err := c.runtime.GetVolume(namedVol.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "error retrieving volume %s to add to container %s", namedVol.Name, c.ID())
		}
		mountPoint := volume.MountPoint()
		volMount := spec.Mount{
			Type:        "bind",
			Source:      mountPoint,
			Destination: namedVol.Dest,
			Options:     namedVol.Options,
		}
		g.AddMount(volMount)
	}

	// Add bind mounts to container
	for dstPath, srcPath := range c.state.BindMounts {
		newMount := spec.Mount{
			Type:        "bind",
			Source:      srcPath,
			Destination: dstPath,
			Options:     []string{"bind", "private"},
		}
		if c.IsReadOnly() && dstPath != "/dev/shm" {
			newMount.Options = append(newMount.Options, "ro", "nosuid", "noexec", "nodev")
		}
		if !MountExists(g.Mounts(), dstPath) {
			g.AddMount(newMount)
		} else {
			logrus.Warnf("User mount overriding libpod mount at %q", dstPath)
		}
	}

	hasHomeSet := false
	for _, s := range c.config.Spec.Process.Env {
		if strings.HasPrefix(s, "HOME=") {
			hasHomeSet = true
			break
		}
	}
	if !hasHomeSet {
		c.config.Spec.Process.Env = append(c.config.Spec.Process.Env, fmt.Sprintf("HOME=%s", execUser.Home))
	}

	if c.config.User != "" {
		// User and Group must go together
		g.SetProcessUID(uint32(execUser.Uid))
		g.SetProcessGID(uint32(execUser.Gid))
	}

	// Add addition groups if c.config.GroupAdd is not empty
	if len(c.config.Groups) > 0 {
		gids, err := lookup.GetContainerGroups(c.config.Groups, c.state.Mountpoint, overrides)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking up supplemental groups for container %s", c.ID())
		}
		for _, gid := range gids {
			g.AddProcessAdditionalGid(gid)
		}
	}

	if c.config.Systemd {
		if err := c.setupSystemd(g.Mounts(), g); err != nil {
			return nil, errors.Wrapf(err, "error adding systemd-specific mounts")
		}
	}

	// Look up and add groups the user belongs to, if a group wasn't directly specified
	if !rootless.IsRootless() && !strings.Contains(c.config.User, ":") {
		for _, gid := range execUser.Sgids {
			g.AddProcessAdditionalGid(uint32(gid))
		}
	}

	// Add shared namespaces from other containers
	if c.config.IPCNsCtr != "" {
		if err := c.addNamespaceContainer(&g, IPCNS, c.config.IPCNsCtr, spec.IPCNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.MountNsCtr != "" {
		if err := c.addNamespaceContainer(&g, MountNS, c.config.MountNsCtr, spec.MountNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.NetNsCtr != "" {
		if err := c.addNamespaceContainer(&g, NetNS, c.config.NetNsCtr, spec.NetworkNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.PIDNsCtr != "" {
		if err := c.addNamespaceContainer(&g, PIDNS, c.config.PIDNsCtr, spec.PIDNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.UserNsCtr != "" {
		if err := c.addNamespaceContainer(&g, UserNS, c.config.UserNsCtr, spec.UserNamespace); err != nil {
			return nil, err
		}
		if len(g.Config.Linux.UIDMappings) == 0 {
			// runc complains if no mapping is specified, even if we join another ns.  So provide a dummy mapping
			g.AddLinuxUIDMapping(uint32(0), uint32(0), uint32(1))
			g.AddLinuxGIDMapping(uint32(0), uint32(0), uint32(1))
		}
	}

	for _, i := range c.config.Spec.Linux.Namespaces {
		if i.Type == spec.UTSNamespace {
			hostname := c.Hostname()
			g.SetHostname(hostname)
			g.AddProcessEnv("HOSTNAME", hostname)
			break
		}
	}

	if c.config.UTSNsCtr != "" {
		if err := c.addNamespaceContainer(&g, UTSNS, c.config.UTSNsCtr, spec.UTSNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.CgroupNsCtr != "" {
		if err := c.addNamespaceContainer(&g, CgroupNS, c.config.CgroupNsCtr, spec.CgroupNamespace); err != nil {
			return nil, err
		}
	}

	if c.config.IDMappings.AutoUserNs {
		if err := g.AddOrReplaceLinuxNamespace(string(spec.UserNamespace), ""); err != nil {
			return nil, err
		}
		g.ClearLinuxUIDMappings()
		for _, uidmap := range c.config.IDMappings.UIDMap {
			g.AddLinuxUIDMapping(uint32(uidmap.HostID), uint32(uidmap.ContainerID), uint32(uidmap.Size))
		}
		g.ClearLinuxGIDMappings()
		for _, gidmap := range c.config.IDMappings.GIDMap {
			g.AddLinuxGIDMapping(uint32(gidmap.HostID), uint32(gidmap.ContainerID), uint32(gidmap.Size))
		}
	}

	g.SetRootPath(c.state.Mountpoint)
	g.AddAnnotation(annotations.Created, c.config.CreatedTime.Format(time.RFC3339Nano))
	g.AddAnnotation("org.opencontainers.image.stopSignal", fmt.Sprintf("%d", c.config.StopSignal))

	if _, exists := g.Config.Annotations[annotations.ContainerManager]; !exists {
		g.AddAnnotation(annotations.ContainerManager, annotations.ContainerManagerLibpod)
	}

	// Only add container environment variable if not already present
	foundContainerEnv := false
	for _, env := range g.Config.Process.Env {
		if strings.HasPrefix(env, "container=") {
			foundContainerEnv = true
			break
		}
	}
	if !foundContainerEnv {
		g.AddProcessEnv("container", "libpod")
	}

	cgroupPath, err := c.getOCICgroupPath()
	if err != nil {
		return nil, err
	}
	g.SetLinuxCgroupsPath(cgroupPath)

	// Mounts need to be sorted so paths will not cover other paths
	mounts := sortMounts(g.Mounts())
	g.ClearMounts()

	// Determine property of RootPropagation based on volume properties. If
	// a volume is shared, then keep root propagation shared. This should
	// work for slave and private volumes too.
	//
	// For slave volumes, it can be either [r]shared/[r]slave.
	//
	// For private volumes any root propagation value should work.
	rootPropagation := ""
	for _, m := range mounts {
		// We need to remove all symlinks from tmpfs mounts.
		// Runc and other runtimes may choke on them.
		// Easy solution: use securejoin to do a scoped evaluation of
		// the links, then trim off the mount prefix.
		if m.Type == "tmpfs" {
			finalPath, err := securejoin.SecureJoin(c.state.Mountpoint, m.Destination)
			if err != nil {
				return nil, errors.Wrapf(err, "error resolving symlinks for mount destination %s", m.Destination)
			}
			trimmedPath := strings.TrimPrefix(finalPath, strings.TrimSuffix(c.state.Mountpoint, "/"))
			m.Destination = trimmedPath
		}
		g.AddMount(m)
		for _, opt := range m.Options {
			switch opt {
			case MountShared, MountRShared:
				if rootPropagation != MountShared && rootPropagation != MountRShared {
					rootPropagation = MountShared
				}
			case MountSlave, MountRSlave:
				if rootPropagation != MountShared && rootPropagation != MountRShared && rootPropagation != MountSlave && rootPropagation != MountRSlave {
					rootPropagation = MountRSlave
				}
			}
		}
	}

	if rootPropagation != "" {
		logrus.Debugf("set root propagation to %q", rootPropagation)
		if err := g.SetLinuxRootPropagation(rootPropagation); err != nil {
			return nil, err
		}
	}

	// Warning: precreate hooks may alter g.Config in place.
	if c.state.ExtensionStageHooks, err = c.setupOCIHooks(ctx, g.Config); err != nil {
		return nil, errors.Wrapf(err, "error setting up OCI Hooks")
	}

	return g.Config, nil
}

// systemd expects to have /run, /run/lock and /tmp on tmpfs
// It also expects to be able to write to /sys/fs/cgroup/systemd and /var/log/journal
func (c *Container) setupSystemd(mounts []spec.Mount, g generate.Generator) error {
	options := []string{"rw", "rprivate", "noexec", "nosuid", "nodev"}
	for _, dest := range []string{"/run", "/run/lock"} {
		if MountExists(mounts, dest) {
			continue
		}
		tmpfsMnt := spec.Mount{
			Destination: dest,
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     append(options, "tmpcopyup", "size=65536k"),
		}
		g.AddMount(tmpfsMnt)
	}
	for _, dest := range []string{"/tmp", "/var/log/journal"} {
		if MountExists(mounts, dest) {
			continue
		}
		tmpfsMnt := spec.Mount{
			Destination: dest,
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     append(options, "tmpcopyup"),
		}
		g.AddMount(tmpfsMnt)
	}

	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return err
	}

	if unified {
		g.RemoveMount("/sys/fs/cgroup")

		hasCgroupNs := false
		for _, ns := range c.config.Spec.Linux.Namespaces {
			if ns.Type == spec.CgroupNamespace {
				hasCgroupNs = true
				break
			}
		}

		var systemdMnt spec.Mount
		if hasCgroupNs {
			systemdMnt = spec.Mount{
				Destination: "/sys/fs/cgroup",
				Type:        "cgroup",
				Source:      "cgroup",
				Options:     []string{"private", "rw"},
			}
		} else {
			systemdMnt = spec.Mount{
				Destination: "/sys/fs/cgroup",
				Type:        "bind",
				Source:      "/sys/fs/cgroup",
				Options:     []string{"bind", "private", "rw"},
			}
		}
		g.AddMount(systemdMnt)
	} else {
		systemdMnt := spec.Mount{
			Destination: "/sys/fs/cgroup/systemd",
			Type:        "bind",
			Source:      "/sys/fs/cgroup/systemd",
			Options:     []string{"bind", "nodev", "noexec", "nosuid"},
		}
		g.AddMount(systemdMnt)
		g.AddLinuxMaskedPaths("/sys/fs/cgroup/systemd/release_agent")
	}

	return nil
}

// Add an existing container's namespace to the spec
func (c *Container) addNamespaceContainer(g *generate.Generator, ns LinuxNS, ctr string, specNS spec.LinuxNamespaceType) error {
	nsCtr, err := c.runtime.state.Container(ctr)
	if err != nil {
		return errors.Wrapf(err, "error retrieving dependency %s of container %s from state", ctr, c.ID())
	}

	if specNS == spec.UTSNamespace {
		hostname := nsCtr.Hostname()
		g.SetHostname(hostname)
		g.AddProcessEnv("HOSTNAME", hostname)
	}

	// TODO need unlocked version of this for use in pods
	nsPath, err := nsCtr.NamespacePath(ns)
	if err != nil {
		return err
	}

	if err := g.AddOrReplaceLinuxNamespace(string(specNS), nsPath); err != nil {
		return err
	}

	return nil
}

func (c *Container) exportCheckpoint(dest string, ignoreRootfs bool) (err error) {
	if (len(c.config.NamedVolumes) > 0) || (len(c.Dependencies()) > 0) {
		return errors.Errorf("Cannot export checkpoints of containers with named volumes or dependencies")
	}
	logrus.Debugf("Exporting checkpoint image of container %q to %q", c.ID(), dest)

	includeFiles := []string{
		"checkpoint",
		"artifacts",
		"ctr.log",
		"config.dump",
		"spec.dump",
		"network.status"}

	// Get root file-system changes included in the checkpoint archive
	rootfsDiffPath := filepath.Join(c.bundlePath(), "rootfs-diff.tar")
	deleteFilesList := filepath.Join(c.bundlePath(), "deleted.files")
	if !ignoreRootfs {
		// To correctly track deleted files, let's go through the output of 'podman diff'
		tarFiles, err := c.runtime.GetDiff("", c.ID())
		if err != nil {
			return errors.Wrapf(err, "error exporting root file-system diff to %q", rootfsDiffPath)
		}
		var rootfsIncludeFiles []string
		var deletedFiles []string

		for _, file := range tarFiles {
			if file.Kind == archive.ChangeAdd {
				rootfsIncludeFiles = append(rootfsIncludeFiles, file.Path)
				continue
			}
			if file.Kind == archive.ChangeDelete {
				deletedFiles = append(deletedFiles, file.Path)
				continue
			}
			fileName, err := os.Stat(file.Path)
			if err != nil {
				continue
			}
			if !fileName.IsDir() && file.Kind == archive.ChangeModify {
				rootfsIncludeFiles = append(rootfsIncludeFiles, file.Path)
				continue
			}
		}

		if len(rootfsIncludeFiles) > 0 {
			rootfsTar, err := archive.TarWithOptions(c.state.Mountpoint, &archive.TarOptions{
				Compression:      archive.Uncompressed,
				IncludeSourceDir: true,
				IncludeFiles:     rootfsIncludeFiles,
			})
			if err != nil {
				return errors.Wrapf(err, "error exporting root file-system diff to %q", rootfsDiffPath)
			}
			rootfsDiffFile, err := os.Create(rootfsDiffPath)
			if err != nil {
				return errors.Wrapf(err, "error creating root file-system diff file %q", rootfsDiffPath)
			}
			defer rootfsDiffFile.Close()
			_, err = io.Copy(rootfsDiffFile, rootfsTar)
			if err != nil {
				return err
			}

			includeFiles = append(includeFiles, "rootfs-diff.tar")
		}

		if len(deletedFiles) > 0 {
			formatJSON, err := json.MarshalIndent(deletedFiles, "", "     ")
			if err != nil {
				return errors.Wrapf(err, "error creating delete files list file %q", deleteFilesList)
			}
			if err := ioutil.WriteFile(deleteFilesList, formatJSON, 0600); err != nil {
				return errors.Wrapf(err, "error creating delete files list file %q", deleteFilesList)
			}

			includeFiles = append(includeFiles, "deleted.files")
		}
	}

	input, err := archive.TarWithOptions(c.bundlePath(), &archive.TarOptions{
		Compression:      archive.Gzip,
		IncludeSourceDir: true,
		IncludeFiles:     includeFiles,
	})

	if err != nil {
		return errors.Wrapf(err, "error reading checkpoint directory %q", c.ID())
	}

	outFile, err := os.Create(dest)
	if err != nil {
		return errors.Wrapf(err, "error creating checkpoint export file %q", dest)
	}
	defer outFile.Close()

	if err := os.Chmod(dest, 0600); err != nil {
		return errors.Wrapf(err, "cannot chmod %q", dest)
	}

	_, err = io.Copy(outFile, input)
	if err != nil {
		return err
	}

	os.Remove(rootfsDiffPath)
	os.Remove(deleteFilesList)

	return nil
}

func (c *Container) checkpointRestoreSupported() (err error) {
	if !criu.CheckForCriu() {
		return errors.Errorf("Checkpoint/Restore requires at least CRIU %d", criu.MinCriuVersion)
	}
	if !c.ociRuntime.SupportsCheckpoint() {
		return errors.Errorf("Configured runtime does not support checkpoint/restore")
	}
	return nil
}

func (c *Container) checkpointRestoreLabelLog(fileName string) (err error) {
	// Create the CRIU log file and label it
	dumpLog := filepath.Join(c.bundlePath(), fileName)

	logFile, err := os.OpenFile(dumpLog, os.O_CREATE, 0600)
	if err != nil {
		return errors.Wrapf(err, "failed to create CRIU log file %q", dumpLog)
	}
	if err := logFile.Close(); err != nil {
		logrus.Errorf("unable to close log file: %q", err)
	}
	if err = label.SetFileLabel(dumpLog, c.MountLabel()); err != nil {
		return errors.Wrapf(err, "failed to label CRIU log file %q", dumpLog)
	}
	return nil
}

func (c *Container) checkpoint(ctx context.Context, options ContainerCheckpointOptions) (err error) {
	if err := c.checkpointRestoreSupported(); err != nil {
		return err
	}

	if c.state.State != define.ContainerStateRunning {
		return errors.Wrapf(define.ErrCtrStateInvalid, "%q is not running, cannot checkpoint", c.state.State)
	}

	if c.AutoRemove() && options.TargetFile == "" {
		return errors.Errorf("Cannot checkpoint containers that have been started with '--rm' unless '--export' is used")
	}

	if err := c.checkpointRestoreLabelLog("dump.log"); err != nil {
		return err
	}

	if err := c.ociRuntime.CheckpointContainer(c, options); err != nil {
		return err
	}

	// Save network.status. This is needed to restore the container with
	// the same IP. Currently limited to one IP address in a container
	// with one interface.
	formatJSON, err := json.MarshalIndent(c.state.NetworkStatus, "", "     ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(c.bundlePath(), "network.status"), formatJSON, 0644); err != nil {
		return err
	}

	defer c.newContainerEvent(events.Checkpoint)

	if options.TargetFile != "" {
		if err = c.exportCheckpoint(options.TargetFile, options.IgnoreRootfs); err != nil {
			return err
		}
	}

	logrus.Debugf("Checkpointed container %s", c.ID())

	if !options.KeepRunning {
		c.state.State = define.ContainerStateStopped

		// Cleanup Storage and Network
		if err := c.cleanup(ctx); err != nil {
			return err
		}
	}

	if !options.Keep {
		cleanup := []string{
			"dump.log",
			"stats-dump",
			"config.dump",
			"spec.dump",
		}
		for _, del := range cleanup {
			file := filepath.Join(c.bundlePath(), del)
			if err := os.Remove(file); err != nil {
				logrus.Debugf("unable to remove file %s", file)
			}
		}
	}

	c.state.FinishedTime = time.Now()
	return c.save()
}

func (c *Container) importCheckpoint(input string) (err error) {
	archiveFile, err := os.Open(input)
	if err != nil {
		return errors.Wrapf(err, "Failed to open checkpoint archive %s for import", input)
	}

	defer archiveFile.Close()
	options := &archive.TarOptions{
		ExcludePatterns: []string{
			// config.dump and spec.dump are only required
			// container creation
			"config.dump",
			"spec.dump",
		},
	}
	err = archive.Untar(archiveFile, c.bundlePath(), options)
	if err != nil {
		return errors.Wrapf(err, "Unpacking of checkpoint archive %s failed", input)
	}

	// Make sure the newly created config.json exists on disk
	g := generate.Generator{Config: c.config.Spec}
	if err = c.saveSpec(g.Config); err != nil {
		return errors.Wrap(err, "Saving imported container specification for restore failed")
	}

	return nil
}

func (c *Container) restore(ctx context.Context, options ContainerCheckpointOptions) (err error) {

	if err := c.checkpointRestoreSupported(); err != nil {
		return err
	}

	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateExited) {
		return errors.Wrapf(define.ErrCtrStateInvalid, "container %s is running or paused, cannot restore", c.ID())
	}

	if options.TargetFile != "" {
		if err = c.importCheckpoint(options.TargetFile); err != nil {
			return err
		}
	}

	// Let's try to stat() CRIU's inventory file. If it does not exist, it makes
	// no sense to try a restore. This is a minimal check if a checkpoint exist.
	if _, err := os.Stat(filepath.Join(c.CheckpointPath(), "inventory.img")); os.IsNotExist(err) {
		return errors.Wrapf(err, "A complete checkpoint for this container cannot be found, cannot restore")
	}

	if err := c.checkpointRestoreLabelLog("restore.log"); err != nil {
		return err
	}

	// If a container is restored multiple times from an exported checkpoint with
	// the help of '--import --name', the restore will fail if during 'podman run'
	// a static container IP was set with '--ip'. The user can tell the restore
	// process to ignore the static IP with '--ignore-static-ip'
	if options.IgnoreStaticIP {
		c.config.StaticIP = nil
	}

	// If a container is restored multiple times from an exported checkpoint with
	// the help of '--import --name', the restore will fail if during 'podman run'
	// a static container MAC address was set with '--mac-address'. The user
	// can tell the restore process to ignore the static MAC with
	// '--ignore-static-mac'
	if options.IgnoreStaticMAC {
		c.config.StaticMAC = nil
	}

	// Read network configuration from checkpoint
	// Currently only one interface with one IP is supported.
	networkStatusFile, err := os.Open(filepath.Join(c.bundlePath(), "network.status"))
	// If the restored container should get a new name, the IP address of
	// the container will not be restored. This assumes that if a new name is
	// specified, the container is restored multiple times.
	// TODO: This implicit restoring with or without IP depending on an
	//       unrelated restore parameter (--name) does not seem like the
	//       best solution.
	if err == nil && options.Name == "" && (!options.IgnoreStaticIP || !options.IgnoreStaticMAC) {
		// The file with the network.status does exist. Let's restore the
		// container with the same IP address / MAC address as during checkpointing.
		defer networkStatusFile.Close()
		var networkStatus []*cnitypes.Result
		networkJSON, err := ioutil.ReadAll(networkStatusFile)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(networkJSON, &networkStatus); err != nil {
			return err
		}
		if !options.IgnoreStaticIP {
			// Take the first IP address
			var IP net.IP
			if len(networkStatus) > 0 {
				if len(networkStatus[0].IPs) > 0 {
					IP = networkStatus[0].IPs[0].Address.IP
				}
			}
			if IP != nil {
				// Tell CNI which IP address we want.
				c.requestedIP = IP
			}
		}
		if !options.IgnoreStaticMAC {
			// Take the first device with a defined sandbox.
			var MAC net.HardwareAddr
			for _, n := range networkStatus[0].Interfaces {
				if n.Sandbox != "" {
					MAC, err = net.ParseMAC(n.Mac)
					if err != nil {
						return errors.Wrapf(err, "failed to parse MAC %v", n.Mac)
					}
					break
				}
			}
			if MAC != nil {
				// Tell CNI which MAC address we want.
				c.requestedMAC = MAC
			}
		}
	}

	defer func() {
		if err != nil {
			if err2 := c.cleanup(ctx); err2 != nil {
				logrus.Errorf("error cleaning up container %s: %v", c.ID(), err2)
			}
		}
	}()

	if err := c.prepare(); err != nil {
		return err
	}

	// Read config
	jsonPath := filepath.Join(c.bundlePath(), "config.json")
	logrus.Debugf("generate.NewFromFile at %v", jsonPath)
	g, err := generate.NewFromFile(jsonPath)
	if err != nil {
		logrus.Debugf("generate.NewFromFile failed with %v", err)
		return err
	}

	// Restoring from an import means that we are doing migration
	if options.TargetFile != "" {
		g.SetRootPath(c.state.Mountpoint)
	}

	// We want to have the same network namespace as before.
	if c.config.CreateNetNS {
		netNSPath := ""
		if !c.config.PostConfigureNetNS {
			netNSPath = c.state.NetNS.Path()
		}

		if err := g.AddOrReplaceLinuxNamespace(string(spec.NetworkNamespace), netNSPath); err != nil {
			return err
		}
	}

	if err := c.makeBindMounts(); err != nil {
		return err
	}

	if options.TargetFile != "" {
		for dstPath, srcPath := range c.state.BindMounts {
			newMount := spec.Mount{
				Type:        "bind",
				Source:      srcPath,
				Destination: dstPath,
				Options:     []string{"bind", "private"},
			}
			if c.IsReadOnly() && dstPath != "/dev/shm" {
				newMount.Options = append(newMount.Options, "ro", "nosuid", "noexec", "nodev")
			}
			if !MountExists(g.Mounts(), dstPath) {
				g.AddMount(newMount)
			}
		}
	}

	// Cleanup for a working restore.
	if err := c.removeConmonFiles(); err != nil {
		return err
	}

	// Save the OCI spec to disk
	if err := c.saveSpec(g.Config); err != nil {
		return err
	}

	// Before actually restarting the container, apply the root file-system changes
	if !options.IgnoreRootfs {
		rootfsDiffPath := filepath.Join(c.bundlePath(), "rootfs-diff.tar")
		if _, err := os.Stat(rootfsDiffPath); err == nil {
			// Only do this if a rootfs-diff.tar actually exists
			rootfsDiffFile, err := os.Open(rootfsDiffPath)
			if err != nil {
				return errors.Wrapf(err, "Failed to open root file-system diff file %s", rootfsDiffPath)
			}
			defer rootfsDiffFile.Close()
			if err := c.runtime.ApplyDiffTarStream(c.ID(), rootfsDiffFile); err != nil {
				return errors.Wrapf(err, "Failed to apply root file-system diff file %s", rootfsDiffPath)
			}
		}
		deletedFilesPath := filepath.Join(c.bundlePath(), "deleted.files")
		if _, err := os.Stat(deletedFilesPath); err == nil {
			deletedFilesFile, err := os.Open(deletedFilesPath)
			if err != nil {
				return errors.Wrapf(err, "Failed to open deleted files file %s", deletedFilesPath)
			}
			defer deletedFilesFile.Close()

			var deletedFiles []string
			deletedFilesJSON, err := ioutil.ReadAll(deletedFilesFile)
			if err != nil {
				return errors.Wrapf(err, "Failed to read deleted files file %s", deletedFilesPath)
			}
			if err := json.Unmarshal(deletedFilesJSON, &deletedFiles); err != nil {
				return errors.Wrapf(err, "Failed to read deleted files file %s", deletedFilesPath)
			}
			for _, deleteFile := range deletedFiles {
				// Using RemoveAll as deletedFiles, which is generated from 'podman diff'
				// lists completely deleted directories as a single entry: 'D /root'.
				err = os.RemoveAll(filepath.Join(c.state.Mountpoint, deleteFile))
				if err != nil {
					return errors.Wrapf(err, "Failed to delete file %s from container %s during restore", deletedFilesPath, c.ID())
				}
			}
		}
	}

	if err := c.ociRuntime.CreateContainer(c, &options); err != nil {
		return err
	}

	logrus.Debugf("Restored container %s", c.ID())

	c.state.State = define.ContainerStateRunning

	if !options.Keep {
		// Delete all checkpoint related files. At this point, in theory, all files
		// should exist. Still ignoring errors for now as the container should be
		// restored and running. Not erroring out just because some cleanup operation
		// failed. Starting with the checkpoint directory
		err = os.RemoveAll(c.CheckpointPath())
		if err != nil {
			logrus.Debugf("Non-fatal: removal of checkpoint directory (%s) failed: %v", c.CheckpointPath(), err)
		}
		cleanup := [...]string{"restore.log", "dump.log", "stats-dump", "stats-restore", "network.status", "rootfs-diff.tar", "deleted.files"}
		for _, del := range cleanup {
			file := filepath.Join(c.bundlePath(), del)
			err = os.Remove(file)
			if err != nil {
				logrus.Debugf("Non-fatal: removal of checkpoint file (%s) failed: %v", file, err)
			}
		}
	}

	return c.save()
}

// Make standard bind mounts to include in the container
func (c *Container) makeBindMounts() error {
	if err := os.Chown(c.state.RunDir, c.RootUID(), c.RootGID()); err != nil {
		return errors.Wrapf(err, "cannot chown run directory %s", c.state.RunDir)
	}

	if c.state.BindMounts == nil {
		c.state.BindMounts = make(map[string]string)
	}
	netDisabled, err := c.NetworkDisabled()
	if err != nil {
		return err
	}

	if !netDisabled {
		// If /etc/resolv.conf and /etc/hosts exist, delete them so we
		// will recreate. Only do this if we aren't sharing them with
		// another container.
		if c.config.NetNsCtr == "" {
			if resolvePath, ok := c.state.BindMounts["/etc/resolv.conf"]; ok {
				if err := os.Remove(resolvePath); err != nil && !os.IsNotExist(err) {
					return errors.Wrapf(err, "error removing container %s resolv.conf", c.ID())
				}
				delete(c.state.BindMounts, "/etc/resolv.conf")
			}
			if hostsPath, ok := c.state.BindMounts["/etc/hosts"]; ok {
				if err := os.Remove(hostsPath); err != nil && !os.IsNotExist(err) {
					return errors.Wrapf(err, "error removing container %s hosts", c.ID())
				}
				delete(c.state.BindMounts, "/etc/hosts")
			}
		}

		if c.config.NetNsCtr != "" && (!c.config.UseImageResolvConf || !c.config.UseImageHosts) {
			// We share a net namespace.
			// We want /etc/resolv.conf and /etc/hosts from the
			// other container. Unless we're not creating both of
			// them.
			var (
				depCtr  *Container
				nextCtr string
			)

			// I don't like infinite loops, but I don't think there's
			// a serious risk of looping dependencies - too many
			// protections against that elsewhere.
			nextCtr = c.config.NetNsCtr
			for {
				depCtr, err = c.runtime.state.Container(nextCtr)
				if err != nil {
					return errors.Wrapf(err, "error fetching dependency %s of container %s", c.config.NetNsCtr, c.ID())
				}
				nextCtr = depCtr.config.NetNsCtr
				if nextCtr == "" {
					break
				}
			}

			// We need that container's bind mounts
			bindMounts, err := depCtr.BindMounts()
			if err != nil {
				return errors.Wrapf(err, "error fetching bind mounts from dependency %s of container %s", depCtr.ID(), c.ID())
			}

			// The other container may not have a resolv.conf or /etc/hosts
			// If it doesn't, don't copy them
			resolvPath, exists := bindMounts["/etc/resolv.conf"]
			if !c.config.UseImageResolvConf && exists {
				c.state.BindMounts["/etc/resolv.conf"] = resolvPath
			}

			// check if dependency container has an /etc/hosts file.
			// It may not have one, so only use it if it does.
			hostsPath, exists := bindMounts["/etc/hosts"]
			if !c.config.UseImageHosts && exists {
				depCtr.lock.Lock()
				// generate a hosts file for the dependency container,
				// based on either its old hosts file, or the default,
				// and add the relevant information from the new container (hosts and IP)
				hostsPath, err = depCtr.appendHosts(hostsPath, c)

				if err != nil {
					depCtr.lock.Unlock()
					return errors.Wrapf(err, "error creating hosts file for container %s which depends on container %s", c.ID(), depCtr.ID())
				}
				depCtr.lock.Unlock()

				// finally, save it in the new container
				c.state.BindMounts["/etc/hosts"] = hostsPath
			}
		} else {
			if !c.config.UseImageResolvConf {
				newResolv, err := c.generateResolvConf()
				if err != nil {
					return errors.Wrapf(err, "error creating resolv.conf for container %s", c.ID())
				}
				c.state.BindMounts["/etc/resolv.conf"] = newResolv
			}

			if !c.config.UseImageHosts {
				newHosts, err := c.generateHosts("/etc/hosts")
				if err != nil {
					return errors.Wrapf(err, "error creating hosts file for container %s", c.ID())
				}
				c.state.BindMounts["/etc/hosts"] = newHosts
			}
		}

		if c.state.BindMounts["/etc/hosts"] != "" {
			if err := label.Relabel(c.state.BindMounts["/etc/hosts"], c.config.MountLabel, true); err != nil {
				return err
			}
		}

		if c.state.BindMounts["/etc/resolv.conf"] != "" {
			if err := label.Relabel(c.state.BindMounts["/etc/resolv.conf"], c.config.MountLabel, true); err != nil {
				return err
			}
		}
	}

	// SHM is always added when we mount the container
	c.state.BindMounts["/dev/shm"] = c.config.ShmDir

	newPasswd, err := c.generatePasswd()
	if err != nil {
		return errors.Wrapf(err, "error creating temporary passwd file for container %s", c.ID())
	}
	if newPasswd != "" {
		// Make /etc/passwd
		if _, ok := c.state.BindMounts["/etc/passwd"]; ok {
			// If it already exists, delete so we can recreate
			delete(c.state.BindMounts, "/etc/passwd")
		}
		logrus.Debugf("adding entry to /etc/passwd for non existent default user")
		c.state.BindMounts["/etc/passwd"] = newPasswd
	}

	// Make /etc/hostname
	// This should never change, so no need to recreate if it exists
	if _, ok := c.state.BindMounts["/etc/hostname"]; !ok {
		hostnamePath, err := c.writeStringToRundir("hostname", c.Hostname())
		if err != nil {
			return errors.Wrapf(err, "error creating hostname file for container %s", c.ID())
		}
		c.state.BindMounts["/etc/hostname"] = hostnamePath
	}

	// Make .containerenv
	// Empty file, so no need to recreate if it exists
	if _, ok := c.state.BindMounts["/run/.containerenv"]; !ok {
		// Empty string for now, but we may consider populating this later
		containerenvPath, err := c.writeStringToRundir(".containerenv", "")
		if err != nil {
			return errors.Wrapf(err, "error creating containerenv file for container %s", c.ID())
		}
		c.state.BindMounts["/run/.containerenv"] = containerenvPath
	}

	// Add Secret Mounts
	secretMounts := secrets.SecretMountsWithUIDGID(c.config.MountLabel, c.state.RunDir, c.runtime.config.Containers.DefaultMountsFile, c.state.Mountpoint, c.RootUID(), c.RootGID(), rootless.IsRootless(), false)
	for _, mount := range secretMounts {
		if _, ok := c.state.BindMounts[mount.Destination]; !ok {
			c.state.BindMounts[mount.Destination] = mount.Source
		}
	}

	return nil
}

// generateResolvConf generates a containers resolv.conf
func (c *Container) generateResolvConf() (string, error) {
	var (
		nameservers    []string
		cniNameServers []string
	)

	resolvConf := "/etc/resolv.conf"
	for _, namespace := range c.config.Spec.Linux.Namespaces {
		if namespace.Type == spec.NetworkNamespace {
			if namespace.Path != "" && !strings.HasPrefix(namespace.Path, "/proc/") {
				definedPath := filepath.Join("/etc/netns", filepath.Base(namespace.Path), "resolv.conf")
				_, err := os.Stat(definedPath)
				if err == nil {
					resolvConf = definedPath
				} else if !os.IsNotExist(err) {
					return "", errors.Wrapf(err, "failed to stat %s", definedPath)
				}
			}
			break
		}
	}

	// Determine the endpoint for resolv.conf in case it is a symlink
	resolvPath, err := filepath.EvalSymlinks(resolvConf)
	if err != nil {
		return "", err
	}

	contents, err := ioutil.ReadFile(resolvPath)
	if err != nil {
		return "", errors.Wrapf(err, "unable to read %s", resolvPath)
	}

	// Ensure that the container's /etc/resolv.conf is compatible with its
	// network configuration.
	// TODO: set ipv6 enable bool more sanely
	resolv, err := resolvconf.FilterResolvDNS(contents, true, c.config.CreateNetNS)
	if err != nil {
		return "", errors.Wrapf(err, "error parsing host resolv.conf")
	}

	// Check if CNI gave back and DNS servers for us to add in
	cniResponse := c.state.NetworkStatus
	for _, i := range cniResponse {
		if i.DNS.Nameservers != nil {
			cniNameServers = append(cniNameServers, i.DNS.Nameservers...)
			logrus.Debugf("adding nameserver(s) from cni response of '%q'", i.DNS.Nameservers)
		}
	}

	var dns []net.IP
	for _, i := range c.runtime.config.Containers.DNSServers {
		result := net.ParseIP(i)
		if result == nil {
			return "", errors.Wrapf(define.ErrInvalidArg, "invalid IP address %s", i)
		}
		dns = append(dns, result)
	}
	dnsServers := append(dns, c.config.DNSServer...)
	// If the user provided dns, it trumps all; then dns masq; then resolv.conf
	switch {
	case len(dnsServers) > 0:

		// We store DNS servers as net.IP, so need to convert to string
		for _, server := range dnsServers {
			nameservers = append(nameservers, server.String())
		}
	case len(cniNameServers) > 0:
		nameservers = append(nameservers, cniNameServers...)
	default:
		// Make a new resolv.conf
		nameservers = resolvconf.GetNameservers(resolv.Content)
		// slirp4netns has a built in DNS server.
		if c.config.NetMode.IsSlirp4netns() {
			nameservers = append([]string{"10.0.2.3"}, nameservers...)
		}
	}

	var search []string
	if len(c.config.DNSSearch) > 0 || len(c.runtime.config.Containers.DNSSearches) > 0 {
		if !util.StringInSlice(".", c.config.DNSSearch) {
			search = c.runtime.config.Containers.DNSSearches
			search = append(search, c.config.DNSSearch...)
		}
	} else {
		search = resolvconf.GetSearchDomains(resolv.Content)
	}

	var options []string
	if len(c.config.DNSOption) > 0 || len(c.runtime.config.Containers.DNSOptions) > 0 {
		options = c.runtime.config.Containers.DNSOptions
		options = append(options, c.config.DNSOption...)
	} else {
		options = resolvconf.GetOptions(resolv.Content)
	}

	destPath := filepath.Join(c.state.RunDir, "resolv.conf")

	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "error removing resolv.conf for container %s", c.ID())
	}

	// Build resolv.conf
	if _, err = resolvconf.Build(destPath, nameservers, search, options); err != nil {
		return "", errors.Wrapf(err, "error building resolv.conf for container %s", c.ID())
	}

	// Relabel resolv.conf for the container
	if err := label.Relabel(destPath, c.config.MountLabel, true); err != nil {
		return "", err
	}

	return filepath.Join(c.state.RunDir, "resolv.conf"), nil
}

// generateHosts creates a containers hosts file
func (c *Container) generateHosts(path string) (string, error) {
	orig, err := ioutil.ReadFile(path)
	if err != nil {
		return "", errors.Wrapf(err, "unable to read %s", path)
	}
	hosts := string(orig)
	hosts += c.getHosts()
	return c.writeStringToRundir("hosts", hosts)
}

// appendHosts appends a container's config and state pertaining to hosts to a container's
// local hosts file. netCtr is the container from which the netNS information is
// taken.
// path is the basis of the hosts file, into which netCtr's netNS information will be appended.
func (c *Container) appendHosts(path string, netCtr *Container) (string, error) {
	return c.appendStringToRundir("hosts", netCtr.getHosts())
}

// getHosts finds the pertinent information for a container's host file in its config and state
// and returns a string in a format that can be written to the host file
func (c *Container) getHosts() string {
	var hosts string
	if len(c.config.HostAdd) > 0 {
		for _, host := range c.config.HostAdd {
			// the host format has already been verified at this point
			fields := strings.SplitN(host, ":", 2)
			hosts += fmt.Sprintf("%s %s\n", fields[1], fields[0])
		}
	}
	if c.config.NetMode.IsSlirp4netns() {
		// When using slirp4netns, the interface gets a static IP
		hosts += fmt.Sprintf("# used by slirp4netns\n%s\t%s\n", "10.0.2.100", c.Hostname())
	}
	if len(c.state.NetworkStatus) > 0 && len(c.state.NetworkStatus[0].IPs) > 0 {
		ipAddress := strings.Split(c.state.NetworkStatus[0].IPs[0].Address.String(), "/")[0]
		hosts += fmt.Sprintf("%s\t%s\n", ipAddress, c.Hostname())
	}
	return hosts
}

// generatePasswd generates a container specific passwd file,
// iff g.config.User is a number
func (c *Container) generatePasswd() (string, error) {
	var (
		groupspec string
		gid       int
	)
	if c.config.User == "" {
		return "", nil
	}
	splitSpec := strings.SplitN(c.config.User, ":", 2)
	userspec := splitSpec[0]
	if len(splitSpec) > 1 {
		groupspec = splitSpec[1]
	}
	// If a non numeric User, then don't generate passwd
	uid, err := strconv.ParseUint(userspec, 10, 32)
	if err != nil {
		return "", nil
	}
	// Lookup the user to see if it exists in the container image
	_, err = lookup.GetUser(c.state.Mountpoint, userspec)
	if err != nil && err != user.ErrNoPasswdEntries {
		return "", err
	}
	if err == nil {
		return "", nil
	}
	if groupspec != "" {
		ugid, err := strconv.ParseUint(groupspec, 10, 32)
		if err == nil {
			gid = int(ugid)
		} else {
			group, err := lookup.GetGroup(c.state.Mountpoint, groupspec)
			if err != nil {
				return "", errors.Wrapf(err, "unable to get gid %s from group file", groupspec)
			}
			gid = group.Gid
		}
	}
	originPasswdFile := filepath.Join(c.state.Mountpoint, "/etc/passwd")
	orig, err := ioutil.ReadFile(originPasswdFile)
	if err != nil && !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "unable to read passwd file %s", originPasswdFile)
	}

	pwd := fmt.Sprintf("%s%d:x:%d:%d:container user:%s:/bin/sh\n", orig, uid, uid, gid, c.WorkingDir())
	passwdFile, err := c.writeStringToRundir("passwd", pwd)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create temporary passwd file")
	}
	if err := os.Chmod(passwdFile, 0644); err != nil {
		return "", err
	}
	return passwdFile, nil
}

func (c *Container) copyOwnerAndPerms(source, dest string) error {
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrapf(err, "cannot stat `%s`", dest)
	}
	if err := os.Chmod(dest, info.Mode()); err != nil {
		return errors.Wrapf(err, "cannot chmod `%s`", dest)
	}
	if err := os.Chown(dest, int(info.Sys().(*syscall.Stat_t).Uid), int(info.Sys().(*syscall.Stat_t).Gid)); err != nil {
		return errors.Wrapf(err, "cannot chown `%s`", dest)
	}
	return nil
}

// Get cgroup path in a format suitable for the OCI spec
func (c *Container) getOCICgroupPath() (string, error) {
	unified, err := cgroups.IsCgroup2UnifiedMode()
	if err != nil {
		return "", err
	}
	switch {
	case (rootless.IsRootless() && !unified) || c.config.NoCgroups:
		return "", nil
	case c.runtime.config.Engine.CgroupManager == config.SystemdCgroupsManager:
		// When runc is set to use Systemd as a cgroup manager, it
		// expects cgroups to be passed as follows:
		// slice:prefix:name
		systemdCgroups := fmt.Sprintf("%s:libpod:%s", path.Base(c.config.CgroupParent), c.ID())
		logrus.Debugf("Setting CGroups for container %s to %s", c.ID(), systemdCgroups)
		return systemdCgroups, nil
	case c.runtime.config.Engine.CgroupManager == config.CgroupfsCgroupsManager:
		cgroupPath, err := c.CGroupPath()
		if err != nil {
			return "", err
		}
		logrus.Debugf("Setting CGroup path for container %s to %s", c.ID(), cgroupPath)
		return cgroupPath, nil
	default:
		return "", errors.Wrapf(define.ErrInvalidArg, "invalid cgroup manager %s requested", c.runtime.config.Engine.CgroupManager)
	}
}
