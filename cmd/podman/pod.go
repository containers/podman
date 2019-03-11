package main

import (
	"os"

	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/containers/libpod/pkg/adapter"
	"github.com/containers/libpod/pkg/rootless"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	podDescription = `Pods are a group of one or more containers sharing the same network, pid and ipc namespaces.`
)
var podCommand = cliconfig.PodmanCommand{
	Command: &cobra.Command{
		Use:   "pod",
		Short: "Manage pods",
		Long:  podDescription,
		RunE:  commandRunE(),
	},
}

//podSubCommands are implemented both in local and remote clients
var podSubCommands = []*cobra.Command{
	_podCreateCommand,
	_podExistsCommand,
	_podInspectCommand,
	_podKillCommand,
	_podPauseCommand,
	_podPsCommand,
	_podRestartCommand,
	_podRmCommand,
	_podStartCommand,
	_podStatsCommand,
	_podStopCommand,
	_podTopCommand,
	_podUnpauseCommand,
}

func joinPodNS(runtime *adapter.LocalRuntime, all, latest bool, inputArgs []string) ([]string, bool, bool, error) {
	if rootless.IsRootless() {
		if os.Geteuid() == 0 {
			return []string{rootless.Argument()}, false, false, nil
		} else {
			var err error
			var pods []*adapter.Pod
			if all {
				pods, err = runtime.GetAllPods()
				if err != nil {
					return nil, false, false, errors.Wrapf(err, "unable to get pods")
				}
			} else if latest {
				pod, err := runtime.GetLatestPod()
				if err != nil {
					return nil, false, false, errors.Wrapf(err, "unable to get latest pod")
				}
				pods = append(pods, pod)
			} else {
				for _, i := range inputArgs {
					pod, err := runtime.LookupPod(i)
					if err != nil {
						return nil, false, false, errors.Wrapf(err, "unable to lookup pod %s", i)
					}
					pods = append(pods, pod)
				}
			}
			for _, p := range pods {
				_, ret, err := runtime.JoinOrCreateRootlessPod(p)
				if err != nil {
					return nil, false, false, err
				}
				if ret != 0 {
					os.Exit(ret)
				}
			}
			os.Exit(0)
		}
	}
	return inputArgs, all, latest, nil
}

func init() {
	podCommand.AddCommand(podSubCommands...)
	podCommand.SetHelpTemplate(HelpTemplate())
	podCommand.SetUsageTemplate(UsageTemplate())
}
