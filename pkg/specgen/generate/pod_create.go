package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/containers/podman/v4/pkg/specgenutil"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

func MakePod(p *entities.PodSpec, rt *libpod.Runtime) (_ *libpod.Pod, finalErr error) {
	var createdPod *libpod.Pod
	defer func() {
		if finalErr != nil && createdPod != nil {
			if _, err := rt.RemovePod(context.Background(), createdPod, true, true, nil); err != nil {
				logrus.Errorf("Removing pod: %v", err)
			}
		}
	}()
	if err := p.PodSpecGen.Validate(); err != nil {
		return nil, err
	}

	if p.PodSpecGen.ResourceLimits == nil {
		p.PodSpecGen.ResourceLimits = &specs.LinuxResources{}
	}

	if !p.PodSpecGen.NoInfra {
		imageName, err := PullOrBuildInfraImage(rt, p.PodSpecGen.InfraImage)
		if err != nil {
			return nil, err
		}
		p.PodSpecGen.InfraImage = imageName
		p.PodSpecGen.InfraContainerSpec.RawImageName = imageName
	}

	if !p.PodSpecGen.NoInfra && p.PodSpecGen.InfraContainerSpec != nil {
		var err error
		p.PodSpecGen.InfraContainerSpec, err = MapSpec(&p.PodSpecGen)
		if err != nil {
			return nil, err
		}
	}

	if !p.PodSpecGen.NoInfra {
		err := specgen.FinishThrottleDevices(p.PodSpecGen.InfraContainerSpec)
		if err != nil {
			return nil, err
		}
		if p.PodSpecGen.InfraContainerSpec.ResourceLimits != nil &&
			p.PodSpecGen.InfraContainerSpec.ResourceLimits.BlockIO != nil {
			p.PodSpecGen.ResourceLimits.BlockIO = p.PodSpecGen.InfraContainerSpec.ResourceLimits.BlockIO
		}
		err = specgen.WeightDevices(p.PodSpecGen.InfraContainerSpec)
		if err != nil {
			return nil, err
		}
		p.PodSpecGen.ResourceLimits = p.PodSpecGen.InfraContainerSpec.ResourceLimits
	}

	options, err := createPodOptions(&p.PodSpecGen)
	if err != nil {
		return nil, err
	}

	pod, err := rt.NewPod(context.Background(), p.PodSpecGen, options...)
	if err != nil {
		return nil, err
	}
	createdPod = pod

	if !p.PodSpecGen.NoInfra && p.PodSpecGen.InfraContainerSpec != nil {
		if p.PodSpecGen.InfraContainerSpec.Name == "" {
			p.PodSpecGen.InfraContainerSpec.Name = pod.ID()[:12] + "-infra"
		}
		_, err = CompleteSpec(context.Background(), rt, p.PodSpecGen.InfraContainerSpec)
		if err != nil {
			return nil, err
		}
		p.PodSpecGen.InfraContainerSpec.User = "" // infraSpec user will get incorrectly assigned via the container creation process, overwrite here
		// infra's resource limits are used as a parsing tool,
		// we do not want infra to get these resources in its cgroup
		// make sure of that here.
		p.PodSpecGen.InfraContainerSpec.ResourceLimits = nil
		p.PodSpecGen.InfraContainerSpec.WeightDevice = nil
		rtSpec, spec, opts, err := MakeContainer(context.Background(), rt, p.PodSpecGen.InfraContainerSpec, false, nil)
		if err != nil {
			return nil, err
		}

		spec.Pod = pod.ID()
		opts = append(opts, rt.WithPod(pod))
		spec.CgroupParent = pod.CgroupParent()
		infraCtr, err := ExecuteCreate(context.Background(), rt, rtSpec, spec, true, opts...)
		if err != nil {
			return nil, err
		}
		pod, err = rt.AddInfra(context.Background(), pod, infraCtr)
		if err != nil {
			return nil, err
		}
	} else {
		// SavePod is used to save the pod state and trigger a create event even if infra is not created
		err := rt.SavePod(pod)
		if err != nil {
			return nil, err
		}
	}
	return pod, nil
}

