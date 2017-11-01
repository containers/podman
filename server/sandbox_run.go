package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/containers/storage"
	"github.com/kubernetes-incubator/cri-o/libkpod/sandbox"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/kubernetes-incubator/cri-o/pkg/annotations"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	"k8s.io/kubernetes/pkg/api/v1"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/leaky"
	"k8s.io/kubernetes/pkg/kubelet/network/hostport"
	"k8s.io/kubernetes/pkg/kubelet/types"
)

const (
	// PodInfraOOMAdj is the value that we set for oom score adj for
	// the pod infra container.
	// TODO: Remove this const once this value is provided over CRI
	// See https://github.com/kubernetes/kubernetes/issues/47938
	PodInfraOOMAdj int = -998
	// PodInfraCPUshares is default cpu shares for sandbox container.
	PodInfraCPUshares = 2
)

// privilegedSandbox returns true if the sandbox configuration
// requires additional host privileges for the sandbox.
func (s *Server) privilegedSandbox(req *pb.RunPodSandboxRequest) bool {
	securityContext := req.GetConfig().GetLinux().GetSecurityContext()
	if securityContext == nil {
		return false
	}

	if securityContext.Privileged {
		return true
	}

	namespaceOptions := securityContext.GetNamespaceOptions()
	if namespaceOptions == nil {
		return false
	}

	if namespaceOptions.HostNetwork ||
		namespaceOptions.HostPid ||
		namespaceOptions.HostIpc {
		return true
	}

	return false
}

// trustedSandbox returns true if the sandbox will run trusted workloads.
func (s *Server) trustedSandbox(req *pb.RunPodSandboxRequest) bool {
	kubeAnnotations := req.GetConfig().GetAnnotations()

	trustedAnnotation, ok := kubeAnnotations[annotations.TrustedSandbox]
	if !ok {
		// A sandbox is trusted by default.
		return true
	}

	return isTrue(trustedAnnotation)
}

func (s *Server) runContainer(container *oci.Container, cgroupParent string) error {
	if err := s.Runtime().CreateContainer(container, cgroupParent); err != nil {
		return err
	}
	return s.Runtime().StartContainer(container)
}

var (
	conflictRE = regexp.MustCompile(`already reserved for pod "([0-9a-z]+)"`)
)

