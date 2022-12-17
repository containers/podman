//go:build freebsd
// +build freebsd

package libpod

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/pkg/rootless"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	bindOptions = []string{}
)

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
		ctrNS                           string
		networkStatus                   map[string]types.StatusBlock
		createNetNSErr, mountStorageErr error
		mountPoint                      string
		tmpStateLock                    sync.Mutex
	)

	wg.Add(2)

	go func() {
		defer wg.Done()
		// Set up network namespace if not already set up
		noNetNS := c.state.NetNS == ""
		if c.config.CreateNetNS && noNetNS && !c.config.PostConfigureNetNS {
			ctrNS, networkStatus, createNetNSErr = c.runtime.createNetNS(c)
			if createNetNSErr != nil {
				return
			}

			tmpStateLock.Lock()
			defer tmpStateLock.Unlock()

			// Assign NetNS attributes to container
			c.state.NetNS = ctrNS
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
	if createNetNSErr != nil {
		createErr = createNetNSErr
	}
	if mountStorageErr != nil {
		if createErr != nil {
			logrus.Errorf("Preparing container %s: %v", c.ID(), createErr)
		}
		createErr = mountStorageErr
	}

	// Only trigger storage cleanup if mountStorage was successful.
	// Otherwise, we may mess up mount counters.
	if createErr != nil {
		if mountStorageErr == nil {
			if err := c.cleanupStorage(); err != nil {
				// createErr is guaranteed non-nil, so print
				// unconditionally
				logrus.Errorf("Preparing container %s: %v", c.ID(), createErr)
				createErr = fmt.Errorf("unmounting storage for container %s after network create failure: %w", c.ID(), err)
			}
		}
		// It's OK to unconditionally trigger network cleanup. If the network
		// isn't ready it will do nothing.
		if err := c.cleanupNetwork(); err != nil {
			logrus.Errorf("Preparing container %s: %v", c.ID(), createErr)
			createErr = fmt.Errorf("cleaning up container %s network after setup failure: %w", c.ID(), err)
		}
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
	if err != nil {
		return fmt.Errorf("retrieving dependency %s of container %s from state: %w", ctr, c.ID(), err)
	}
	c.runtime.state.UpdateContainer(nsCtr)
	if nsCtr.state.NetNS != "" {
		g.AddAnnotation("org.freebsd.parentJail", nsCtr.state.NetNS)
	}
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

func openDirectory(path string) (fd int, err error) {
	const O_PATH = 0x00400000
	return unix.Open(path, unix.O_RDONLY|O_PATH|unix.O_CLOEXEC, 0)
}

func (c *Container) addNetworkNamespace(g *generate.Generator) error {
	if c.config.CreateNetNS {
		if c.state.NetNS == "" {
			// This should not happen since network setup
			// errors should be propagated correctly from
			// (*Runtime).createNetNS. Check for it anyway
			// since it caused nil pointer dereferences in
			// the past (see #16333).
			return fmt.Errorf("Inconsistent state: c.config.CreateNetNS is set but c.state.NetNS is nil")
		}
		g.AddAnnotation("org.freebsd.parentJail", c.state.NetNS)
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

func (c *Container) addSlirp4netnsDNS(nameservers []string) []string {
	return nameservers
}

func (c *Container) isSlirp4netnsIPv6() (bool, error) {
	return false, nil
}

// check for net=none
func (c *Container) hasNetNone() bool {
	return c.state.NetNS == ""
}

func setVolumeAtime(mountPoint string, st os.FileInfo) error {
	stat := st.Sys().(*syscall.Stat_t)
	atime := time.Unix(int64(stat.Atimespec.Sec), int64(stat.Atimespec.Nsec)) //nolint: unconvert
	if err := os.Chtimes(mountPoint, atime, st.ModTime()); err != nil {
		return err
	}
	return nil
}

func (c *Container) makePlatformBindMounts() error {
	return nil
}

func (c *Container) getConmonPidFd() int {
	// Note: kqueue(2) could be used here but that would require
	// factoring out the call to unix.PollFd from WaitForExit so
	// keeping things simple for now.
	return -1
}

func (c *Container) jailName() string {
	if c.state.NetNS != "" {
		return c.state.NetNS + "." + c.ID()
	} else {
		return c.ID()
	}
}