func createPodOptions(p *specgen.PodSpecGenerator) ([]libpod.PodCreateOption, error) {
	var (
		options []libpod.PodCreateOption
	)
	if !p.NoInfra {
		options = append(options, libpod.WithInfraContainer())
		if p.ShareParent == nil || (p.ShareParent != nil && *p.ShareParent) {
			options = append(options, libpod.WithPodParent())
		}
		nsOptions, err := GetNamespaceOptions(p.SharedNamespaces, p.InfraContainerSpec.NetNS.IsHost())
		if err != nil {
			return nil, err
		}
		options = append(options, nsOptions...)
		// Use pod user and infra userns only when --userns is not set to host
		if !p.InfraContainerSpec.UserNS.IsHost() && !p.InfraContainerSpec.UserNS.IsDefault() {
			options = append(options, libpod.WithPodUser())
		}
	}

	if len(p.ServiceContainerID) > 0 {
		options = append(options, libpod.WithServiceContainer(p.ServiceContainerID))
	}

	if len(p.CgroupParent) > 0 {
		options = append(options, libpod.WithPodCgroupParent(p.CgroupParent))
	}
	if len(p.Labels) > 0 {
		options = append(options, libpod.WithPodLabels(p.Labels))
	}
	if len(p.Name) > 0 {
		options = append(options, libpod.WithPodName(p.Name))
	}
	if p.PodCreateCommand != nil {
		options = append(options, libpod.WithPodCreateCommand(p.PodCreateCommand))
	}

	if len(p.Hostname) > 0 {
		options = append(options, libpod.WithPodHostname(p.Hostname))
	}

	if p.ResourceLimits != nil {
		options = append(options, libpod.WithPodResources(*p.ResourceLimits))
	}

	options = append(options, libpod.WithPodExitPolicy(p.ExitPolicy))
	options = append(options, libpod.WithPodRestartPolicy(p.RestartPolicy))
	if p.RestartRetries != nil {
		options = append(options, libpod.WithPodRestartRetries(*p.RestartRetries))
	}

	return options, nil
}

// MapSpec modifies the already filled Infra specgenerator,
// replacing necessary values with those specified in pod creation
func MapSpec(p *specgen.PodSpecGenerator) (*specgen.SpecGenerator, error) {
	if len(p.PortMappings) > 0 {
		ports, err := ParsePortMapping(p.PortMappings, nil)
		if err != nil {
			return nil, err
		}
		p.InfraContainerSpec.PortMappings = ports
	}
	switch p.NetNS.NSMode {
	case specgen.Default, "":
		if p.NoInfra {
			logrus.Debugf("No networking because the infra container is missing")
			break
		}
	case specgen.Bridge:
		p.InfraContainerSpec.NetNS.NSMode = specgen.Bridge
		logrus.Debugf("Pod using bridge network mode")
	case specgen.Private:
		p.InfraContainerSpec.NetNS.NSMode = specgen.Private
		logrus.Debugf("Pod will use default network mode")
	case specgen.Host:
		logrus.Debugf("Pod will use host networking")
		if len(p.InfraContainerSpec.PortMappings) > 0 ||
			len(p.InfraContainerSpec.Networks) > 0 ||
			p.InfraContainerSpec.NetNS.NSMode == specgen.NoNetwork {
			return nil, fmt.Errorf("cannot set host network if network-related configuration is specified: %w", define.ErrInvalidArg)
		}
		p.InfraContainerSpec.NetNS.NSMode = specgen.Host
	case specgen.Slirp:
		logrus.Debugf("Pod will use slirp4netns")
		if p.InfraContainerSpec.NetNS.NSMode != specgen.Host {
			p.InfraContainerSpec.NetworkOptions = p.NetworkOptions
			p.InfraContainerSpec.NetNS.NSMode = specgen.Slirp
		}
	case specgen.Pasta:
		logrus.Debugf("Pod will use pasta")
		if p.InfraContainerSpec.NetNS.NSMode != specgen.Host {
			p.InfraContainerSpec.NetworkOptions = p.NetworkOptions
			p.InfraContainerSpec.NetNS.NSMode = specgen.Pasta
		}
	case specgen.Path:
		logrus.Debugf("Pod will use namespace path networking")
		p.InfraContainerSpec.NetNS.NSMode = specgen.Path
		p.InfraContainerSpec.NetNS.Value = p.PodNetworkConfig.NetNS.Value
	case specgen.NoNetwork:
		logrus.Debugf("Pod will not use networking")
		if len(p.InfraContainerSpec.PortMappings) > 0 ||
			len(p.InfraContainerSpec.Networks) > 0 ||
			p.InfraContainerSpec.NetNS.NSMode == specgen.Host {
			return nil, fmt.Errorf("cannot disable pod network if network-related configuration is specified: %w", define.ErrInvalidArg)
		}
		p.InfraContainerSpec.NetNS.NSMode = specgen.NoNetwork
	default:
		return nil, fmt.Errorf("pods presently do not support network mode %s", p.NetNS.NSMode)
	}

	if len(p.InfraCommand) > 0 {
		p.InfraContainerSpec.Entrypoint = p.InfraCommand
	}

	if len(p.HostAdd) > 0 {
		p.InfraContainerSpec.HostAdd = p.HostAdd
	}
	if len(p.DNSServer) > 0 {
		var dnsServers []net.IP
		dnsServers = append(dnsServers, p.DNSServer...)

		p.InfraContainerSpec.DNSServers = dnsServers
	}
	if len(p.DNSOption) > 0 {
		p.InfraContainerSpec.DNSOptions = p.DNSOption
	}
	if len(p.DNSSearch) > 0 {
		p.InfraContainerSpec.DNSSearch = p.DNSSearch
	}
	if p.NoManageResolvConf {
		p.InfraContainerSpec.UseImageResolvConf = true
	}
	if len(p.Networks) > 0 {
		p.InfraContainerSpec.Networks = p.Networks
	}
	// deprecated cni networks for api users
	if len(p.CNINetworks) > 0 {
		p.InfraContainerSpec.CNINetworks = p.CNINetworks
	}
	if p.NoManageHosts {
		p.InfraContainerSpec.UseImageHosts = p.NoManageHosts
	}

	if len(p.InfraConmonPidFile) > 0 {
		p.InfraContainerSpec.ConmonPidFile = p.InfraConmonPidFile
	}

	if p.Sysctl != nil && len(p.Sysctl) > 0 {
		p.InfraContainerSpec.Sysctl = p.Sysctl
	}

	p.InfraContainerSpec.Image = p.InfraImage
	return p.InfraContainerSpec, nil
}

