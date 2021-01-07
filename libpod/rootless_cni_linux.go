// +build linux

package libpod

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/network"
	rootlesscni "github.com/containers/podman/v2/pkg/rootless/cni"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/containers/storage/pkg/mount"
	"github.com/hashicorp/go-multierror"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	rootlessCNIInfraContainerNamespace        = "podman-system"
	rootlessCNIInfraContainerName             = "rootless-cni-infra"
	rootlessCNIInfraContainerVersionLabelName = "rootless-cni-infra-container-version"
)

func getRootlessCNIConfig(c *Container, network string) (*rootlesscni.Config, error) {
	conf := rootlesscni.Config{
		ID:          c.ID(),
		Network:     network,
		CNIPodName:  getCNIPodName(c),
		PluginPaths: c.runtime.config.Network.CNIPluginDirs,
		NetConfPath: c.runtime.config.Network.NetworkConfigDir,
	}
	// add static ip if given
	if c.config.StaticIP != nil {
		conf.IP = c.config.StaticIP.String()
	}
	// add static mac if given
	if c.config.StaticMAC != nil {
		conf.MAC = c.config.StaticMAC.String()
	}
	// add aliases as CapabilityArgs
	aliases, err := c.runtime.state.GetAllNetworkAliases(c)
	if err != nil {
		return nil, err
	}
	conf.Aliases = aliases
	if eth, exists := c.state.NetInterfaceDescriptions.getInterfaceByName(network); exists {
		conf.InterfaceName = eth
	} else {
		return nil, errors.Errorf("no network interface name for %s", network)
	}
	return &conf, nil
}

// AllocRootlessCNI allocates a CNI netns inside the rootless CNI infra container.
// Locks "rootless-cni-infra.lck".
//
// When the infra container is not running, it is created.
//
// AllocRootlessCNI does not lock c. c should be already locked.
func AllocRootlessCNI(ctx context.Context, c *Container) (ns.NetNS, []*cnitypes.Result, error) {
	networks, _, err := c.networks()
	if err != nil {
		return nil, nil, err
	}
	if len(networks) == 0 {
		return nil, nil, errors.New("rootless CNI networking requires that the container has joined at least one CNI network")
	}
	// check early that all given networks exists
	for _, nw := range networks {
		exists, err := network.Exists(c.runtime.config, nw)
		if err != nil {
			return nil, nil, err
		}
		if !exists {
			return nil, nil, errors.Errorf("CNI network %q not found", nw)
		}
	}
	// Update container map of interface descriptions
	if err := c.setupNetworkDescriptions(networks); err != nil {
		return nil, nil, err
	}
	l, err := getRootlessCNIInfraLock(c.runtime)
	if err != nil {
		return nil, nil, err
	}
	l.Lock()
	defer l.Unlock()
	infra, err := ensureRootlessCNIInfraContainerRunning(ctx, c.runtime)
	if err != nil {
		return nil, nil, err
	}
	cniResults := make([]*cnitypes.Result, len(networks))
	for i, nw := range networks {
		rootlessCNIConf, err := getRootlessCNIConfig(c, nw)
		if err != nil {
			return nil, nil, err
		}

		cniRes, err := rootlessCNIInfraCallAlloc(infra, rootlessCNIConf)
		if err != nil {
			return nil, nil, err
		}
		cniResults[i] = cniRes
	}
	nsObj, err := rootlessCNIInfraGetNS(infra, c.ID())
	if err != nil {
		return nil, nil, err
	}
	logrus.Debugf("rootless CNI: container %q will join %q", c.ID(), nsObj.Path())
	return nsObj, cniResults, nil
}

