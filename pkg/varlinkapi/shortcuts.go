package varlinkapi

import (
	"github.com/containers/podman/v2/libpod"
	"github.com/sirupsen/logrus"
)

// getPodsByContext returns a slice of pods. Note that all, latest and pods are
// mutually exclusive arguments.
func getPodsByContext(all, latest bool, pods []string, runtime *libpod.Runtime) ([]*libpod.Pod, error) {
	var outpods []*libpod.Pod
	if all {
		return runtime.GetAllPods()
	}
	if latest {
		p, err := runtime.GetLatestPod()
		if err != nil {
			return nil, err
		}
		outpods = append(outpods, p)
		return outpods, nil
	}
	var err error
	for _, p := range pods {
		pod, e := runtime.LookupPod(p)
		if e != nil {
			// Log all errors here, so callers don't need to.
			logrus.Debugf("Error looking up pod %q: %v", p, e)
			if err == nil {
				err = e
			}
		} else {
			outpods = append(outpods, pod)
		}
	}
	return outpods, err
}

// getContainersByContext gets pods whether all, latest, or a slice of names/ids
// is specified.
func getContainersByContext(all, latest bool, names []string, runtime *libpod.Runtime) (ctrs []*libpod.Container, err error) {
	var ctr *libpod.Container
	ctrs = []*libpod.Container{}

	switch {
	case all:
		ctrs, err = runtime.GetAllContainers()
	case latest:
		ctr, err = runtime.GetLatestContainer()
		ctrs = append(ctrs, ctr)
	default:
		for _, n := range names {
			ctr, e := runtime.LookupContainer(n)
			if e != nil {
				// Log all errors here, so callers don't need to.
				logrus.Debugf("Error looking up container %q: %v", n, e)
				if err == nil {
					err = e
				}
			} else {
				ctrs = append(ctrs, ctr)
			}
		}
	}
	return
}
