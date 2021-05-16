package shimv2

import (
	"io/ioutil"
	"os"

	apievents "github.com/containerd/containerd/api/events"
	"github.com/containerd/typeurl"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	// Hidden option, only used by shimv2 runtime to publish container events
	publishCmd = &cobra.Command{
		Use:     "publish [options]",
		Short:   "Handle shimv2 events to update container state",
		Long:    "Handle shimv2 events to update container state",
		RunE:    publish,
		Example: `podman --address [ADDRESS] publish --topic [TOPIC] --namespace [NAMESPACE]`,
		Hidden:  true,
	}
)

func runFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	topicFlagName := "topic"
	flags.String(topicFlagName, "", "")

	namespaceFlagName := "namespace"
	flags.String(namespaceFlagName, "", "")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: publishCmd,
	})

	runFlags(publishCmd)
}

func publish(cmd *cobra.Command, args []string) error {
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return errors.Wrap(err, "can not read stdin")
	}

	var any types.Any
	if err := any.Unmarshal(data); err != nil {
		return errors.Wrap(err, "can not unmarshal stdin")
	}

	e, err := typeurl.UnmarshalAny(&any)
	if err != nil {
		return errors.Wrap(err, "can not unmarshal shimv2 event")
	}

	// There are more shimv2 events but as of now only the exit event
	// produced by the shimv2 runtime will trigger a container state update
	containerID := ""
	exitCode := 0
	switch e.(type) {
	case *apievents.TaskExit:
		te, _ := e.(*apievents.TaskExit)
		containerID = te.ContainerID
		exitCode = int(te.ExitStatus)

		if err := registry.ContainerEngine().Shimv2ContainerCleanup(
			registry.Context(), containerID, exitCode); err != nil {
			return errors.Wrap(err, "can not clean up shimv2 container")
		}
	default:
		return nil
	}

	return nil
}
