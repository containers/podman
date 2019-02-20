package shortcuts

import "github.com/containers/libpod/libpod"

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
