//go:build linux || freebsd
// +build linux freebsd

package libpod

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cdi "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	"github.com/containers/buildah/pkg/chrootuser"
	"github.com/containers/buildah/pkg/overlay"
	butil "github.com/containers/buildah/util"
	"github.com/containers/common/pkg/apparmor"
	cutil "github.com/containers/common/pkg/util"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/annotations"
	"github.com/containers/podman/v4/pkg/lookup"
	"github.com/containers/podman/v4/pkg/rootless"
	"github.com/containers/podman/v4/pkg/util"
	"github.com/containers/storage/pkg/idtools"
	securejoin "github.com/cyphar/filepath-securejoin"
	runcuser "github.com/opencontainers/runc/libcontainer/user"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
)

// Internal only function which returns upper and work dir from
// overlay options.
func getOverlayUpperAndWorkDir(options []string) (string, string, error) {
	upperDir := ""
	workDir := ""
	for _, o := range options {
		if strings.HasPrefix(o, "upperdir") {
			splitOpt := strings.SplitN(o, "=", 2)
			if len(splitOpt) > 1 {
				upperDir = splitOpt[1]
				if upperDir == "" {
					return "", "", errors.New("cannot accept empty value for upperdir")
				}
			}
		}
		if strings.HasPrefix(o, "workdir") {
			splitOpt := strings.SplitN(o, "=", 2)
			if len(splitOpt) > 1 {
				workDir = splitOpt[1]
				if workDir == "" {
					return "", "", errors.New("cannot accept empty value for workdir")
				}
			}
		}
	}
	if (upperDir != "" && workDir == "") || (upperDir == "" && workDir != "") {
		return "", "", errors.New("must specify both upperdir and workdir")
	}
	return upperDir, workDir, nil
}

