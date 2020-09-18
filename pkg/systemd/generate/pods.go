package generate

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/containers/podman/v2/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// podInfo contains data required for generating a pod's systemd
// unit file.
type podInfo struct {
	// ServiceName of the systemd service.
	ServiceName string
	// Name or ID of the infra container.
	InfraNameOrID string
	// StopTimeout sets the timeout Podman waits before killing the container
	// during service stop.
	StopTimeout uint
	// RestartPolicy of the systemd unit (e.g., no, on-failure, always).
	RestartPolicy string
	// PIDFile of the service. Required for forking services. Must point to the
	// PID of the associated conmon process.
	PIDFile string
	// PodIDFile of the unit.
	PodIDFile string
	// GenerateTimestamp, if set the generated unit file has a time stamp.
	GenerateTimestamp bool
	// RequiredServices are services this service requires. Note that this
	// service runs before them.
	RequiredServices []string
	// PodmanVersion for the header. Will be set internally. Will be auto-filled
	// if left empty.
	PodmanVersion string
	// Executable is the path to the podman executable. Will be auto-filled if
	// left empty.
	Executable string
	// TimeStamp at the time of creating the unit file. Will be set internally.
	TimeStamp string
	// CreateCommand is the full command plus arguments of the process the
	// container has been created with.
	CreateCommand []string
	// PodCreateCommand - a post-processed variant of CreateCommand to use
	// when creating the pod.
	PodCreateCommand string
	// EnvVariable is generate.EnvVariable and must not be set.
	EnvVariable string
	// ExecStartPre1 of the unit.
	ExecStartPre1 string
	// ExecStartPre2 of the unit.
	ExecStartPre2 string
	// ExecStart of the unit.
	ExecStart string
	// ExecStop of the unit.
	ExecStop string
	// ExecStopPost of the unit.
	ExecStopPost string
}

const podTemplate = headerTemplate + `Requires={{- range $index, $value := .RequiredServices -}}{{if $index}} {{end}}{{ $value }}.service{{end}}
Before={{- range $index, $value := .RequiredServices -}}{{if $index}} {{end}}{{ $value }}.service{{end}}

[Service]
Environment={{.EnvVariable}}=%n
Restart={{.RestartPolicy}}
{{- if .ExecStartPre1}}
ExecStartPre={{.ExecStartPre1}}
{{- end}}
{{- if .ExecStartPre2}}
ExecStartPre={{.ExecStartPre2}}
{{- end}}
ExecStart={{.ExecStart}}
ExecStop={{.ExecStop}}
ExecStopPost={{.ExecStopPost}}
PIDFile={{.PIDFile}}
KillMode=none
Type=forking

[Install]
WantedBy=multi-user.target default.target
`

// PodUnits generates systemd units for the specified pod and its containers.
// Based on the options, the return value might be the content of all units or
// the files they been written to.
func PodUnits(pod *libpod.Pod, options entities.GenerateSystemdOptions) (map[string]string, error) {
	// Error out if the pod has no infra container, which we require to be the
	// main service.
	if !pod.HasInfraContainer() {
		return nil, errors.Errorf("error generating systemd unit files: Pod %q has no infra container", pod.Name())
	}

	podInfo, err := generatePodInfo(pod, options)
	if err != nil {
		return nil, err
	}

	infraID, err := pod.InfraContainerID()
	if err != nil {
		return nil, err
	}

	// Compute the container-dependency graph for the Pod.
	containers, err := pod.AllContainers()
	if err != nil {
		return nil, err
	}
	if len(containers) == 0 {
		return nil, errors.Errorf("error generating systemd unit files: Pod %q has no containers", pod.Name())
	}
	graph, err := libpod.BuildContainerGraph(containers)
	if err != nil {
		return nil, err
	}

	// Traverse the dependency graph and create systemdgen.containerInfo's for
	// each container.
	containerInfos := []*containerInfo{}
	for ctr, dependencies := range graph.DependencyMap() {
		// Skip the infra container as we already generated it.
		if ctr.ID() == infraID {
			continue
		}
		ctrInfo, err := generateContainerInfo(ctr, options)
		if err != nil {
			return nil, err
		}
		// Now add the container's dependencies and at the container as a
		// required service of the infra container.
		for _, dep := range dependencies {
			if dep.ID() == infraID {
				ctrInfo.BoundToServices = append(ctrInfo.BoundToServices, podInfo.ServiceName)
			} else {
				_, serviceName := containerServiceName(dep, options)
				ctrInfo.BoundToServices = append(ctrInfo.BoundToServices, serviceName)
			}
		}
		podInfo.RequiredServices = append(podInfo.RequiredServices, ctrInfo.ServiceName)
		containerInfos = append(containerInfos, ctrInfo)
	}

	units := map[string]string{}
	// Now generate the systemd service for all containers.
	out, err := executePodTemplate(podInfo, options)
	if err != nil {
		return nil, err
	}
	units[podInfo.ServiceName] = out
	for _, info := range containerInfos {
		info.pod = podInfo
		out, err := executeContainerTemplate(info, options)
		if err != nil {
			return nil, err
		}
		units[info.ServiceName] = out
	}

	return units, nil
}