func PodConfigToSpec(rt *libpod.Runtime, spec *specgen.PodSpecGenerator, infraOptions *entities.ContainerCreateOptions, id string) (p *libpod.Pod, err error) {
	pod, err := rt.LookupPod(id)
	if err != nil {
		return nil, err
	}

	infraSpec := &specgen.SpecGenerator{}
	if pod.HasInfraContainer() {
		infraID, err := pod.InfraContainerID()
		if err != nil {
			return nil, err
		}
		_, _, err = ConfigToSpec(rt, infraSpec, infraID)
		if err != nil {
			return nil, err
		}

		infraSpec.Hostname = ""
		infraSpec.CgroupParent = ""
		infraSpec.Pod = "" // remove old pod...
		infraOptions.IsClone = true
		infraOptions.IsInfra = true

		n := infraSpec.Name
		_, err = rt.LookupContainer(n + "-clone")
		if err == nil { // if we found a ctr with this name, set it so the below switch can tell
			n += "-clone"
		}

		switch {
		case strings.Contains(n, "-clone"):
			ind := strings.Index(n, "-clone") + 6
			num, err := strconv.Atoi(n[ind:])
			if num == 0 && err != nil { // clone1 is hard to get with this logic, just check for it here.
				_, err = rt.LookupContainer(n + "1")
				if err != nil {
					infraSpec.Name = n + "1"
					break
				}
			} else {
				n = n[0:ind]
			}
			err = nil
			count := num
			for err == nil {
				count++
				tempN := n + strconv.Itoa(count)
				_, err = rt.LookupContainer(tempN)
			}
			n += strconv.Itoa(count)
			infraSpec.Name = n
		default:
			infraSpec.Name = n + "-clone"
		}

		err = specgenutil.FillOutSpecGen(infraSpec, infraOptions, []string{})
		if err != nil {
			return nil, err
		}

		out, err := CompleteSpec(context.Background(), rt, infraSpec)
		if err != nil {
			return nil, err
		}

		// Print warnings
		if len(out) > 0 {
			for _, w := range out {
				fmt.Println("Could not properly complete the spec as expected:")
				fmt.Fprintf(os.Stderr, "%s\n", w)
			}
		}

		spec.InfraContainerSpec = infraSpec
		matching, err := json.Marshal(infraSpec)
		if err != nil {
			return nil, err
		}

		// track name before unmarshal so we do not overwrite w/ infra
		name := spec.Name
		err = json.Unmarshal(matching, spec)
		if err != nil {
			return nil, err
		}

		spec.Name = name
	}

	// need to reset hostname, name etc of both pod and infra
	spec.Hostname = ""

	if len(spec.InfraContainerSpec.Image) > 0 {
		spec.InfraImage = spec.InfraContainerSpec.Image
	}
	return pod, nil
}
