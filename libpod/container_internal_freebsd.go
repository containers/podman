//go:build freebsd
// +build freebsd

package libpod

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/containers/buildah/pkg/overlay"
	"github.com/containers/common/libnetwork/types"
	"github.com/containers/common/pkg/chown"
	"github.com/containers/common/pkg/umask"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/storage/pkg/idtools"
	securejoin "github.com/cyphar/filepath-securejoin"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	bindOptions = []string{}
)

// Network stubs to decouple container_internal_freebsd.go from
// networking_freebsd.go so they can be reviewed separately.
func (r *Runtime) createNetNS(ctr *Container) (netJail string, q map[string]types.StatusBlock, retErr error) {
	return "", nil, errors.New("not implemented (*Runtime) createNetNS")
}

func (r *Runtime) teardownNetNS(ctr *Container) error {
	return errors.New("not implemented (*Runtime) teardownNetNS")
}

func (r *Runtime) reloadContainerNetwork(ctr *Container) (map[string]types.StatusBlock, error) {
	return nil, errors.New("not implemented (*Runtime) reloadContainerNetwork")
}

func (c *Container) mountSHM(shmOptions string) error {
	return nil
}

func (c *Container) unmountSHM(path string) error {
	return nil
}

// prepare mounts the container and sets up other required resources like net
// namespaces
func (c *Container) prepare() error {
	var (
		wg                              sync.WaitGroup
		jailName                        string
		networkStatus                   map[string]types.StatusBlock
		createNetNSErr, mountStorageErr error
		mountPoint                      string
		tmpStateLock                    sync.Mutex
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		// Set up network namespace if not already set up
		noNetNS := c.state.NetworkJail == ""
		if c.config.CreateNetNS && noNetNS && !c.config.PostConfigureNetNS {
			jailName, networkStatus, createNetNSErr = c.runtime.createNetNS(c)
			if createNetNSErr != nil {
				return
			}

			tmpStateLock.Lock()
			defer tmpStateLock.Unlock()

			// Assign NetNS attributes to container
			c.state.NetworkJail = jailName
			c.state.NetworkStatus = networkStatus
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
	if mountStorageErr != nil {
		if createErr != nil {
			logrus.Errorf("Preparing container %s: %v", c.ID(), createErr)
		}
		createErr = mountStorageErr
	}

	if createErr != nil {
		return createErr
	}

	// Save changes to container state
	if err := c.save(); err != nil {
		return err
	}

	return nil
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

	// Stop the container's network namespace (if it has one)
	if err := c.runtime.teardownNetNS(c); err != nil {
		logrus.Errorf("Unable to cleanup network for container %s: %q", c.ID(), err)
	}

	if c.valid {
		return c.save()
	}

	return nil
}

// reloadNetwork reloads the network for the given container, recreating
// firewall rules.
func (c *Container) reloadNetwork() error {
	result, err := c.runtime.reloadContainerNetwork(c)
	if err != nil {
		return err
	}

	c.state.NetworkStatus = result

	return c.save()
}

// Add an existing container's network jail
func (c *Container) addNetworkContainer(g *generate.Generator, ctr string) error {
	nsCtr, err := c.runtime.state.Container(ctr)
	c.runtime.state.UpdateContainer(nsCtr)
	if err != nil {
		return fmt.Errorf("error retrieving dependency %s of container %s from state: %w", ctr, c.ID(), err)
	}
	g.AddAnnotation("org.freebsd.parentJail", nsCtr.state.NetworkJail)
	return nil
}

func isRootlessCgroupSet(cgroup string) bool {
	return false
}

func (c *Container) expectPodCgroup() (bool, error) {
	return false, nil
}

func (c *Container) getOCICgroupPath() (string, error) {
	return "", nil
}

func (c *Container) copyTimezoneFile(zonePath string) (string, error) {
	var localtimeCopy string = filepath.Join(c.state.RunDir, "localtime")
	file, err := os.Stat(zonePath)
	if err != nil {
		return "", err
	}
	if file.IsDir() {
		return "", errors.New("Invalid timezone: is a directory")
	}
	src, err := os.Open(zonePath)
	if err != nil {
		return "", err
	}
	defer src.Close()
	dest, err := os.Create(localtimeCopy)
	if err != nil {
		return "", err
	}
	defer dest.Close()
	_, err = io.Copy(dest, src)
	if err != nil {
		return "", err
	}
	if err := c.relabel(localtimeCopy, c.config.MountLabel, false); err != nil {
		return "", err
	}
	if err := dest.Chown(c.RootUID(), c.RootGID()); err != nil {
		return "", err
	}
	return localtimeCopy, err
}

func (c *Container) cleanupOverlayMounts() error {
	return overlay.CleanupContent(c.config.StaticDir)
}

// Check if a file exists at the given path in the container's root filesystem.
// Container must already be mounted for this to be used.
func (c *Container) checkFileExistsInRootfs(file string) (bool, error) {
	checkPath, err := securejoin.SecureJoin(c.state.Mountpoint, file)
	if err != nil {
		return false, fmt.Errorf("cannot create path to container %s file %q: %w", c.ID(), file, err)
	}
	stat, err := os.Stat(checkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("container %s: %w", c.ID(), err)
	}
	if stat.IsDir() {
		return false, nil
	}
	return true, nil
}

// Creates and mounts an empty dir to mount secrets into, if it does not already exist
func (c *Container) createSecretMountDir() error {
	src := filepath.Join(c.state.RunDir, "/run/secrets")
	_, err := os.Stat(src)
	if os.IsNotExist(err) {
		oldUmask := umask.Set(0)
		defer umask.Set(oldUmask)

		if err := os.MkdirAll(src, 0755); err != nil {
			return err
		}
		if err := label.Relabel(src, c.config.MountLabel, false); err != nil {
			return err
		}
		if err := os.Chown(src, c.RootUID(), c.RootGID()); err != nil {
			return err
		}
		c.state.BindMounts["/run/secrets"] = src
		return nil
	}

	return err
}

// Fix ownership and permissions of the specified volume if necessary.
func (c *Container) fixVolumePermissions(v *ContainerNamedVolume) error {
	vol, err := c.runtime.state.Volume(v.Name)
	if err != nil {
		return fmt.Errorf("error retrieving named volume %s for container %s: %w", v.Name, c.ID(), err)
	}

	vol.lock.Lock()
	defer vol.lock.Unlock()

	// The volume may need a copy-up. Check the state.
	if err := vol.update(); err != nil {
		return err
	}

	// TODO: For now, I've disabled chowning volumes owned by non-Podman
	// drivers. This may be safe, but it's really going to be a case-by-case
	// thing, I think - safest to leave disabled now and re-enable later if
	// there is a demand.
	if vol.state.NeedsChown && !vol.UsesVolumeDriver() {
		vol.state.NeedsChown = false

		uid := int(c.config.Spec.Process.User.UID)
		gid := int(c.config.Spec.Process.User.GID)

		if c.config.IDMappings.UIDMap != nil {
			p := idtools.IDPair{
				UID: uid,
				GID: gid,
			}
			mappings := idtools.NewIDMappingsFromMaps(c.config.IDMappings.UIDMap, c.config.IDMappings.GIDMap)
			newPair, err := mappings.ToHost(p)
			if err != nil {
				return fmt.Errorf("error mapping user %d:%d: %w", uid, gid, err)
			}
			uid = newPair.UID
			gid = newPair.GID
		}

		vol.state.UIDChowned = uid
		vol.state.GIDChowned = gid

		if err := vol.save(); err != nil {
			return err
		}

		mountPoint, err := vol.MountPoint()
		if err != nil {
			return err
		}

		if err := os.Lchown(mountPoint, uid, gid); err != nil {
			return err
		}

		// Make sure the new volume matches the permissions of the target directory.
		// https://github.com/containers/podman/issues/10188
		st, err := os.Lstat(filepath.Join(c.state.Mountpoint, v.Dest))
		if err == nil {
			if stat, ok := st.Sys().(*syscall.Stat_t); ok {
				if err := os.Lchown(mountPoint, int(stat.Uid), int(stat.Gid)); err != nil {
					return err
				}
			}
			if err := os.Chmod(mountPoint, st.Mode()); err != nil {
				return err
			}
			/*
				stat := st.Sys().(*syscall.Stat_t)
				atime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec))
				if err := os.Chtimes(mountPoint, atime, st.ModTime()); err != nil {
					return err
				}*/
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (c *Container) relabel(src, mountLabel string, recurse bool) error {
	if !selinux.GetEnabled() || mountLabel == "" {
		return nil
	}
	// only relabel on initial creation of container
	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateUnknown) {
		label, err := label.FileLabel(src)
		if err != nil {
			return err
		}
		// If labels are different, might be on a tmpfs
		if label == mountLabel {
			return nil
		}
	}
	return label.Relabel(src, mountLabel, recurse)
}

func (c *Container) ChangeHostPathOwnership(src string, recurse bool, uid, gid int) error {
	// only chown on initial creation of container
	if !c.ensureState(define.ContainerStateConfigured, define.ContainerStateUnknown) {
		st, err := os.Stat(src)
		if err != nil {
			return err
		}

		// If labels are different, might be on a tmpfs
		if int(st.Sys().(*syscall.Stat_t).Uid) == uid && int(st.Sys().(*syscall.Stat_t).Gid) == gid {
			return nil
		}
	}
	return chown.ChangeHostPathOwnership(src, recurse, uid, gid)
}

func openDirectory(path string) (fd int, err error) {
	const O_PATH = 0x00400000
	return unix.Open(path, unix.O_RDONLY|O_PATH, 0)
}

func (c *Container) addNetworkNamespace(g *generate.Generator) error {
	if c.config.CreateNetNS {
		g.AddAnnotation("org.freebsd.parentJail", c.state.NetworkJail)
	}
	return nil
}

func (c *Container) addSystemdMounts(g *generate.Generator) error {
	return nil
}

func (c *Container) addSharedNamespaces(g *generate.Generator) error {
	if c.config.NetNsCtr != "" {
		if err := c.addNetworkContainer(g, c.config.NetNsCtr); err != nil {
			return err
		}
	}

	availableUIDs, availableGIDs, err := rootless.GetAvailableIDMaps()
	if err != nil {
		if os.IsNotExist(err) {
			// The kernel-provided files only exist if user namespaces are supported
			logrus.Debugf("User or group ID mappings not available: %s", err)
		} else {
			return err
		}
	} else {
		g.Config.Linux.UIDMappings = rootless.MaybeSplitMappings(g.Config.Linux.UIDMappings, availableUIDs)
		g.Config.Linux.GIDMappings = rootless.MaybeSplitMappings(g.Config.Linux.GIDMappings, availableGIDs)
	}

	// Hostname handling:
	// If we have a UTS namespace, set Hostname in the OCI spec.
	// Set the HOSTNAME environment variable unless explicitly overridden by
	// the user (already present in OCI spec). If we don't have a UTS ns,
	// set it to the host's hostname instead.
	hostname := c.Hostname()
	foundUTS := false

	// TODO: make this optional, needs progress on adding FreeBSD section to the spec
	foundUTS = true
	g.SetHostname(hostname)

	if !foundUTS {
		tmpHostname, err := os.Hostname()
		if err != nil {
			return err
		}
		hostname = tmpHostname
	}
	needEnv := true
	for _, checkEnv := range g.Config.Process.Env {
		if strings.SplitN(checkEnv, "=", 2)[0] == "HOSTNAME" {
			needEnv = false
			break
		}
	}
	if needEnv {
		g.AddProcessEnv("HOSTNAME", hostname)
	}
	return nil
}

func (c *Container) addRootPropagation(g *generate.Generator, mounts []spec.Mount) error {
	return nil
}

func (c *Container) setProcessLabel(g *generate.Generator) {
}

func (c *Container) setMountLabel(g *generate.Generator) {
}

func (c *Container) setCgroupsPath(g *generate.Generator) error {
	return nil
}