func generatePodInfo(pod *libpod.Pod, options entities.GenerateSystemdOptions) (*podInfo, error) {
	// Generate a systemdgen.containerInfo for the infra container. This
	// containerInfo acts as the main service of the pod.
	infraCtr, err := pod.InfraContainer()
	if err != nil {
		return nil, errors.Wrap(err, "could not find infra container")
	}

	timeout := infraCtr.StopTimeout()
	if options.StopTimeout != nil {
		timeout = *options.StopTimeout
	}

	config := infraCtr.Config()
	conmonPidFile := config.ConmonPidFile
	if conmonPidFile == "" {
		return nil, errors.Errorf("conmon PID file path is empty, try to recreate the container with --conmon-pidfile flag")
	}

	createCommand := pod.CreateCommand()
	if options.New && len(createCommand) == 0 {
		return nil, errors.Errorf("cannot use --new on pod %q: no create command found", pod.ID())
	}

	nameOrID := pod.ID()
	ctrNameOrID := infraCtr.ID()
	if options.Name {
		nameOrID = pod.Name()
		ctrNameOrID = infraCtr.Name()
	}
	serviceName := fmt.Sprintf("%s%s%s", options.PodPrefix, options.Separator, nameOrID)

	info := podInfo{
		ServiceName:       serviceName,
		InfraNameOrID:     ctrNameOrID,
		RestartPolicy:     options.RestartPolicy,
		PIDFile:           conmonPidFile,
		StopTimeout:       timeout,
		GenerateTimestamp: true,
		CreateCommand:     createCommand,
	}
	return &info, nil
}