// RunPodSandbox creates and runs a pod-level sandbox.
func (s *Server) RunPodSandbox(ctx context.Context, req *pb.RunPodSandboxRequest) (resp *pb.RunPodSandboxResponse, err error) {
	s.updateLock.RLock()
	defer s.updateLock.RUnlock()

	logrus.Debugf("RunPodSandboxRequest %+v", req)
	var processLabel, mountLabel, resolvPath string
	// process req.Name
	kubeName := req.GetConfig().GetMetadata().Name
	if kubeName == "" {
		return nil, fmt.Errorf("PodSandboxConfig.Name should not be empty")
	}

	namespace := req.GetConfig().GetMetadata().Namespace
	attempt := req.GetConfig().GetMetadata().Attempt

	id, name, err := s.generatePodIDandName(req.GetConfig())
	if err != nil {
		if strings.Contains(err.Error(), "already reserved for pod") {
			matches := conflictRE.FindStringSubmatch(err.Error())
			if len(matches) != 2 {
				return nil, err
			}
			dupID := matches[1]
			if _, err := s.StopPodSandbox(ctx, &pb.StopPodSandboxRequest{PodSandboxId: dupID}); err != nil {
				return nil, err
			}
			if _, err := s.RemovePodSandbox(ctx, &pb.RemovePodSandboxRequest{PodSandboxId: dupID}); err != nil {
				return nil, err
			}
			id, name, err = s.generatePodIDandName(req.GetConfig())
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	defer func() {
		if err != nil {
			s.ReleasePodName(name)
		}
	}()

	_, containerName, err := s.generateContainerIDandNameForSandbox(req.GetConfig())
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			s.ReleaseContainerName(containerName)
		}
	}()

	podContainer, err := s.StorageRuntimeServer().CreatePodSandbox(s.ImageContext(),
		name, id,
		s.config.PauseImage, "",
		containerName,
		req.GetConfig().GetMetadata().Name,
		req.GetConfig().GetMetadata().Uid,
		namespace,
		attempt,
		nil)
	if errors.Cause(err) == storage.ErrDuplicateName {
		return nil, fmt.Errorf("pod sandbox with name %q already exists", name)
	}
	if err != nil {
		return nil, fmt.Errorf("error creating pod sandbox with name %q: %v", name, err)
	}
	defer func() {
		if err != nil {
			if err2 := s.StorageRuntimeServer().RemovePodSandbox(id); err2 != nil {
				logrus.Warnf("couldn't cleanup pod sandbox %q: %v", id, err2)
			}
		}
	}()

	// TODO: factor generating/updating the spec into something other projects can vendor

	// creates a spec Generator with the default spec.
	g := generate.New()

	// setup defaults for the pod sandbox
	g.SetRootReadonly(true)
	if s.config.PauseCommand == "" {
		if podContainer.Config != nil {
			g.SetProcessArgs(podContainer.Config.Config.Cmd)
		} else {
			g.SetProcessArgs([]string{sandbox.PodInfraCommand})
		}
	} else {
		g.SetProcessArgs([]string{s.config.PauseCommand})
	}

	// set DNS options
	if req.GetConfig().GetDnsConfig() != nil {
		dnsServers := req.GetConfig().GetDnsConfig().Servers
		dnsSearches := req.GetConfig().GetDnsConfig().Searches
		dnsOptions := req.GetConfig().GetDnsConfig().Options
		resolvPath = fmt.Sprintf("%s/resolv.conf", podContainer.RunDir)
		err = parseDNSOptions(dnsServers, dnsSearches, dnsOptions, resolvPath)
		if err != nil {
			err1 := removeFile(resolvPath)
			if err1 != nil {
				err = err1
				return nil, fmt.Errorf("%v; failed to remove %s: %v", err, resolvPath, err1)
			}
			return nil, err
		}
		if err := label.Relabel(resolvPath, mountLabel, true); err != nil && err != unix.ENOTSUP {
			return nil, err
		}

		g.AddBindMount(resolvPath, "/etc/resolv.conf", []string{"ro"})
	}

	// add metadata
	metadata := req.GetConfig().GetMetadata()
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	// add labels
	labels := req.GetConfig().GetLabels()

	// Add special container name label for the infra container
	labelsJSON := []byte{}
	if labels != nil {
		labels[types.KubernetesContainerNameLabel] = leaky.PodInfraContainerName
		labelsJSON, err = json.Marshal(labels)
		if err != nil {
			return nil, err
		}
	}

	// add annotations
	kubeAnnotations := req.GetConfig().GetAnnotations()
	kubeAnnotationsJSON, err := json.Marshal(kubeAnnotations)
	if err != nil {
		return nil, err
	}

	// set log directory
	logDir := req.GetConfig().LogDirectory
	if logDir == "" {
		logDir = filepath.Join(s.config.LogDir, id)
	}
	if err = os.MkdirAll(logDir, 0700); err != nil {
		return nil, err
	}
	// This should always be absolute from k8s.
	if !filepath.IsAbs(logDir) {
		return nil, fmt.Errorf("requested logDir for sbox id %s is a relative path: %s", id, logDir)
	}

	privileged := s.privilegedSandbox(req)

	securityContext := req.GetConfig().GetLinux().GetSecurityContext()
	if securityContext == nil {
		logrus.Warn("no security context found in config.")
	}

	processLabel, mountLabel, err = getSELinuxLabels(securityContext.GetSelinuxOptions(), privileged)
	if err != nil {
		return nil, err
	}

	// Don't use SELinux separation with Host Pid or IPC Namespace or privileged.
	if securityContext.GetNamespaceOptions().GetHostPid() || securityContext.GetNamespaceOptions().GetHostIpc() {
		processLabel, mountLabel = "", ""
	}
	g.SetProcessSelinuxLabel(processLabel)
	g.SetLinuxMountLabel(mountLabel)

	// create shm mount for the pod containers.
	var shmPath string
	if securityContext.GetNamespaceOptions().GetHostIpc() {
		shmPath = "/dev/shm"
	} else {
		shmPath, err = setupShm(podContainer.RunDir, mountLabel)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err != nil {
				if err2 := unix.Unmount(shmPath, unix.MNT_DETACH); err2 != nil {
					logrus.Warnf("failed to unmount shm for pod: %v", err2)
				}
			}
		}()
	}

	err = s.setPodSandboxMountLabel(id, mountLabel)
	if err != nil {
		return nil, err
	}

	if err = s.CtrIDIndex().Add(id); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			if err2 := s.CtrIDIndex().Delete(id); err2 != nil {
				logrus.Warnf("couldn't delete ctr id %s from idIndex", id)
			}
		}
	}()

	// set log path inside log directory
	logPath := filepath.Join(logDir, id+".log")

	// Handle https://issues.k8s.io/44043
	if err := ensureSaneLogPath(logPath); err != nil {
		return nil, err
	}

	hostNetwork := securityContext.GetNamespaceOptions().GetHostNetwork()

	hostname, err := getHostname(id, req.GetConfig().Hostname, hostNetwork)
	if err != nil {
		return nil, err
	}
	g.SetHostname(hostname)

	trusted := s.trustedSandbox(req)
	g.AddAnnotation(annotations.Metadata, string(metadataJSON))
	g.AddAnnotation(annotations.Labels, string(labelsJSON))
	g.AddAnnotation(annotations.Annotations, string(kubeAnnotationsJSON))
	g.AddAnnotation(annotations.LogPath, logPath)
	g.AddAnnotation(annotations.Name, name)
	g.AddAnnotation(annotations.ContainerType, annotations.ContainerTypeSandbox)
	g.AddAnnotation(annotations.SandboxID, id)
	g.AddAnnotation(annotations.ContainerName, containerName)
	g.AddAnnotation(annotations.ContainerID, id)
	g.AddAnnotation(annotations.ShmPath, shmPath)
	g.AddAnnotation(annotations.PrivilegedRuntime, fmt.Sprintf("%v", privileged))
	g.AddAnnotation(annotations.TrustedSandbox, fmt.Sprintf("%v", trusted))
	g.AddAnnotation(annotations.ResolvPath, resolvPath)
	g.AddAnnotation(annotations.HostName, hostname)
	g.AddAnnotation(annotations.KubeName, kubeName)
	if podContainer.Config.Config.StopSignal != "" {
		// this key is defined in image-spec conversion document at https://github.com/opencontainers/image-spec/pull/492/files#diff-8aafbe2c3690162540381b8cdb157112R57
		g.AddAnnotation("org.opencontainers.image.stopSignal", podContainer.Config.Config.StopSignal)
	}

	created := time.Now()
	g.AddAnnotation(annotations.Created, created.Format(time.RFC3339Nano))

	portMappings := convertPortMappings(req.GetConfig().GetPortMappings())

	// setup cgroup settings
	cgroupParent := req.GetConfig().GetLinux().GetCgroupParent()
	if cgroupParent != "" {
		if s.config.CgroupManager == oci.SystemdCgroupsManager {
			if len(cgroupParent) <= 6 || !strings.HasSuffix(path.Base(cgroupParent), ".slice") {
				return nil, fmt.Errorf("cri-o configured with systemd cgroup manager, but did not receive slice as parent: %s", cgroupParent)
			}
			cgPath, err := convertCgroupFsNameToSystemd(cgroupParent)
			if err != nil {
				return nil, err
			}
			g.SetLinuxCgroupsPath(cgPath + ":" + "crio" + ":" + id)
			cgroupParent = cgPath
		} else {
			if strings.HasSuffix(path.Base(cgroupParent), ".slice") {
				return nil, fmt.Errorf("cri-o configured with cgroupfs cgroup manager, but received systemd slice as parent: %s", cgroupParent)
			}
			cgPath := filepath.Join(cgroupParent, scopePrefix+"-"+id)
			g.SetLinuxCgroupsPath(cgPath)
		}
	}

	sb, err := sandbox.New(id, namespace, name, kubeName, logDir, labels, kubeAnnotations, processLabel, mountLabel, metadata, shmPath, cgroupParent, privileged, trusted, resolvPath, hostname, portMappings)
	if err != nil {
		return nil, err
	}

	s.addSandbox(sb)
	defer func() {
		if err != nil {
			s.removeSandbox(id)
		}
	}()

	if err = s.PodIDIndex().Add(id); err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			if err := s.PodIDIndex().Delete(id); err != nil {
				logrus.Warnf("couldn't delete pod id %s from idIndex", id)
			}
		}
	}()

	for k, v := range kubeAnnotations {
		g.AddAnnotation(k, v)
	}
	for k, v := range labels {
		g.AddAnnotation(k, v)
	}

	// extract linux sysctls from annotations and pass down to oci runtime
	safe, unsafe, err := SysctlsFromPodAnnotations(kubeAnnotations)
	if err != nil {
		return nil, err
	}
	for _, sysctl := range safe {
		g.AddLinuxSysctl(sysctl.Name, sysctl.Value)
	}
	for _, sysctl := range unsafe {
		g.AddLinuxSysctl(sysctl.Name, sysctl.Value)
	}

	// Set OOM score adjust of the infra container to be very low
	// so it doesn't get killed.
	g.SetProcessOOMScoreAdj(PodInfraOOMAdj)

	g.SetLinuxResourcesCPUShares(PodInfraCPUshares)

	// set up namespaces
	if hostNetwork {
		err = g.RemoveLinuxNamespace(string(runtimespec.NetworkNamespace))
		if err != nil {
			return nil, err
		}
	} else {
		// Create the sandbox network namespace
		if err = sb.NetNsCreate(); err != nil {
			return nil, err
		}

		defer func() {
			if err == nil {
				return
			}

			if netnsErr := sb.NetNsRemove(); netnsErr != nil {
				logrus.Warnf("Failed to remove networking namespace: %v", netnsErr)
			}
		}()

		// Pass the created namespace path to the runtime
		err = g.AddOrReplaceLinuxNamespace(string(runtimespec.NetworkNamespace), sb.NetNsPath())
		if err != nil {
			return nil, err
		}
	}

	if securityContext.GetNamespaceOptions().GetHostPid() {
		err = g.RemoveLinuxNamespace(string(runtimespec.PIDNamespace))
		if err != nil {
			return nil, err
		}
	}

	if securityContext.GetNamespaceOptions().GetHostIpc() {
		err = g.RemoveLinuxNamespace(string(runtimespec.IPCNamespace))
		if err != nil {
			return nil, err
		}
	}

	if !s.seccompEnabled {
		g.Spec().Linux.Seccomp = nil
	}

	saveOptions := generate.ExportOptions{}
	mountPoint, err := s.StorageRuntimeServer().StartContainer(id)
	if err != nil {
		return nil, fmt.Errorf("failed to mount container %s in pod sandbox %s(%s): %v", containerName, sb.Name(), id, err)
	}
	g.AddAnnotation(annotations.MountPoint, mountPoint)
	g.SetRootPath(mountPoint)

	hostnamePath := fmt.Sprintf("%s/hostname", podContainer.RunDir)
	if err := ioutil.WriteFile(hostnamePath, []byte(hostname+"\n"), 0644); err != nil {
		return nil, err
	}
	if err := label.Relabel(hostnamePath, mountLabel, true); err != nil && err != unix.ENOTSUP {
		return nil, err
	}
	g.AddBindMount(hostnamePath, "/etc/hostname", []string{"ro"})
	g.AddAnnotation(annotations.HostnamePath, hostnamePath)
	sb.AddHostnamePath(hostnamePath)

	container, err := oci.NewContainer(id, containerName, podContainer.RunDir, logPath, sb.NetNs(), labels, g.Spec().Annotations, kubeAnnotations, "", "", "", nil, id, false, false, false, sb.Privileged(), sb.Trusted(), podContainer.Dir, created, podContainer.Config.Config.StopSignal)
	if err != nil {
		return nil, err
	}
	container.SetSpec(g.Spec())
	container.SetMountPoint(mountPoint)

	sb.SetInfraContainer(container)

	var ip string
	ip, err = s.networkStart(hostNetwork, sb)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			s.networkStop(hostNetwork, sb)
		}
	}()

	g.AddAnnotation(annotations.IP, ip)
	sb.AddIP(ip)

	err = g.SaveToFile(filepath.Join(podContainer.Dir, "config.json"), saveOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to save template configuration for pod sandbox %s(%s): %v", sb.Name(), id, err)
	}
	if err = g.SaveToFile(filepath.Join(podContainer.RunDir, "config.json"), saveOptions); err != nil {
		return nil, fmt.Errorf("failed to write runtime configuration for pod sandbox %s(%s): %v", sb.Name(), id, err)
	}

	if err = s.runContainer(container, sb.CgroupParent()); err != nil {
		return nil, err
	}

	s.addInfraContainer(container)

	s.ContainerStateToDisk(container)

	resp = &pb.RunPodSandboxResponse{PodSandboxId: id}
	logrus.Debugf("RunPodSandboxResponse: %+v", resp)
	return resp, nil
}