// DeallocRootlessCNI deallocates a CNI netns inside the rootless CNI infra container.
// Locks "rootless-cni-infra.lck".
//
// When the infra container is no longer needed, it is removed.
//
// DeallocRootlessCNI does not lock c. c should be already locked.
func DeallocRootlessCNI(ctx context.Context, c *Container) error {
	networks, _, err := c.networks()
	if err != nil {
		return err
	}
	if len(networks) == 0 {
		return errors.New("rootless CNI networking requires that the container has joined at least one CNI network")
	}
	l, err := getRootlessCNIInfraLock(c.runtime)
	if err != nil {
		return err
	}
	l.Lock()
	defer l.Unlock()
	infra, _ := getRootlessCNIInfraContainer(c.runtime)
	if infra == nil {
		return nil
	}
	var errs *multierror.Error
	for _, nw := range networks {
		rootlessCNIConf, err := getRootlessCNIConfig(c, nw)
		if err != nil {
			return err
		}
		err = rootlessCNIInfraCallDealloc(infra, rootlessCNIConf)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	if isIdle, err := rootlessCNIInfraIsIdle(infra); isIdle || err != nil {
		if err != nil {
			logrus.Warn(err)
		}
		logrus.Debugf("rootless CNI: removing infra container %q", infra.ID())
		// Kill the infra container. There is no need to cleanup files because
		// are stored in tmpfs so we can just sigkill it. It is important to kill the
		// container before we remove it otherwise we have locking issues.
		if err := infra.Kill(9); err != nil {
			logrus.Error(err)
		}
		infra.lock.Lock()
		defer infra.lock.Unlock()
		if err := c.runtime.removeContainer(ctx, infra, true, false, true); err != nil {
			return err
		}
		rootfs := filepath.Join(c.runtime.GetStore().RunRoot(), rootlessCNIInfraContainerName)
		if err := mount.RecursiveUnmount(rootfs); err != nil {
			return errors.Wrapf(err, "failed to unmount rootfs for %s", rootlessCNIInfraContainerName)
		}
		logrus.Debugf("rootless CNI: removed infra container %q", infra.ID())
	}
	return errs.ErrorOrNil()
}

func getRootlessCNIInfraLock(r *Runtime) (lockfile.Locker, error) {
	fname := filepath.Join(r.config.Engine.TmpDir, "rootless-cni-infra.lck")
	return lockfile.GetLockfile(fname)
}

// getCNIPodName return the pod name (hostname) used by CNI and the dnsname plugin.
// If we are in the pod network namespace use the pod name otherwise the container name
func getCNIPodName(c *Container) string {
	if c.config.NetMode.IsPod() || c.IsInfra() {
		pod, err := c.runtime.GetPod(c.PodID())
		if err == nil {
			return pod.Name()
		}
	}
	return c.Name()
}

func rootlessCNIInfraCallAlloc(infra *Container, cfg *rootlesscni.Config) (*cnitypes.Result, error) {
	logrus.Debugf("rootless CNI: alloc %v", cfg)
	var err error
	var cniRes cnitypes.Result
	var cniResBytes []byte
	labels := infra.Labels()
	// we might want to check for the version here but for now the existence of the label is fine
	if _, ok := labels[rootlessCNIInfraContainerVersionLabelName]; ok {
		bytes, err := json.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		cniResBytes, err = rootlessCNIInfraExec(infra, bytes, "alloc")
		if err != nil {
			return nil, err
		}
	} else {
		// old rootless-cni-infra container api
		// keep for backwarts compatibility with previous version to support live migration
		// TODO: remove this in a future release maybe 4.0?

		// add network aliases json encoded as capabilityArgs for cni
		capArgs := ""
		if len(cfg.Aliases) > 0 {
			capabilityArgs := make(map[string]interface{})
			capabilityArgs["aliases"] = cfg.Aliases
			b, err := json.Marshal(capabilityArgs)
			if err != nil {
				return nil, err
			}
			capArgs = string(b)
		}
		_, err = rootlessCNIInfraExec(infra, nil, "alloc", cfg.ID, cfg.Network, cfg.CNIPodName, cfg.IP, cfg.MAC, capArgs)
		if err != nil {
			return nil, err
		}
		cniResBytes, err = rootlessCNIInfraExec(infra, nil, "print-cni-result", cfg.ID, cfg.Network)
		if err != nil {
			return nil, err
		}
	}
	if err := json.Unmarshal(cniResBytes, &cniRes); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling as cnitypes.Result: %q", string(cniResBytes))
	}
	return &cniRes, nil
}

