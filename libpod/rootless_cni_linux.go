// +build linux

package libpod

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"runtime"

	cnitypes "github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containers/podman/v2/libpod/define"
	"github.com/containers/podman/v2/libpod/image"
	"github.com/containers/podman/v2/pkg/env"
	"github.com/containers/podman/v2/pkg/util"
	"github.com/containers/storage/pkg/lockfile"
	"github.com/hashicorp/go-multierror"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Built from ../contrib/rootless-cni-infra.
var rootlessCNIInfraImage = map[string]string{
	"amd64": "quay.io/libpod/rootless-cni-infra@sha256:304742d5d221211df4ec672807a5842ff11e3729c50bc424ea0cea858f69d7b7", // 3-amd64
}

const (
	rootlessCNIInfraContainerNamespace = "podman-system"
	rootlessCNIInfraContainerName      = "rootless-cni-infra"
)

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
	k8sPodName := getCNIPodName(c) // passed to CNI as K8S_POD_NAME
	cniResults := make([]*cnitypes.Result, len(networks))
	for i, nw := range networks {
		cniRes, err := rootlessCNIInfraCallAlloc(infra, c.ID(), nw, k8sPodName)
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
		err := rootlessCNIInfraCallDelloc(infra, c.ID(), nw)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	if isIdle, err := rootlessCNIInfraIsIdle(infra); isIdle || err != nil {
		if err != nil {
			logrus.Warn(err)
		}
		logrus.Debugf("rootless CNI: removing infra container %q", infra.ID())
		if err := c.runtime.removeContainer(ctx, infra, true, false, true); err != nil {
			return err
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

func rootlessCNIInfraCallAlloc(infra *Container, id, nw, k8sPodName string) (*cnitypes.Result, error) {
	logrus.Debugf("rootless CNI: alloc %q, %q, %q", id, nw, k8sPodName)
	var err error

	_, err = rootlessCNIInfraExec(infra, "alloc", id, nw, k8sPodName)
	if err != nil {
		return nil, err
	}
	cniResStr, err := rootlessCNIInfraExec(infra, "print-cni-result", id, nw)
	if err != nil {
		return nil, err
	}
	var cniRes cnitypes.Result
	if err := json.Unmarshal([]byte(cniResStr), &cniRes); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling as cnitypes.Result: %q", cniResStr)
	}
	return &cniRes, nil
}

func rootlessCNIInfraCallDelloc(infra *Container, id, nw string) error {
	logrus.Debugf("rootless CNI: dealloc %q, %q", id, nw)
	_, err := rootlessCNIInfraExec(infra, "dealloc", id, nw)
	return err
}

func rootlessCNIInfraIsIdle(infra *Container) (bool, error) {
	type isIdle struct {
		Idle bool `json:"idle"`
	}
	resStr, err := rootlessCNIInfraExec(infra, "is-idle")
	if err != nil {
		return false, err
	}
	var res isIdle
	if err := json.Unmarshal([]byte(resStr), &res); err != nil {
		return false, errors.Wrapf(err, "unmarshaling as isIdle: %q", resStr)
	}
	return res.Idle, nil
}

func rootlessCNIInfraGetNS(infra *Container, id string) (ns.NetNS, error) {
	type printNetnsPath struct {
		Path string `json:"path"`
	}
	resStr, err := rootlessCNIInfraExec(infra, "print-netns-path", id)
	if err != nil {
		return nil, err
	}
	var res printNetnsPath
	if err := json.Unmarshal([]byte(resStr), &res); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling as printNetnsPath: %q", resStr)
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
	logrus.Debugf("rootless CNI: infra container %q is %q, being started", c.ID(), st.State)
	if err := c.initAndStart(ctx); err != nil {
		return nil, err
	}
	logrus.Debugf("rootless CNI: infra container %q is running", c.ID())
	return c, nil
}

func startRootlessCNIInfraContainer(ctx context.Context, r *Runtime) (*Container, error) {
	imageName, ok := rootlessCNIInfraImage[runtime.GOARCH]
	if !ok {
		return nil, errors.Errorf("cannot find rootless-podman-network-sandbox image for %s", runtime.GOARCH)
	}
	logrus.Debugf("rootless CNI: ensuring image %q to exist", imageName)
	newImage, err := r.ImageRuntime().New(ctx, imageName, "", "", nil, nil,
		image.SigningOptions{}, nil, util.PullImageMissing)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("rootless CNI: image %q is ready", imageName)

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
	procMount := spec.Mount{
		Destination: "/proc",
		Type:        "bind",
		Source:      "/proc",
		Options:     []string{"rbind", "nosuid", "noexec", "nodev"},
	}
	g.AddMount(procMount)
	// Mount CNI networks
	etcCNINetD := spec.Mount{
		Destination: "/etc/cni/net.d",
		Type:        "bind",
		Source:      r.config.Network.NetworkConfigDir,
		Options:     []string{"ro", "bind"},
	}
	g.AddMount(etcCNINetD)

	inspectData, err := newImage.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	imageEnv, err := env.ParseSlice(inspectData.Config.Env)
	if err != nil {
		return nil, err
	}
	for k, v := range imageEnv {
		g.AddProcessEnv(k, v)
	}
	if len(inspectData.Config.Cmd) == 0 {
		return nil, errors.Errorf("rootless CNI infra image %q has no command specified", imageName)
	}
	g.SetProcessArgs(inspectData.Config.Cmd)

	var options []CtrCreateOption
	options = append(options, WithRootFSFromImage(newImage.ID(), imageName, imageName))
	options = append(options, WithCtrNamespace(rootlessCNIInfraContainerNamespace))
	options = append(options, WithName(rootlessCNIInfraContainerName))
	options = append(options, WithPrivileged(true))
	options = append(options, WithSecLabels([]string{"disable"}))
	options = append(options, WithRestartPolicy("always"))
	options = append(options, WithNetNS(nil, false, "slirp4netns", nil))
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

func rootlessCNIInfraExec(c *Container, args ...string) (string, error) {
	cmd := "rootless-cni-infra"
	var (
		outB    bytes.Buffer
		errB    bytes.Buffer
		streams define.AttachStreams
		config  ExecConfig
	)
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
		return "", err
	}
	if code != 0 {
		return "", errors.Errorf("command %s %v in container %s failed with status %d, stdout=%q, stderr=%q",
			cmd, args, c.ID(), code, outB.String(), errB.String())
	}
	return outB.String(), nil
}

type nopWriteCloser struct {
	io.Writer
}

func (nwc *nopWriteCloser) Close() error {
	return nil
}