// Generate spec for a container
// Accepts a map of the container's dependencies
func (c *Container) generateSpec(ctx context.Context) (*spec.Spec, error) {
	overrides := c.getUserOverrides()
	execUser, err := lookup.GetUserGroupInfo(c.state.Mountpoint, c.config.User, overrides)
	if err != nil {
		if cutil.StringInSlice(c.config.User, c.config.HostUsers) {
			execUser, err = lookupHostUser(c.config.User)
		}
		if err != nil {
			return nil, err
		}
	}

	// NewFromSpec() is deprecated according to its comment
	// however the recommended replace just causes a nil map panic
	//nolint:staticcheck
	g := generate.NewFromSpec(c.config.Spec)

	// If the flag to mount all devices is set for a privileged container, add
	// all the devices from the host's machine into the container
	if c.config.MountAllDevices {
		if err := util.AddPrivilegedDevices(&g); err != nil {
			return nil, err
		}
	}

	// If network namespace was requested, add it now
	if err := c.addNetworkNamespace(&g); err != nil {
		return nil, err
	}

	// Apply AppArmor checks and load the default profile if needed.
	if len(c.config.Spec.Process.ApparmorProfile) > 0 {
		updatedProfile, err := apparmor.CheckProfileAndLoadDefault(c.config.Spec.Process.ApparmorProfile)
		if err != nil {
			return nil, err
		}
		g.SetProcessApparmorProfile(updatedProfile)
	}

	if err := c.makeBindMounts(); err != nil {
		return nil, err
	}

	if err := c.mountNotifySocket(g); err != nil {
		return nil, err
	}

	// Get host UID and GID based on the container process UID and GID.
	hostUID, hostGID, err := butil.GetHostIDs(util.IDtoolsToRuntimeSpec(c.config.IDMappings.UIDMap), util.IDtoolsToRuntimeSpec(c.config.IDMappings.GIDMap), uint32(execUser.Uid), uint32(execUser.Gid))
	if err != nil {
		return nil, err
	}

	// Add named volumes
	for _, namedVol := range c.config.NamedVolumes {
		volume, err := c.runtime.GetVolume(namedVol.Name)
		if err != nil {
			return nil, fmt.Errorf("error retrieving volume %s to add to container %s: %w", namedVol.Name, c.ID(), err)
		}
		mountPoint, err := volume.MountPoint()
		if err != nil {
			return nil, err
		}

		overlayFlag := false
		upperDir := ""
		workDir := ""
		for _, o := range namedVol.Options {
			if o == "O" {
				overlayFlag = true
				upperDir, workDir, err = getOverlayUpperAndWorkDir(namedVol.Options)
				if err != nil {
					return nil, err
				}
			}
		}

		if overlayFlag {
			var overlayMount spec.Mount
			var overlayOpts *overlay.Options
			contentDir, err := overlay.TempDir(c.config.StaticDir, c.RootUID(), c.RootGID())
			if err != nil {
				return nil, err
			}

			overlayOpts = &overlay.Options{RootUID: c.RootUID(),
				RootGID:                c.RootGID(),
				UpperDirOptionFragment: upperDir,
				WorkDirOptionFragment:  workDir,
				GraphOpts:              c.runtime.store.GraphOptions(),
			}

			overlayMount, err = overlay.MountWithOptions(contentDir, mountPoint, namedVol.Dest, overlayOpts)
			if err != nil {
				return nil, fmt.Errorf("mounting overlay failed %q: %w", mountPoint, err)
			}

			for _, o := range namedVol.Options {
				if o == "U" {
					if err := c.ChangeHostPathOwnership(mountPoint, true, int(hostUID), int(hostGID)); err != nil {
						return nil, err
					}

					if err := c.ChangeHostPathOwnership(contentDir, true, int(hostUID), int(hostGID)); err != nil {
						return nil, err
					}
				}
			}
			g.AddMount(overlayMount)
		} else {
			volMount := spec.Mount{
				Type:        define.TypeBind,
				Source:      mountPoint,
				Destination: namedVol.Dest,
				Options:     namedVol.Options,
			}
			g.AddMount(volMount)
		}
	}

	// Check if the spec file mounts contain the options z, Z or U.
	// If they have z or Z, relabel the source directory and then remove the option.
	// If they have U, chown the source directory and them remove the option.
	for i := range g.Config.Mounts {
		m := &g.Config.Mounts[i]
		var options []string
		for _, o := range m.Options {
			switch o {
			case "U":
				if m.Type == "tmpfs" {
					options = append(options, []string{fmt.Sprintf("uid=%d", execUser.Uid), fmt.Sprintf("gid=%d", execUser.Gid)}...)
				} else {
					// only chown on initial creation of container
					if err := c.ChangeHostPathOwnership(m.Source, true, int(hostUID), int(hostGID)); err != nil {
						return nil, err
					}
				}
			case "z":
				fallthrough
			case "Z":
				if err := c.relabel(m.Source, c.MountLabel(), label.IsShared(o)); err != nil {
					return nil, err
				}

			default:
				options = append(options, o)
			}
		}
		m.Options = options
	}

	c.setProcessLabel(&g)
	c.setMountLabel(&g)

	// Add bind mounts to container
	for dstPath, srcPath := range c.state.BindMounts {
		newMount := spec.Mount{
			Type:        define.TypeBind,
			Source:      srcPath,
			Destination: dstPath,
			Options:     bindOptions,
		}
		if c.IsReadOnly() && dstPath != "/dev/shm" {
			newMount.Options = append(newMount.Options, "ro", "nosuid", "noexec", "nodev")
		}
		if dstPath == "/dev/shm" && c.state.BindMounts["/dev/shm"] == c.config.ShmDir {
			newMount.Options = append(newMount.Options, "nosuid", "noexec", "nodev")
		}
		if !MountExists(g.Mounts(), dstPath) {
			g.AddMount(newMount)
		} else {
			logrus.Infof("User mount overriding libpod mount at %q", dstPath)
		}
	}

	// Add overlay volumes
	for _, overlayVol := range c.config.OverlayVolumes {
		upperDir, workDir, err := getOverlayUpperAndWorkDir(overlayVol.Options)
		if err != nil {
			return nil, err
		}
		contentDir, err := overlay.TempDir(c.config.StaticDir, c.RootUID(), c.RootGID())
		if err != nil {
			return nil, err
		}
		overlayOpts := &overlay.Options{RootUID: c.RootUID(),
			RootGID:                c.RootGID(),
			UpperDirOptionFragment: upperDir,
			WorkDirOptionFragment:  workDir,
			GraphOpts:              c.runtime.store.GraphOptions(),
		}

		overlayMount, err := overlay.MountWithOptions(contentDir, overlayVol.Source, overlayVol.Dest, overlayOpts)
		if err != nil {
			return nil, fmt.Errorf("mounting overlay failed %q: %w", overlayVol.Source, err)
		}

		// Check overlay volume options
		for _, o := range overlayVol.Options {
			if o == "U" {
				if err := c.ChangeHostPathOwnership(overlayVol.Source, true, int(hostUID), int(hostGID)); err != nil {
					return nil, err
				}

				if err := c.ChangeHostPathOwnership(contentDir, true, int(hostUID), int(hostGID)); err != nil {
					return nil, err
				}
			}
		}

		g.AddMount(overlayMount)
	}

	// Add image volumes as overlay mounts
	for _, volume := range c.config.ImageVolumes {
		// Mount the specified image.
		img, _, err := c.runtime.LibimageRuntime().LookupImage(volume.Source, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating image volume %q:%q: %w", volume.Source, volume.Dest, err)
		}
		mountPoint, err := img.Mount(ctx, nil, "")
		if err != nil {
			return nil, fmt.Errorf("error mounting image volume %q:%q: %w", volume.Source, volume.Dest, err)
		}

		contentDir, err := overlay.TempDir(c.config.StaticDir, c.RootUID(), c.RootGID())
		if err != nil {
			return nil, fmt.Errorf("failed to create TempDir in the %s directory: %w", c.config.StaticDir, err)
		}

		var overlayMount spec.Mount
		if volume.ReadWrite {
			overlayMount, err = overlay.Mount(contentDir, mountPoint, volume.Dest, c.RootUID(), c.RootGID(), c.runtime.store.GraphOptions())
		} else {
			overlayMount, err = overlay.MountReadOnly(contentDir, mountPoint, volume.Dest, c.RootUID(), c.RootGID(), c.runtime.store.GraphOptions())
		}
		if err != nil {
			return nil, fmt.Errorf("creating overlay mount for image %q failed: %w", volume.Source, err)
		}
		g.AddMount(overlayMount)
	}

	hasHomeSet := false
	for _, s := range c.config.Spec.Process.Env {
		if strings.HasPrefix(s, "HOME=") {
			hasHomeSet = true
			break
		}
	}
	if !hasHomeSet && execUser.Home != "" {
		c.config.Spec.Process.Env = append(c.config.Spec.Process.Env, fmt.Sprintf("HOME=%s", execUser.Home))
	}

	if c.config.User != "" {
		// User and Group must go together
		g.SetProcessUID(uint32(execUser.Uid))
		g.SetProcessGID(uint32(execUser.Gid))
		g.AddProcessAdditionalGid(uint32(execUser.Gid))
	}

	if c.config.Umask != "" {
		decVal, err := strconv.ParseUint(c.config.Umask, 8, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid Umask Value: %w", err)
		}
		umask := uint32(decVal)
		g.Config.Process.User.Umask = &umask
	}

	// Add addition groups if c.config.GroupAdd is not empty
	if len(c.config.Groups) > 0 {
		gids, err := lookup.GetContainerGroups(c.config.Groups, c.state.Mountpoint, overrides)
		if err != nil {
			return nil, fmt.Errorf("error looking up supplemental groups for container %s: %w", c.ID(), err)
		}
		for _, gid := range gids {
			g.AddProcessAdditionalGid(gid)
		}
	}

	if err := c.addSystemdMounts(&g); err != nil {
		return nil, err
	}

	// Look up and add groups the user belongs to, if a group wasn't directly specified
	if !strings.Contains(c.config.User, ":") {
		// the gidMappings that are present inside the container user namespace
		var gidMappings []idtools.IDMap

		switch {
		case len(c.config.IDMappings.GIDMap) > 0:
			gidMappings = c.config.IDMappings.GIDMap
		case rootless.IsRootless():
			// Check whether the current user namespace has enough gids available.
			availableGids, err := rootless.GetAvailableGids()
			if err != nil {
				return nil, fmt.Errorf("cannot read number of available GIDs: %w", err)
			}
			gidMappings = []idtools.IDMap{{
				ContainerID: 0,
				HostID:      0,
				Size:        int(availableGids),
			}}
		default:
			gidMappings = []idtools.IDMap{{
				ContainerID: 0,
				HostID:      0,
				Size:        math.MaxInt32,
			}}
		}
		for _, gid := range execUser.Sgids {
			isGIDAvailable := false
			for _, m := range gidMappings {
				if gid >= m.ContainerID && gid < m.ContainerID+m.Size {
					isGIDAvailable = true
					break
				}
			}
			if isGIDAvailable {
				g.AddProcessAdditionalGid(uint32(gid))
			} else {
				logrus.Warnf("Additional gid=%d is not present in the user namespace, skip setting it", gid)
			}
		}
	}

	// Add shared namespaces from other containers
	if err := c.addSharedNamespaces(&g); err != nil {
		return nil, err
	}

	g.SetRootPath(c.state.Mountpoint)
	g.AddAnnotation(annotations.Created, c.config.CreatedTime.Format(time.RFC3339Nano))
	g.AddAnnotation("org.opencontainers.image.stopSignal", fmt.Sprintf("%d", c.config.StopSignal))

	if _, exists := g.Config.Annotations[annotations.ContainerManager]; !exists {
		g.AddAnnotation(annotations.ContainerManager, annotations.ContainerManagerLibpod)
	}

	if err := c.setCgroupsPath(&g); err != nil {
		return nil, err
	}

	// Warning: CDI may alter g.Config in place.
	if len(c.config.CDIDevices) > 0 {
		registry := cdi.GetRegistry(
			cdi.WithAutoRefresh(false),
		)
		if err := registry.Refresh(); err != nil {
			logrus.Debugf("The following error was triggered when refreshing the CDI registry: %v", err)
		}
		_, err := registry.InjectDevices(g.Config, c.config.CDIDevices...)
		if err != nil {
			return nil, fmt.Errorf("error setting up CDI devices: %w", err)
		}
	}

	// Mounts need to be sorted so paths will not cover other paths
	mounts := sortMounts(g.Mounts())
	g.ClearMounts()

	for _, m := range mounts {
		// We need to remove all symlinks from tmpfs mounts.
		// Runc and other runtimes may choke on them.
		// Easy solution: use securejoin to do a scoped evaluation of
		// the links, then trim off the mount prefix.
		if m.Type == "tmpfs" {
			finalPath, err := securejoin.SecureJoin(c.state.Mountpoint, m.Destination)
			if err != nil {
				return nil, fmt.Errorf("error resolving symlinks for mount destination %s: %w", m.Destination, err)
			}
			trimmedPath := strings.TrimPrefix(finalPath, strings.TrimSuffix(c.state.Mountpoint, "/"))
			m.Destination = trimmedPath
		}
		g.AddMount(m)
	}

	if err := c.addRootPropagation(&g, mounts); err != nil {
		return nil, err
	}

	// Warning: precreate hooks may alter g.Config in place.
	if c.state.ExtensionStageHooks, err = c.setupOCIHooks(ctx, g.Config); err != nil {
		return nil, fmt.Errorf("error setting up OCI Hooks: %w", err)
	}
	if len(c.config.EnvSecrets) > 0 {
		manager, err := c.runtime.SecretsManager()
		if err != nil {
			return nil, err
		}
		if err != nil {
			return nil, err
		}
		for name, secr := range c.config.EnvSecrets {
			_, data, err := manager.LookupSecretData(secr.Name)
			if err != nil {
				return nil, err
			}
			g.AddProcessEnv(name, string(data))
		}
	}

	// Pass down the LISTEN_* environment (see #10443).
	for _, key := range []string{"LISTEN_PID", "LISTEN_FDS", "LISTEN_FDNAMES"} {
		if val, ok := os.LookupEnv(key); ok {
			// Force the PID to `1` since we cannot rely on (all
			// versions of) all runtimes to do it for us.
			if key == "LISTEN_PID" {
				val = "1"
			}
			g.AddProcessEnv(key, val)
		}
	}

	return g.Config, nil
}