func rootlessCNIInfraCallDealloc(infra *Container, cfg *rootlesscni.Config) error {
	logrus.Debugf("rootless CNI: dealloc %v", cfg)
	var err error
	labels := infra.Labels()
	// we might want to check for the version here but for now the existence of the label is fine
	if _, ok := labels[rootlessCNIInfraContainerVersionLabelName]; ok {
		var bytes []byte
		bytes, err = json.Marshal(cfg)
		if err != nil {
			return err
		}
		_, err = rootlessCNIInfraExec(infra, bytes, "dealloc")
	} else {
		// old rootless-cni-infra container api
		// keep for backwarts compatibility with previous version to support live migration
		// TODO: remove this in a future release maybe 4.0?
		_, err = rootlessCNIInfraExec(infra, nil, "dealloc", cfg.ID, cfg.Network)
	}
	return err
}

func rootlessCNIInfraIsIdle(infra *Container) (bool, error) {
	resBytes, err := rootlessCNIInfraExec(infra, nil, "is-idle")
	if err != nil {
		return false, err
	}
	var res rootlesscni.IsIdle
	if err := json.Unmarshal(resBytes, &res); err != nil {
		return false, errors.Wrapf(err, "unmarshaling as IsIdle: %q", string(resBytes))
	}
	return res.Idle, nil
}

func rootlessCNIInfraGetNS(infra *Container, cid string) (ns.NetNS, error) {
	resBytes, err := rootlessCNIInfraExec(infra, nil, "print-netns-path", cid)
	if err != nil {
		return nil, err
	}
	var res rootlesscni.PrintNetnsPath
	if err := json.Unmarshal(resBytes, &res); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling as PrintNetnsPath: %q", string(resBytes))
	}
	nsObj, err := ns.GetNS(res.Path)
	if err != nil {
		return nil, err
	}
	return nsObj, nil
}

func getRootlessCNIInfraContainer(r *Runtime) (*Container, error) {
	containers, err := r.GetContainersWithoutLock(func(c *Container) bool {
		return c.Namespace() == rootlessCNIInfraContainerNamespace &&
			c.Name() == rootlessCNIInfraContainerName
	})
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, nil
	}
	return containers[0], nil
}

func ensureRootlessCNIInfraContainerRunning(ctx context.Context, r *Runtime) (*Container, error) {
	c, err := getRootlessCNIInfraContainer(r)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return startRootlessCNIInfraContainer(ctx, r)
	}
	st, err := c.ContainerState()
	if err != nil {
		return nil, err
	}
	if st.State == define.ContainerStateRunning {
		logrus.Debugf("rootless CNI: infra container %q is already running", c.ID())
		return c, nil
	}
	// we have to mount the rootfs before we start it
	rootfs := filepath.Join(r.GetStore().RunRoot(), rootlessCNIInfraContainerName)
	err = mountRootlessCNIINfraRootfs(rootfs)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("rootless CNI: infra container %q is %q, being started", c.ID(), st.State)
	if err := c.initAndStart(ctx); err != nil {
		return nil, err
	}
	logrus.Debugf("rootless CNI: infra container %q is running", c.ID())
	return c, nil
}

