package generate

import (
	"fmt"
	"os"

	"github.com/containers/podman/v4/libpod"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/sirupsen/logrus"
)

func specConfigureNamespaces(s *specgen.SpecGenerator, g *generate.Generator, rt *libpod.Runtime, pod *libpod.Pod) error {
	// UTS

	hostname := s.Hostname
	if hostname == "" {
		switch {
		case s.UtsNS.NSMode == specgen.FromPod:
			hostname = pod.Hostname()
		case s.UtsNS.NSMode == specgen.FromContainer:
			utsCtr, err := rt.LookupContainer(s.UtsNS.Value)
			if err != nil {
				return fmt.Errorf("looking up container to share uts namespace with: %w", err)
			}
			hostname = utsCtr.Hostname()
		case (s.NetNS.NSMode == specgen.Host && hostname == "") || s.UtsNS.NSMode == specgen.Host:
			tmpHostname, err := os.Hostname()
			if err != nil {
				return fmt.Errorf("unable to retrieve hostname of the host: %w", err)
			}
			hostname = tmpHostname
		default:
			logrus.Debug("No hostname set; container's hostname will default to runtime default")
		}
	}

	g.RemoveHostname()
	if s.Hostname != "" || s.UtsNS.NSMode != specgen.Host {
		// Set the hostname in the OCI configuration only if specified by
		// the user or if we are creating a new UTS namespace.
		// TODO: Should we be doing this for pod or container shared
		// namespaces?
		g.SetHostname(hostname)
	}
	if _, ok := s.Env["HOSTNAME"]; !ok && s.Hostname != "" {
		g.AddProcessEnv("HOSTNAME", hostname)
	}

	return nil
}
