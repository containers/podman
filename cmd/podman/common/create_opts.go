package common

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/containers/podman/v2/pkg/api/handlers"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/pkg/specgen"
)

type ContainerCLIOpts struct {
	Annotation        []string
	Attach            []string
	Authfile          string
	BlkIOWeight       string
	BlkIOWeightDevice []string
	CapAdd            []string
	CapDrop           []string
	CgroupNS          string
	CGroupsMode       string
	CGroupParent      string
	CIDFile           string
	ConmonPIDFile     string
	CPUPeriod         uint64
	CPUQuota          int64
	CPURTPeriod       uint64
	CPURTRuntime      int64
	CPUShares         uint64
	CPUS              float64
	CPUSetCPUs        string
	CPUSetMems        string
	Devices           []string
	DeviceCGroupRule  []string
	DeviceReadBPs     []string
	DeviceReadIOPs    []string
	DeviceWriteBPs    []string
	DeviceWriteIOPs   []string
	Entrypoint        *string
	Env               []string
	EnvHost           bool
	EnvFile           []string
	Expose            []string
	GIDMap            []string
	GroupAdd          []string
	HealthCmd         string
	HealthInterval    string
	HealthRetries     uint
	HealthStartPeriod string
	HealthTimeout     string
	Hostname          string
	HTTPProxy         bool
	ImageVolume       string
	Init              bool
	InitPath          string
	Interactive       bool
	IPC               string
	KernelMemory      string
	Label             []string
	LabelFile         []string
	LogDriver         string
	LogOptions        []string
	Memory            string
	MemoryReservation string
	MemorySwap        string
	MemorySwappiness  int64
	Name              string
	NoHealthCheck     bool
	OOMKillDisable    bool
	OOMScoreAdj       int
	OverrideArch      string
	OverrideOS        string
	OverrideVariant   string
	PID               string
	PIDsLimit         *int64
	Pod               string
	PodIDFile         string
	PreserveFDs       uint
	Privileged        bool
	PublishAll        bool
	Pull              string
	Quiet             bool
	ReadOnly          bool
	ReadOnlyTmpFS     bool
	Restart           string
	Replace           bool
	Rm                bool
	RootFS            bool
	SecurityOpt       []string
	SdNotifyMode      string
	ShmSize           string
	SignaturePolicy   string
	StopSignal        string
	StopTimeout       uint
	StoreageOpt       []string
	SubUIDName        string
	SubGIDName        string
	Sysctl            []string
	Systemd           string
	TmpFS             []string
	TTY               bool
	Timezone          string
	Umask             string
	UIDMap            []string
	Ulimit            []string
	User              string
	UserNS            string
	UTS               string
	Mount             []string
	Volume            []string
	VolumesFrom       []string
	Workdir           string
	SeccompPolicy     string

	Net *entities.NetOptions

	CgroupConf []string
}

func stringMaptoArray(m map[string]string) []string {
	a := make([]string, 0, len(m))
	for k, v := range m {
		a = append(a, fmt.Sprintf("%s=%s", k, v))
	}
	return a
}