// executePodTemplate executes the pod template on the specified podInfo.  Note
// that the podInfo is also post processed and completed, which allows for an
// easier unit testing.
func executePodTemplate(info *podInfo, options entities.GenerateSystemdOptions) (string, error) {
	if err := validateRestartPolicy(info.RestartPolicy); err != nil {
		return "", err
	}

	// Make sure the executable is set.
	if info.Executable == "" {
		executable, err := os.Executable()
		if err != nil {
			executable = "/usr/bin/podman"
			logrus.Warnf("Could not obtain podman executable location, using default %s", executable)
		}
		info.Executable = executable
	}

	info.EnvVariable = EnvVariable
	info.ExecStart = "{{.Executable}} start {{.InfraNameOrID}}"
	info.ExecStop = "{{.Executable}} stop {{if (ge .StopTimeout 0)}}-t {{.StopTimeout}}{{end}} {{.InfraNameOrID}}"
	info.ExecStopPost = "{{.Executable}} stop {{if (ge .StopTimeout 0)}}-t {{.StopTimeout}}{{end}} {{.InfraNameOrID}}"

	// Assemble the ExecStart command when creating a new pod.
	//
	// Note that we cannot catch all corner cases here such that users
	// *must* manually check the generated files.  A pod might have been
	// created via a Python script, which would certainly yield an invalid
	// `info.CreateCommand`.  Hence, we're doing a best effort unit
	// generation and don't try aiming at completeness.
	if options.New {
		info.PIDFile = "%t/" + info.ServiceName + ".pid"
		info.PodIDFile = "%t/" + info.ServiceName + ".pod-id"

		podCreateIndex := 0
		var podRootArgs, podCreateArgs []string
		switch len(info.CreateCommand) {
		case 0, 1, 2:
			return "", errors.Errorf("pod does not appear to be created via `podman pod create`: %v", info.CreateCommand)
		default:
			// Make sure that pod was created with `pod create` and
			// not something else, such as `run --pod new`.
			for i := 1; i < len(info.CreateCommand); i++ {
				if info.CreateCommand[i-1] == "pod" && info.CreateCommand[i] == "create" {
					podCreateIndex = i
					break
				}
			}
			if podCreateIndex == 0 {
				return "", errors.Errorf("pod does not appear to be created via `podman pod create`: %v", info.CreateCommand)
			}
			podRootArgs = info.CreateCommand[0 : podCreateIndex-2]
			podCreateArgs = filterPodFlags(info.CreateCommand[podCreateIndex+1:])
		}
		// We're hard-coding the first five arguments and append the
		// CreateCommand with a stripped command and subcomand.
		startCommand := []string{info.Executable}
		startCommand = append(startCommand, podRootArgs...)
		startCommand = append(startCommand,
			[]string{"pod", "create",
				"--infra-conmon-pidfile", "{{.PIDFile}}",
				"--pod-id-file", "{{.PodIDFile}}"}...)

		// Presence check for certain flags/options.
		hasNameParam := false
		hasReplaceParam := false
		for _, p := range podCreateArgs {
			switch p {
			case "--name":
				hasNameParam = true
			case "--replace":
				hasReplaceParam = true
			}
		}
		if hasNameParam && !hasReplaceParam {
			podCreateArgs = append(podCreateArgs, "--replace")
		}

		startCommand = append(startCommand, podCreateArgs...)
		startCommand = quoteArguments(startCommand)

		info.ExecStartPre1 = "/bin/rm -f {{.PIDFile}} {{.PodIDFile}}"
		info.ExecStartPre2 = strings.Join(startCommand, " ")
		info.ExecStart = "{{.Executable}} pod start --pod-id-file {{.PodIDFile}}"
		info.ExecStop = "{{.Executable}} pod stop --ignore --pod-id-file {{.PodIDFile}} {{if (ge .StopTimeout 0)}}-t {{.StopTimeout}}{{end}}"
		info.ExecStopPost = "{{.Executable}} pod rm --ignore -f --pod-id-file {{.PodIDFile}}"
	}
	if info.PodmanVersion == "" {
		info.PodmanVersion = version.Version.String()
	}
	if info.GenerateTimestamp {
		info.TimeStamp = fmt.Sprintf("%v", time.Now().Format(time.UnixDate))
	}

	// Sort the slices to assure a deterministic output.
	sort.Strings(info.RequiredServices)

	// Generate the template and compile it.
	//
	// Note that we need a two-step generation process to allow for fields
	// embedding other fields.  This way we can replace `A -> B -> C` and
	// make the code easier to maintain at the cost of a slightly slower
	// generation.  That's especially needed for embedding the PID and ID
	// files in other fields which will eventually get replaced in the 2nd
	// template execution.
	templ, err := template.New("pod_template").Parse(podTemplate)
	if err != nil {
		return "", errors.Wrap(err, "error parsing systemd service template")
	}

	var buf bytes.Buffer
	if err := templ.Execute(&buf, info); err != nil {
		return "", err
	}

	// Now parse the generated template (i.e., buf) and execute it.
	templ, err = template.New("pod_template").Parse(buf.String())
	if err != nil {
		return "", errors.Wrap(err, "error parsing systemd service template")
	}

	buf = bytes.Buffer{}
	if err := templ.Execute(&buf, info); err != nil {
		return "", err
	}

	return buf.String(), nil
}