// isWorkDirSymlink returns true if resolved workdir is symlink or a chain of symlinks,
// and final resolved target is present either on  volume, mount or inside of container
// otherwise it returns false. Following function is meant for internal use only and
// can change at any point of time.
func (c *Container) isWorkDirSymlink(resolvedPath string) bool {
	// We cannot create workdir since explicit --workdir is
	// set in config but workdir could also be a symlink.
	// If it's a symlink, check if the resolved target is present in the container.
	// If so, that's a valid use case: return nil.

	maxSymLinks := 0
	for {
		// Linux only supports a chain of 40 links.
		// Reference: https://github.com/torvalds/linux/blob/master/include/linux/namei.h#L13
		if maxSymLinks > 40 {
			break
		}
		resolvedSymlink, err := os.Readlink(resolvedPath)
		if err != nil {
			// End sym-link resolution loop.
			break
		}
		if resolvedSymlink != "" {
			_, resolvedSymlinkWorkdir, err := c.resolvePath(c.state.Mountpoint, resolvedSymlink)
			if isPathOnVolume(c, resolvedSymlinkWorkdir) || isPathOnBindMount(c, resolvedSymlinkWorkdir) {
				// Resolved symlink exists on external volume or mount
				return true
			}
			if err != nil {
				// Could not resolve path so end sym-link resolution loop.
				break
			}
			if resolvedSymlinkWorkdir != "" {
				resolvedPath = resolvedSymlinkWorkdir
				_, err := os.Stat(resolvedSymlinkWorkdir)
				if err == nil {
					// Symlink resolved successfully and resolved path exists on container,
					// this is a valid use-case so return nil.
					logrus.Debugf("Workdir is a symlink with target to %q and resolved symlink exists on container", resolvedSymlink)
					return true
				}
			}
		}
		maxSymLinks++
	}
	return false
}