// ContainerCreateToContainerCLIOpts converts a compat input struct to cliopts so it can be converted to
// a specgen spec.
func ContainerCreateToContainerCLIOpts(cc handlers.CreateContainerConfig) (*ContainerCLIOpts, []string, error) {
	var (
		capAdd     []string
		cappDrop   []string
		entrypoint string
		init       bool
		specPorts  []specgen.PortMapping
	)

	if cc.HostConfig.Init != nil {
		init = *cc.HostConfig.Init
	}

	// Iterate devices and convert back to string
	devices := make([]string, 0, len(cc.HostConfig.Devices))
	for _, dev := range cc.HostConfig.Devices {
		devices = append(devices, fmt.Sprintf("%s:%s:%s", dev.PathOnHost, dev.PathInContainer, dev.CgroupPermissions))
	}

	// iterate blkreaddevicebps
	readBps := make([]string, 0, len(cc.HostConfig.BlkioDeviceReadBps))
	for _, dev := range cc.HostConfig.BlkioDeviceReadBps {
		readBps = append(readBps, dev.String())
	}

	// iterate blkreaddeviceiops
	readIops := make([]string, 0, len(cc.HostConfig.BlkioDeviceReadIOps))
	for _, dev := range cc.HostConfig.BlkioDeviceReadIOps {
		readIops = append(readIops, dev.String())
	}

	// iterate blkwritedevicebps
	writeBps := make([]string, 0, len(cc.HostConfig.BlkioDeviceWriteBps))
	for _, dev := range cc.HostConfig.BlkioDeviceWriteBps {
		writeBps = append(writeBps, dev.String())
	}

	// iterate blkwritedeviceiops
	writeIops := make([]string, 0, len(cc.HostConfig.BlkioDeviceWriteIOps))
	for _, dev := range cc.HostConfig.BlkioDeviceWriteIOps {
		writeIops = append(writeIops, dev.String())
	}

	// entrypoint
	// can be a string or slice. if it is a slice, we need to
	// marshall it to json; otherwise it should just be the string
	// value
	if len(cc.Config.Entrypoint) > 0 {
		entrypoint = cc.Config.Entrypoint[0]
		if len(cc.Config.Entrypoint) > 1 {
			b, err := json.Marshal(cc.Config.Entrypoint)
			if err != nil {
				return nil, nil, err
			}
			entrypoint = string(b)
		}
	}

	// expose ports
	expose := make([]string, 0, len(cc.Config.ExposedPorts))
	for p := range cc.Config.ExposedPorts {
		expose = append(expose, fmt.Sprintf("%s/%s", p.Port(), p.Proto()))
	}

	// mounts type=tmpfs/bind,source=,dest=,opt=val
	// TODO options
	mounts := make([]string, 0, len(cc.HostConfig.Mounts))
	for _, m := range cc.HostConfig.Mounts {
		mount := fmt.Sprintf("type=%s", m.Type)
		if len(m.Source) > 0 {
			mount += fmt.Sprintf("source=%s", m.Source)
		}
		if len(m.Target) > 0 {
			mount += fmt.Sprintf("dest=%s", m.Target)
		}
		mounts = append(mounts, mount)
	}

	//volumes
	volumes := make([]string, 0, len(cc.Config.Volumes))
	for v := range cc.Config.Volumes {
		volumes = append(volumes, v)
	}

	// dns
	dns := make([]net.IP, 0, len(cc.HostConfig.DNS))
	for _, d := range cc.HostConfig.DNS {
		dns = append(dns, net.ParseIP(d))
	}

	// publish
	for port, pbs := range cc.HostConfig.PortBindings {
		for _, pb := range pbs {
			hostport, err := strconv.Atoi(pb.HostPort)
			if err != nil {
				return nil, nil, err
			}
			tmpPort := specgen.PortMapping{
				HostIP:        pb.HostIP,
				ContainerPort: uint16(port.Int()),
				HostPort:      uint16(hostport),
				Range:         0,
				Protocol:      port.Proto(),
			}
			specPorts = append(specPorts, tmpPort)
		}
	}

	// network names
	endpointsConfig := cc.NetworkingConfig.EndpointsConfig
	cniNetworks := make([]string, 0, len(endpointsConfig))
	for netName := range endpointsConfig {
		cniNetworks = append(cniNetworks, netName)
	}

	// netMode
	nsmode, _, err := specgen.ParseNetworkNamespace(cc.HostConfig.NetworkMode.NetworkName())
	if err != nil {
		return nil, nil, err
	}

	netNS := specgen.Namespace{
		NSMode: nsmode.NSMode,
		Value:  nsmode.Value,
	}

	// network
	// Note: we cannot emulate compat exactly here. we only allow specifics of networks to be
	// defined when there is only one network.
	netInfo := entities.NetOptions{
		AddHosts:     cc.HostConfig.ExtraHosts,
		CNINetworks:  cniNetworks,
		DNSOptions:   cc.HostConfig.DNSOptions,
		DNSSearch:    cc.HostConfig.DNSSearch,
		DNSServers:   dns,
		Network:      netNS,
		PublishPorts: specPorts,
	}

	// static IP and MAC
	if len(endpointsConfig) == 1 {
		for _, ep := range endpointsConfig {
			// if IP address is provided
			if len(ep.IPAddress) > 0 {
				staticIP := net.ParseIP(ep.IPAddress)
				netInfo.StaticIP = &staticIP
			}
			// If MAC address is provided
			if len(ep.MacAddress) > 0 {
				staticMac, err := net.ParseMAC(ep.MacAddress)
				if err != nil {
					return nil, nil, err
				}
				netInfo.StaticMAC = &staticMac
			}
			break
		}
	}

	// Note: several options here are marked as "don't need". this is based
	// on speculation by Matt and I. We think that these come into play later
	// like with start. We believe this is just a difference in podman/compat
	cliOpts := ContainerCLIOpts{
		//Attach:            nil, // dont need?
		Authfile:     "",
		CapAdd:       append(capAdd, cc.HostConfig.CapAdd...),
		CapDrop:      append(cappDrop, cc.HostConfig.CapDrop...),
		CGroupParent: cc.HostConfig.CgroupParent,
		CIDFile:      cc.HostConfig.ContainerIDFile,
		CPUPeriod:    uint64(cc.HostConfig.CPUPeriod),
		CPUQuota:     cc.HostConfig.CPUQuota,
		CPURTPeriod:  uint64(cc.HostConfig.CPURealtimePeriod),
		CPURTRuntime: cc.HostConfig.CPURealtimeRuntime,
		CPUShares:    uint64(cc.HostConfig.CPUShares),
		//CPUS:              0, // dont need?
		CPUSetCPUs: cc.HostConfig.CpusetCpus,
		CPUSetMems: cc.HostConfig.CpusetMems,
		//Detach:            false, // dont need
		//DetachKeys:        "",    // dont need
		Devices:          devices,
		DeviceCGroupRule: nil,
		DeviceReadBPs:    readBps,
		DeviceReadIOPs:   readIops,
		DeviceWriteBPs:   writeBps,
		DeviceWriteIOPs:  writeIops,
		Entrypoint:       &entrypoint,
		Env:              cc.Config.Env,
		Expose:           expose,
		GroupAdd:         cc.HostConfig.GroupAdd,
		Hostname:         cc.Config.Hostname,
		ImageVolume:      "bind",
		Init:             init,
		Interactive:      cc.Config.OpenStdin,
		IPC:              string(cc.HostConfig.IpcMode),
		Label:            stringMaptoArray(cc.Config.Labels),
		LogDriver:        cc.HostConfig.LogConfig.Type,
		LogOptions:       stringMaptoArray(cc.HostConfig.LogConfig.Config),
		Name:             cc.Name,
		OOMScoreAdj:      cc.HostConfig.OomScoreAdj,
		OverrideArch:     "",
		OverrideOS:       "",
		OverrideVariant:  "",
		PID:              string(cc.HostConfig.PidMode),
		PIDsLimit:        cc.HostConfig.PidsLimit,
		Privileged:       cc.HostConfig.Privileged,
		PublishAll:       cc.HostConfig.PublishAllPorts,
		Quiet:            false,
		ReadOnly:         cc.HostConfig.ReadonlyRootfs,
		ReadOnlyTmpFS:    true, // podman default
		Rm:               cc.HostConfig.AutoRemove,
		SecurityOpt:      cc.HostConfig.SecurityOpt,
		StopSignal:       cc.Config.StopSignal,
		StoreageOpt:      stringMaptoArray(cc.HostConfig.StorageOpt),
		Sysctl:           stringMaptoArray(cc.HostConfig.Sysctls),
		Systemd:          "true", // podman default
		TmpFS:            stringMaptoArray(cc.HostConfig.Tmpfs),
		TTY:              cc.Config.Tty,
		//Ulimit:            cc.HostConfig.Ulimits,            // ask dan, no documented format
		Ulimit:      []string{"nproc=4194304:4194304"},
		User:        cc.Config.User,
		UserNS:      string(cc.HostConfig.UsernsMode),
		UTS:         string(cc.HostConfig.UTSMode),
		Mount:       mounts,
		Volume:      volumes,
		VolumesFrom: cc.HostConfig.VolumesFrom,
		Workdir:     cc.Config.WorkingDir,
		Net:         &netInfo,
	}

	if len(cc.HostConfig.BlkioWeightDevice) > 0 {
		devices := make([]string, 0, len(cc.HostConfig.BlkioWeightDevice))
		for _, d := range cc.HostConfig.BlkioWeightDevice {
			devices = append(devices, d.String())
		}
		cliOpts.BlkIOWeightDevice = devices
	}
	if cc.HostConfig.BlkioWeight > 0 {
		cliOpts.BlkIOWeight = strconv.Itoa(int(cc.HostConfig.BlkioWeight))
	}

	if cc.HostConfig.Memory > 0 {
		cliOpts.Memory = strconv.Itoa(int(cc.HostConfig.Memory))
	}

	if cc.HostConfig.MemoryReservation > 0 {
		cliOpts.MemoryReservation = strconv.Itoa(int(cc.HostConfig.MemoryReservation))
	}

	if cc.HostConfig.MemorySwap > 0 {
		cliOpts.MemorySwap = strconv.Itoa(int(cc.HostConfig.MemorySwap))
	}

	if cc.Config.StopTimeout != nil {
		cliOpts.StopTimeout = uint(*cc.Config.StopTimeout)
	}

	if cc.HostConfig.ShmSize > 0 {
		cliOpts.ShmSize = strconv.Itoa(int(cc.HostConfig.ShmSize))
	}

	if cc.HostConfig.KernelMemory > 0 {
		cliOpts.KernelMemory = strconv.Itoa(int(cc.HostConfig.KernelMemory))
	}
	if len(cc.HostConfig.RestartPolicy.Name) > 0 {
		policy := cc.HostConfig.RestartPolicy.Name
		// only add restart count on failure
		if cc.HostConfig.RestartPolicy.IsOnFailure() {
			policy += fmt.Sprintf(":%d", cc.HostConfig.RestartPolicy.MaximumRetryCount)
		}
		cliOpts.Restart = policy
	}

	if cc.HostConfig.MemorySwappiness != nil {
		cliOpts.MemorySwappiness = *cc.HostConfig.MemorySwappiness
	}
	if cc.HostConfig.OomKillDisable != nil {
		cliOpts.OOMKillDisable = *cc.HostConfig.OomKillDisable
	}
	if cc.Config.Healthcheck != nil {
		cliOpts.HealthCmd = strings.Join(cc.Config.Healthcheck.Test, " ")
		cliOpts.HealthInterval = cc.Config.Healthcheck.Interval.String()
		cliOpts.HealthRetries = uint(cc.Config.Healthcheck.Retries)
		cliOpts.HealthStartPeriod = cc.Config.Healthcheck.StartPeriod.String()
		cliOpts.HealthTimeout = cc.Config.Healthcheck.Timeout.String()
	}

	// specgen assumes the image name is arg[0]
	cmd := []string{cc.Image}
	cmd = append(cmd, cc.Config.Cmd...)
	return &cliOpts, cmd, nil
}
