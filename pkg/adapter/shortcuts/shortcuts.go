package shortcuts

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
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
func GetContainersByContext(all, latest bool, names []string, runtime *libpod.Runtime) ([]*libpod.Container, error) {
	var (
		ctrs = []*libpod.Container{}
		err  error
	)

	// When running in rootless mode we cannot manage different containers and
	// user namespaces from the same context, so be sure to re-exec once for each
	// container we are dealing with.
	// What we do is to first collect all the containers we want to delete, then
	// we re-exec in each of the container namespaces and from there remove the single
	// container.
	if rootless.IsRootless() && os.Geteuid() == 0 {
		// We are in the namespace, override InputArgs with the single
		// argument that was passed down to us.
		all = false
		latest = false
		names = []string{rootless.Argument()}
	}

	if all {
		ctrs, err = runtime.GetAllContainers()
		if err != nil {
			return nil, err
		}
	}

	if latest {
		ctr, err := runtime.GetLatestContainer()
		if err != nil {
			return nil, err
		}
		ctrs = append(ctrs, ctr)
	}

	for _, c := range names {
		ctr, err := runtime.LookupContainer(c)
		if err != nil {
			return nil, err
		}
		ctrs = append(ctrs, ctr)
	}

	if rootless.IsRootless() && os.Geteuid() != 0 {
		for _, ctr := range ctrs {
			_, ret, err := joinContainerOrCreateRootlessUserNS(runtime, ctr)
			if err != nil {
				return nil, err
			}

			if ret != 0 {
				os.Exit(ret)
			}
		}
		os.Exit(0)
	}
	return ctrs, nil
}

func joinContainerOrCreateRootlessUserNS(runtime *libpod.Runtime, ctr *libpod.Container) (bool, int, error) {
	if os.Geteuid() == 0 {
		return false, 0, nil
	}

	s, err := ctr.State()
	if err != nil {
		return false, -1, err
	}

	opts := rootless.Opts{
		Argument: ctr.ID(),
	}
	if s == libpod.ContainerStateRunning || s == libpod.ContainerStatePaused {
		data, err := ioutil.ReadFile(ctr.Config().ConmonPidFile)
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot read conmon PID file %q", ctr.Config().ConmonPidFile)
		}
		conmonPid, err := strconv.Atoi(string(data))
		if err != nil {
			return false, -1, errors.Wrapf(err, "cannot parse PID %q", data)
		}
		return rootless.JoinDirectUserAndMountNSWithOpts(uint(conmonPid), &opts)
	}
	return rootless.BecomeRootInUserNSWithOpts(&opts)
}
