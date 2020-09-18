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

// containerInfo contains data required for generating a container's systemd
// unit file.
type containerInfo struct {
	// ServiceName of the systemd service.
	ServiceName string
	// Name or ID of the container.
	ContainerNameOrID string
	// StopTimeout sets the timeout Podman waits before killing the container
	// during service stop.
	StopTimeout uint
	// RestartPolicy of the systemd unit (e.g., no, on-failure, always).
	RestartPolicy string
	// PIDFile of the service. Required for forking services. Must point to the
	// PID of the associated conmon process.
	PIDFile string
	// ContainerIDFile to be used in the unit.
	ContainerIDFile string
	// GenerateTimestamp, if set the generated unit file has a time stamp.
	GenerateTimestamp bool
	// BoundToServices are the services this service binds to.  Note that this
	// service runs after them.
	BoundToServices []string
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
	// EnvVariable is generate.EnvVariable and must not be set.
	EnvVariable string
	// ExecStartPre of the unit.
	ExecStartPre string
	// ExecStart of the unit.
	ExecStart string
	// ExecStop of the unit.
	ExecStop string
	// ExecStopPost of the unit.
	ExecStopPost string

	// If not nil, the container is part of the pod.  We can use the
	// podInfo to extract the relevant data.
	pod *podInfo
}