func convertPortMappings(in []*pb.PortMapping) []*hostport.PortMapping {
	if in == nil {
		return nil
	}
	out := make([]*hostport.PortMapping, len(in))
	for i, v := range in {
		out[i] = &hostport.PortMapping{
			HostPort:      v.HostPort,
			ContainerPort: v.ContainerPort,
			Protocol:      v1.Protocol(v.Protocol.String()),
			HostIP:        v.HostIp,
		}
	}
	return out
}

func getHostname(id, hostname string, hostNetwork bool) (string, error) {
	if hostNetwork {
		if hostname == "" {
			h, err := os.Hostname()
			if err != nil {
				return "", err
			}
			hostname = h
		}
	} else {
		if hostname == "" {
			hostname = id[:12]
		}
	}
	return hostname, nil
}

func (s *Server) setPodSandboxMountLabel(id, mountLabel string) error {
	storageMetadata, err := s.StorageRuntimeServer().GetContainerMetadata(id)
	if err != nil {
		return err
	}
	storageMetadata.SetMountLabel(mountLabel)
	return s.StorageRuntimeServer().SetContainerMetadata(id, storageMetadata)
}

func getSELinuxLabels(selinuxOptions *pb.SELinuxOption, privileged bool) (processLabel string, mountLabel string, err error) {
	if privileged {
		return "", "", nil
	}
	labels := []string{}
	if selinuxOptions != nil {
		if selinuxOptions.User != "" {
			labels = append(labels, "user:"+selinuxOptions.User)
		}
		if selinuxOptions.Role != "" {
			labels = append(labels, "role:"+selinuxOptions.Role)
		}
		if selinuxOptions.Type != "" {
			labels = append(labels, "type:"+selinuxOptions.Type)
		}
		if selinuxOptions.Level != "" {
			labels = append(labels, "level:"+selinuxOptions.Level)
		}
	}
	return label.InitLabels(labels)
}