// resolveWorkDir resolves the container's workdir and, depending on the
// configuration, will create it, or error out if it does not exist.
// Note that the container must be mounted before.
func (c *Container) resolveWorkDir() error {
	workdir := c.WorkingDir()

	// If the specified workdir is a subdir of a volume or mount,
	// we don't need to do anything.  The runtime is taking care of
	// that.
	if isPathOnVolume(c, workdir) || isPathOnBindMount(c, workdir) {
		logrus.Debugf("Workdir %q resolved to a volume or mount", workdir)
		return nil
	}

	_, resolvedWorkdir, err := c.resolvePath(c.state.Mountpoint, workdir)
	if err != nil {
		return err
	}
	logrus.Debugf("Workdir %q resolved to host path %q", workdir, resolvedWorkdir)

	st, err := os.Stat(resolvedWorkdir)
	if err == nil {
		if !st.IsDir() {
			return fmt.Errorf("workdir %q exists on container %s, but is not a directory", workdir, c.ID())
		}
		return nil
	}
	if !c.config.CreateWorkingDir {
		// No need to create it (e.g., `--workdir=/foo`), so let's make sure
		// the path exists on the container.
		if err != nil {
			if os.IsNotExist(err) {
				// If resolved Workdir path gets marked as a valid symlink,
				// return nil cause this is valid use-case.
				if c.isWorkDirSymlink(resolvedWorkdir) {
					return nil
				}
				return fmt.Errorf("workdir %q does not exist on container %s", workdir, c.ID())
			}
			// This might be a serious error (e.g., permission), so
			// we need to return the full error.
			return fmt.Errorf("error detecting workdir %q on container %s: %w", workdir, c.ID(), err)
		}
		return nil
	}
	if err := os.MkdirAll(resolvedWorkdir, 0755); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("error creating container %s workdir: %w", c.ID(), err)
	}

	// Ensure container entrypoint is created (if required).
	uid, gid, _, err := chrootuser.GetUser(c.state.Mountpoint, c.User())
	if err != nil {
		return fmt.Errorf("error looking up %s inside of the container %s: %w", c.User(), c.ID(), err)
	}
	if err := os.Chown(resolvedWorkdir, int(uid), int(gid)); err != nil {
		return fmt.Errorf("error chowning container %s workdir to container root: %w", c.ID(), err)
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
	if path, ok := c.state.BindMounts["/etc/passwd"]; ok {
		overrides.ContainerEtcPasswdPath = path
	}
	return &overrides
}

