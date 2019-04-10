package shortcuts

import (
	"github.com/containers/libpod/libpod"
)

// GetPodsByContext gets pods whether all, latest, or a slice of names/ids
func GetPodsByContext(all, latest bool, pods []string, runtime *libpod.Runtime) ([]*libpod.Pod, error) {
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
	for _, p := range pods {
		pod, err := runtime.LookupPod(p)
		if err != nil {
			return nil, err
		}
		outpods = append(outpods, pod)
	}
	return outpods, nil
}

// GetContainersByContext gets pods whether all, latest, or a slice of names/ids
func GetContainersByContext(all, latest bool, names []string, runtime *libpod.Runtime) (ctrs []*libpod.Container, err error) {
	var ctr *libpod.Container
	ctrs = []*libpod.Container{}

	if all {
		ctrs, err = runtime.GetAllContainers()
	} else if latest {
		ctr, err = runtime.GetLatestContainer()
		ctrs = append(ctrs, ctr)
	} else {
		for _, n := range names {
			ctr, e := runtime.LookupContainer(n)
			if e != nil && err == nil {
				err = e
			}
			ctrs = append(ctrs, ctr)
		}
	}
	return
}