func setupShm(podSandboxRunDir, mountLabel string) (shmPath string, err error) {
	shmPath = filepath.Join(podSandboxRunDir, "shm")
	if err = os.Mkdir(shmPath, 0700); err != nil {
		return "", err
	}
	shmOptions := "mode=1777,size=" + strconv.Itoa(sandbox.DefaultShmSize)
	if err = unix.Mount("shm", shmPath, "tmpfs", unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV,
		label.FormatMountLabel(shmOptions, mountLabel)); err != nil {
		return "", fmt.Errorf("failed to mount shm tmpfs for pod: %v", err)
	}
	return shmPath, nil
}

// convertCgroupFsNameToSystemd converts an expanded cgroupfs name to its systemd name.
// For example, it will convert test.slice/test-a.slice/test-a-b.slice to become test-a-b.slice
// NOTE: this is public right now to allow its usage in dockermanager and dockershim, ideally both those
// code areas could use something from libcontainer if we get this style function upstream.
func convertCgroupFsNameToSystemd(cgroupfsName string) (string, error) {
	// TODO: see if libcontainer systemd implementation could use something similar, and if so, move
	// this function up to that library.  At that time, it would most likely do validation specific to systemd
	// above and beyond the simple assumption here that the base of the path encodes the hierarchy
	// per systemd convention.
	return path.Base(cgroupfsName), nil
}