func startRootlessCNIInfraContainer(ctx context.Context, r *Runtime) (*Container, error) {
	g, err := generate.New("linux")
	if err != nil {
		return nil, err
	}
	g.SetupPrivileged(true)
	// Set --pid=host for ease of propagating "/proc/PID/ns/net" string
	if err := g.RemoveLinuxNamespace(string(spec.PIDNamespace)); err != nil {
		return nil, err
	}
	g.RemoveMount("/proc")

	// need writable /run
	run := spec.Mount{
		Destination: "/run",
		Type:        "tmpfs",
		Source:      "none",
		Options:     []string{"rw", "nosuid", "nodev"},
	}
	g.AddMount(run)

	// mount /var as tmpfs
	// On ungraceful shutdown cni leaves the ip allocation files in place.
	// This causes issues when we try to use containers with the same ip again.
	// The best way to clean them up is using a tmpfs mount. These files do not have to
	// be persistent since the network namespace is destroyed anyway if the container exits.
	// CNI tries to write to /var/lib/cni however we cannot mount there because
	// it might not exists and we have no permission to create this directory.
	cni := spec.Mount{
		Destination: "/var",
		Type:        "tmpfs",
		Source:      "none",
		Options:     []string{"rw", "nosuid", "nodev"},
	}
	g.AddMount(cni)

	g.SetProcessArgs([]string{rootlesscni.InfraCmd, "sleep"})

	// get the current path this executable so we can mount it
	podmanexe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	podman := spec.Mount{
		// mount with different name to trigger the reexec for rooless-cni-infra
		Destination: rootlesscni.InfraCmd,
		Type:        "bind",
		Source:      podmanexe,
		Options:     []string{"ro", "bind"},
	}
	g.AddMount(podman)

	rootfs := filepath.Join(r.GetStore().RunRoot(), rootlessCNIInfraContainerName)
	err = mountRootlessCNIINfraRootfs(rootfs)
	if err != nil {
		return nil, err
	}

	g.SetRootReadonly(true)
	g.SetHostname(rootlessCNIInfraContainerName)

	infraLabels := map[string]string{
		rootlessCNIInfraContainerVersionLabelName: strconv.Itoa(rootlesscni.Version),
	}

	options := []CtrCreateOption{
		WithRootFS(rootfs),
		WithCtrNamespace(rootlessCNIInfraContainerNamespace),
		WithName(rootlessCNIInfraContainerName),
		WithPrivileged(true),
		// label=disable doesn't work correct for a rootfs mount
		// set labels manually to unconfined
		WithSecLabels([]string{"user:unconfined_u", "role:system_r", "type:unconfined_t"}),
		WithRestartPolicy("always"),
		WithNetNS(nil, false, "slirp4netns", nil),
		WithLabels(infraLabels),
	}
	c, err := r.NewContainer(ctx, g.Config, options...)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("rootless CNI infra container %q is created, now being started", c.ID())
	if err := c.initAndStart(ctx); err != nil {
		return nil, err
	}
	logrus.Debugf("rootless CNI: infra container %q is running", c.ID())

	return c, nil
}

func mountRootlessCNIINfraRootfs(rootfs string) error {
	if err := os.MkdirAll(rootfs, 0700); err != nil {
		return err
	}
	// bind mount the rootfs recursive in the userns
	// only the root will be read-only
	if err := mount.Mount("/", rootfs, "bind", "rbind,rprivate,ro"); err != nil {
		return errors.Wrapf(err, "failed to mount rootfs for %s", rootlessCNIInfraContainerName)
	}
	return nil
}

func rootlessCNIInfraExec(c *Container, stdin []byte, args ...string) ([]byte, error) {
	cmd := rootlesscni.InfraCmd
	labels := c.Labels()
	if _, ok := labels[rootlessCNIInfraContainerVersionLabelName]; !ok {
		// the old infra container had a different exec cmd
		// change it for backwarts compatibility
		cmd = rootlessCNIInfraContainerName
	}
	var (
		outB    bytes.Buffer
		errB    bytes.Buffer
		streams define.AttachStreams
		config  ExecConfig
	)

	if len(stdin) > 0 {
		logrus.Debugf("rootlessCNIInfraExec: stdin=%s", string(stdin))
		r := bufio.NewReader(bytes.NewReader(stdin))
		streams.InputStream = r
		streams.AttachInput = true
	}
	streams.OutputStream = &nopWriteCloser{Writer: &outB}
	streams.ErrorStream = &nopWriteCloser{Writer: &errB}
	streams.AttachOutput = true
	streams.AttachError = true
	config.Command = append([]string{cmd}, args...)
	config.Privileged = true
	logrus.Debugf("rootlessCNIInfraExec: c.ID()=%s, config=%+v, streams=%v, begin",
		c.ID(), config, streams)
	code, err := c.Exec(&config, &streams, nil)
	logrus.Debugf("rootlessCNIInfraExec: c.ID()=%s, config=%+v, streams=%v, end (code=%d, err=%v)",
		c.ID(), config, streams, code, err)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.Errorf("command %s %v in container %s failed with status %d, stdout=%q, stderr=%q",
			cmd, args, c.ID(), code, outB.String(), errB.String())
	}
	return outB.Bytes(), nil
}

type nopWriteCloser struct {
	io.Writer
}

func (nwc *nopWriteCloser) Close() error {
	return nil
}