const containerTemplate = headerTemplate + `
{{- if .BoundToServices}}
BindsTo={{- range $index, $value := .BoundToServices -}}{{if $index}} {{end}}{{ $value }}.service{{end}}
After={{- range $index, $value := .BoundToServices -}}{{if $index}} {{end}}{{ $value }}.service{{end}}
{{- end}}

[Service]
Environment={{.EnvVariable}}=%n
Restart={{.RestartPolicy}}
{{- if .ExecStartPre}}
ExecStartPre={{.ExecStartPre}}
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

// ContainerUnit generates a systemd unit for the specified container.  Based
// on the options, the return value might be the entire unit or a file it has
// been written to.
func ContainerUnit(ctr *libpod.Container, options entities.GenerateSystemdOptions) (string, string, error) {
	info, err := generateContainerInfo(ctr, options)
	if err != nil {
		return "", "", err
	}
	content, err := executeContainerTemplate(info, options)
	if err != nil {
		return "", "", err
	}
	return info.ServiceName, content, nil
}

func generateContainerInfo(ctr *libpod.Container, options entities.GenerateSystemdOptions) (*containerInfo, error) {
	timeout := ctr.StopTimeout()
	if options.StopTimeout != nil {
		timeout = *options.StopTimeout
	}

	config := ctr.Config()
	conmonPidFile := config.ConmonPidFile
	if conmonPidFile == "" {
		return nil, errors.Errorf("conmon PID file path is empty, try to recreate the container with --conmon-pidfile flag")
	}

	createCommand := []string{}
	if config.CreateCommand != nil {
		createCommand = config.CreateCommand
	} else if options.New {
		return nil, errors.Errorf("cannot use --new on container %q: no create command found", ctr.ID())
	}

	nameOrID, serviceName := containerServiceName(ctr, options)

	info := containerInfo{
		ServiceName:       serviceName,
		ContainerNameOrID: nameOrID,
		RestartPolicy:     options.RestartPolicy,
		PIDFile:           conmonPidFile,
		StopTimeout:       timeout,
		GenerateTimestamp: true,
		CreateCommand:     createCommand,
	}

	return &info, nil
}

// containerServiceName returns the nameOrID and the service name of the
// container.
func containerServiceName(ctr *libpod.Container, options entities.GenerateSystemdOptions) (string, string) {
	nameOrID := ctr.ID()
	if options.Name {
		nameOrID = ctr.Name()
	}
	serviceName := fmt.Sprintf("%s%s%s", options.ContainerPrefix, options.Separator, nameOrID)
	return nameOrID, serviceName
}

// executeContainerTemplate executes the container template on the specified
// containerInfo.  Note that the containerInfo is also post processed and
// completed, which allows for an easier unit testing.
func executeContainerTemplate(info *containerInfo, options entities.GenerateSystemdOptions) (string, error) {
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
	info.ExecStart = "{{.Executable}} start {{.ContainerNameOrID}}"
	info.ExecStop = "{{.Executable}} stop {{if (ge .StopTimeout 0)}}-t {{.StopTimeout}}{{end}} {{.ContainerNameOrID}}"
	info.ExecStopPost = "{{.Executable}} stop {{if (ge .StopTimeout 0)}}-t {{.StopTimeout}}{{end}} {{.ContainerNameOrID}}"

	// Assemble the ExecStart command when creating a new container.
	//
	// Note that we cannot catch all corner cases here such that users
	// *must* manually check the generated files.  A container might have
	// been created via a Python script, which would certainly yield an
	// invalid `info.CreateCommand`.  Hence, we're doing a best effort unit
	// generation and don't try aiming at completeness.
	if options.New {
		info.PIDFile = "%t/" + info.ServiceName + ".pid"
		info.ContainerIDFile = "%t/" + info.ServiceName + ".ctr-id"
		// The create command must at least have three arguments:
		// 	/usr/bin/podman run $IMAGE
		index := 2
		if info.CreateCommand[1] == "container" {
			index = 3
		}
		if len(info.CreateCommand) < index+1 {
			return "", errors.Errorf("container's create command is too short or invalid: %v", info.CreateCommand)
		}
		// We're hard-coding the first five arguments and append the
		// CreateCommand with a stripped command and subcomand.
		startCommand := []string{
			info.Executable,
			"run",
			"--conmon-pidfile", "{{.PIDFile}}",
			"--cidfile", "{{.ContainerIDFile}}",
			"--cgroups=no-conmon",
		}
		// If the container is in a pod, make sure that the
		// --pod-id-file is set correctly.
		if info.pod != nil {
			podFlags := []string{"--pod-id-file", info.pod.PodIDFile}
			startCommand = append(startCommand, podFlags...)
			info.CreateCommand = filterPodFlags(info.CreateCommand)
		}

		// Presence check for certain flags/options.
		hasDetachParam := false
		hasNameParam := false
		hasReplaceParam := false
		for _, p := range info.CreateCommand[index:] {
			switch p {
			case "--detach", "-d":
				hasDetachParam = true
			case "--name":
				hasNameParam = true
			case "--replace":
				hasReplaceParam = true
			}
			if strings.HasPrefix(p, "--name=") {
				hasNameParam = true
			}
		}

		if !hasDetachParam {
			// Enforce detaching
			//
			// since we use systemd `Type=forking` service @see
			// https://www.freedesktop.org/software/systemd/man/systemd.service.html#Type=
			// when we generated systemd service file with the
			// --new param, `ExecStart` will have `/usr/bin/podman
			// run ...` if `info.CreateCommand` has no `-d` or
			// `--detach` param, podman will run the container in
			// default attached mode, as a result, `systemd start`
			// will wait the `podman run` command exit until failed
			// with timeout error.
			startCommand = append(startCommand, "-d")
		}
		if hasNameParam && !hasReplaceParam {
			// Enforce --replace for named containers.  This will
			// make systemd units more robuts as it allows them to
			// start after system crashes (see
			// github.com/containers/podman/issues/5485).
			startCommand = append(startCommand, "--replace")
		}
		startCommand = append(startCommand, info.CreateCommand[index:]...)
		startCommand = quoteArguments(startCommand)

		info.ExecStartPre = "/bin/rm -f {{.PIDFile}} {{.ContainerIDFile}}"
		info.ExecStart = strings.Join(startCommand, " ")
		info.ExecStop = "{{.Executable}} stop --ignore --cidfile {{.ContainerIDFile}} {{if (ge .StopTimeout 0)}}-t {{.StopTimeout}}{{end}}"
		info.ExecStopPost = "{{.Executable}} rm --ignore -f --cidfile {{.ContainerIDFile}}"
	}

	if info.PodmanVersion == "" {
		info.PodmanVersion = version.Version.String()
	}
	if info.GenerateTimestamp {
		info.TimeStamp = fmt.Sprintf("%v", time.Now().Format(time.UnixDate))
	}

	// Sort the slices to assure a deterministic output.
	sort.Strings(info.BoundToServices)

	// Generate the template and compile it.
	//
	// Note that we need a two-step generation process to allow for fields
	// embedding other fields.  This way we can replace `A -> B -> C` and
	// make the code easier to maintain at the cost of a slightly slower
	// generation.  That's especially needed for embedding the PID and ID
	// files in other fields which will eventually get replaced in the 2nd
	// template execution.
	templ, err := template.New("container_template").Parse(containerTemplate)
	if err != nil {
		return "", errors.Wrap(err, "error parsing systemd service template")
	}

	var buf bytes.Buffer
	if err := templ.Execute(&buf, info); err != nil {
		return "", err
	}

	// Now parse the generated template (i.e., buf) and execute it.
	templ, err = template.New("container_template").Parse(buf.String())
	if err != nil {
		return "", errors.Wrap(err, "error parsing systemd service template")
	}

	buf = bytes.Buffer{}
	if err := templ.Execute(&buf, info); err != nil {
		return "", err
	}

	return buf.String(), nil
}
