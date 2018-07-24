// +build linux

package libpod

import (
	"context"
	"fmt"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/containers/storage/pkg/idtools"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	crioAnnotations "github.com/projectatomic/libpod/pkg/annotations"
	"github.com/projectatomic/libpod/pkg/chrootuser"
	"github.com/projectatomic/libpod/pkg/rootless"
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
	if err := unix.Unmount(mount, unix.MNT_DETACH); err != nil {
		if err != syscall.EINVAL {
			logrus.Warnf("container %s failed to unmount %s : %v", c.ID(), mount, err)
		}
	}
	return nil
}

// prepare mounts the container and sets up other required resources like net
// namespaces
func (c *Container) prepare() (err error) {
	// Mount storage if not mounted
	if err := c.mountStorage(); err != nil {
		return err
	}

	// Set up network namespace if not already set up
	if c.config.CreateNetNS && c.state.NetNS == nil && !c.config.PostConfigureNetNS {
		if err := c.runtime.createNetNS(c); err != nil {
			// Tear down storage before exiting to make sure we
			// don't leak mounts
			if err2 := c.cleanupStorage(); err2 != nil {
				logrus.Errorf("Error cleaning up storage for container %s: %v", c.ID(), err2)
			}
			return err
		}
	}

	return nil
}

// cleanupNetwork unmounts and cleans up the container's network
func (c *Container) cleanupNetwork() error {
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

// Generate spec for a container
// Accepts a map of the container's dependencies
func (c *Container) generateSpec(ctx context.Context) (*spec.Spec, error) {
	g := generate.NewFromSpec(c.config.Spec)

	// If network namespace was requested, add it now
	if c.config.CreateNetNS {
		if c.config.PostConfigureNetNS {
			g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, "")
		} else {
			g.AddOrReplaceLinuxNamespace(spec.NetworkNamespace, c.state.NetNS.Path())
		}
	}

	// Remove the default /dev/shm mount to ensure we overwrite it
	g.RemoveMount("/dev/shm")

	// Add bind mounts to container
	for dstPath, srcPath := range c.state.BindMounts {
		newMount := spec.Mount{
			Type:        "bind",
			Source:      srcPath,
			Destination: dstPath,
			Options:     []string{"rw", "bind"},
		}
		if !MountExists(g.Mounts(), dstPath) {
			g.AddMount(newMount)
		} else {
			logrus.Warnf("User mount overriding libpod mount at %q", dstPath)
		}
	}

	var err error
	if !rootless.IsRootless() {
		if c.state.ExtensionStageHooks, err = c.setupOCIHooks(ctx, g.Config); err != nil {
			return nil, errors.Wrapf(err, "error setting up OCI Hooks")
		}
	}

	// Bind builtin image volumes
	if c.config.Rootfs == "" && c.config.ImageVolumes {
		if err := c.addLocalVolumes(ctx, &g); err != nil {
			return nil, errors.Wrapf(err, "error mounting image volumes")
		}
	}

	if c.config.User != "" {
		if !c.state.Mounted {
			return nil, errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to translate User field", c.ID())
		}
		uid, gid, err := chrootuser.GetUser(c.state.Mountpoint, c.config.User)
		if err != nil {
			return nil, err
		}
		// User and Group must go together
		g.SetProcessUID(uid)
		g.SetProcessGID(gid)
	}

	// Add addition groups if c.config.GroupAdd is not empty
	if len(c.config.Groups) > 0 {
		if !c.state.Mounted {
			return nil, errors.Wrapf(ErrCtrStateInvalid, "container %s must be mounted in order to add additional groups", c.ID())
		}
		for _, group := range c.config.Groups {
			gid, err := chrootuser.GetGroup(c.state.Mountpoint, group)
			if err != nil {
				return nil, err
			}
			g.AddProcessAdditionalGid(gid)
		}
	}

	// Look up and add groups the user belongs to, if a group wasn't directly specified
	if !rootless.IsRootless() && !strings.Contains(c.config.User, ":") {
		groups, err := chrootuser.GetAdditionalGroupsForUser(c.state.Mountpoint, uint64(g.Config.Process.User.UID))
		if err != nil && errors.Cause(err) != chrootuser.ErrNoSuchUser {
			return nil, err
		}
		for _, gid := range groups {
			g.AddProcessAdditionalGid(gid)
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
	if c.config.NetNs != "" {
		if err := c.addNamespacePath(&g, NetNS, c.config.NetNs, spec.NetworkNamespace); err != nil {
			return nil, err
		}
	}
	if c.config.PIDNsCtr != "" {
		if err := c.addNamespaceContainer(&g, PIDNS, c.config.PIDNsCtr, string(spec.PIDNamespace)); err != nil {
			return nil, err
		}
	}
	if c.config.UserNsCtr != "" {
		if err := c.addNamespaceContainer(&g, UserNS, c.config.UserNsCtr, spec.UserNamespace); err != nil {
			return nil, err
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

	if c.config.Rootfs == "" {
		if err := idtools.MkdirAllAs(c.state.RealMountpoint, 0700, c.RootUID(), c.RootGID()); err != nil {
			return nil, err
		}
	}

	g.SetRootPath(c.state.RealMountpoint)
	g.AddAnnotation(crioAnnotations.Created, c.config.CreatedTime.Format(time.RFC3339Nano))
	g.AddAnnotation("org.opencontainers.image.stopSignal", fmt.Sprintf("%d", c.config.StopSignal))

	g.SetHostname(c.Hostname())
	g.AddProcessEnv("HOSTNAME", g.Config.Hostname)

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

	if rootless.IsRootless() {
		g.SetLinuxCgroupsPath("")
	} else if c.runtime.config.CgroupManager == SystemdCgroupsManager {
		// When runc is set to use Systemd as a cgroup manager, it
		// expects cgroups to be passed as follows:
		// slice:prefix:name
		systemdCgroups := fmt.Sprintf("%s:libpod:%s", path.Base(c.config.CgroupParent), c.ID())
		logrus.Debugf("Setting CGroups for container %s to %s", c.ID(), systemdCgroups)
		g.SetLinuxCgroupsPath(systemdCgroups)
	} else {
		cgroupPath, err := c.CGroupPath()
		if err != nil {
			return nil, err
		}
		logrus.Debugf("Setting CGroup path for container %s to %s", c.ID(), cgroupPath)
		g.SetLinuxCgroupsPath(cgroupPath)
	}

	return g.Config, nil
}

// Add an existing container's namespace to the spec
func (c *Container) addNamespaceContainer(g *generate.Generator, ns LinuxNS, ctr string, specNS string) error {
	nsCtr, err := c.runtime.state.Container(ctr)
	if err != nil {
		return errors.Wrapf(err, "error retrieving dependency %s of container %s from state", ctr, c.ID())
	}

	// TODO need unlocked version of this for use in pods
	nsPath, err := nsCtr.NamespacePath(ns)
	if err != nil {
		return err
	}

	if err := g.AddOrReplaceLinuxNamespace(specNS, nsPath); err != nil {
		return err
	}

	return nil
}

// Add an existing namespace to the spec
func (c *Container) addNamespacePath(g *generate.Generator, ns LinuxNS, nsPath string, specNS string) error {
	if err := g.AddOrReplaceLinuxNamespace(specNS, nsPath); err != nil {
		return err
	}

	return nil
}