func lookupHostUser(name string) (*runcuser.ExecUser, error) {
	var execUser runcuser.ExecUser
	// Look up User on host
	u, err := util.LookupUser(name)
	if err != nil {
		return &execUser, err
	}
	uid, err := strconv.ParseUint(u.Uid, 8, 32)
	if err != nil {
		return &execUser, err
	}

	gid, err := strconv.ParseUint(u.Gid, 8, 32)
	if err != nil {
		return &execUser, err
	}
	execUser.Uid = int(uid)
	execUser.Gid = int(gid)
	execUser.Home = u.HomeDir
	return &execUser, nil
}

// mountNotifySocket mounts the NOTIFY_SOCKET into the container if it's set
// and if the sdnotify mode is set to container.  It also sets c.notifySocket
// to avoid redundantly looking up the env variable.
func (c *Container) mountNotifySocket(g generate.Generator) error {
	if c.config.SdNotifySocket == "" {
		return nil
	}
	if c.config.SdNotifyMode != define.SdNotifyModeContainer {
		return nil
	}

	notifyDir := filepath.Join(c.bundlePath(), "notify")
	logrus.Debugf("Checking notify %q dir", notifyDir)
	if err := os.MkdirAll(notifyDir, 0755); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("unable to create notify %q dir: %w", notifyDir, err)
		}
	}
	if err := label.Relabel(notifyDir, c.MountLabel(), true); err != nil {
		return fmt.Errorf("relabel failed %q: %w", notifyDir, err)
	}
	logrus.Debugf("Add bindmount notify %q dir", notifyDir)
	if _, ok := c.state.BindMounts["/run/notify"]; !ok {
		c.state.BindMounts["/run/notify"] = notifyDir
	}

	// Set the container's notify socket to the proxy socket created by conmon
	g.AddProcessEnv("NOTIFY_SOCKET", "/run/notify/notify.sock")

	return nil
}
