package generate

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/containers/libpod/libpod"
	"github.com/containers/libpod/pkg/domain/entities"
	"github.com/containers/libpod/version"
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
	// GenerateTimestamp, if set the generated unit file has a time stamp.
	GenerateTimestamp bool
	// BoundToServices are the services this service binds to.  Note that this
	// service runs after them.
	BoundToServices []string
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
	// New controls if a new container is created or if an existing one is started.
	New bool
	// CreateCommand is the full command plus arguments of the process the
	// container has been created with.
	CreateCommand []string
	// RunCommand is a post-processed variant of CreateCommand and used for
	// the ExecStart field in generic unit files.
	RunCommand string
	// EnvVariable is generate.EnvVariable and must not be set.
	EnvVariable string
}

const containerTemplate = headerTemplate + `
{{- if .BoundToServices}}
RefuseManualStart=yes
RefuseManualStop=yes
BindsTo={{- range $index, $value := .BoundToServices -}}{{if $index}} {{end}}{{ $value }}.service{{end}}
After={{- range $index, $value := .BoundToServices -}}{{if $index}} {{end}}{{ $value }}.service{{end}}
{{- end}}
{{- if .RequiredServices}}
Requires={{- range $index, $value := .RequiredServices -}}{{if $index}} {{end}}{{ $value }}.service{{end}}
Before={{- range $index, $value := .RequiredServices -}}{{if $index}} {{end}}{{ $value }}.service{{end}}
{{- end}}

[Service]
Environment={{.EnvVariable}}=%n
Restart={{.RestartPolicy}}
{{- if .New}}
ExecStartPre=/usr/bin/rm -f %t/%n-pid %t/%n-ctr-id
ExecStart={{.RunCommand}}
ExecStop={{.Executable}} stop --ignore --cidfile %t/%n-ctr-id {{if (ge .StopTimeout 0)}}-t {{.StopTimeout}}{{end}}
ExecStopPost={{.Executable}} rm --ignore -f --cidfile %t/%n-ctr-id
PIDFile=%t/%n-pid
{{- else}}
ExecStart={{.Executable}} start {{.ContainerNameOrID}}
ExecStop={{.Executable}} stop {{if (ge .StopTimeout 0)}}-t {{.StopTimeout}}{{end}} {{.ContainerNameOrID}}
PIDFile={{.PIDFile}}
{{- end}}
KillMode=none
Type=forking

[Install]
WantedBy=multi-user.target default.target`

// ContainerUnit generates a systemd unit for the specified container.  Based
// on the options, the return value might be the entire unit or a file it has
// been written to.
func ContainerUnit(ctr *libpod.Container, options entities.GenerateSystemdOptions) (string, error) {
	info, err := generateContainerInfo(ctr, options)
	if err != nil {
		return "", err
	}
	return createContainerSystemdUnit(info, options)
}

// createContainerSystemdUnit creates a systemd unit file for a container.
func createContainerSystemdUnit(info *containerInfo, options entities.GenerateSystemdOptions) (string, error) {
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

	// Assemble the ExecStart command when creating a new container.
	//
	// Note that we cannot catch all corner cases here such that users
	// *must* manually check the generated files.  A container might have
	// been created via a Python script, which would certainly yield an
	// invalid `info.CreateCommand`.  Hence, we're doing a best effort unit
	// generation and don't try aiming at completeness.
	if options.New {
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
		command := []string{
			info.Executable,
			"run",
			"--conmon-pidfile", "%t/%n-pid",
			"--cidfile", "%t/%n-ctr-id",
			"--cgroups=no-conmon",
		}

		// Enforce detaching
		//
		// since we use systemd `Type=forking` service
		// @see https://www.freedesktop.org/software/systemd/man/systemd.service.html#Type=
		// when we generated systemd service file with the --new param,
		// `ExecStart` will have `/usr/bin/podman run ...`
		// if `info.CreateCommand` has no `-d` or `--detach` param,
		// podman will run the container in default attached mode,
		// as a result, `systemd start` will wait the `podman run` command exit until failed with timeout error.
		hasDetachParam := false
		for _, p := range info.CreateCommand[index:] {
			if p == "--detach" || p == "-d" {
				hasDetachParam = true
			}
		}
		if !hasDetachParam {
			command = append(command, "-d")
		}

		command = append(command, info.CreateCommand[index:]...)
		info.RunCommand = strings.Join(command, " ")
		info.New = true
	}

	if info.PodmanVersion == "" {
		info.PodmanVersion = version.Version
	}
	if info.GenerateTimestamp {
		info.TimeStamp = fmt.Sprintf("%v", time.Now().Format(time.UnixDate))
	}

	// Sort the slices to assure a deterministic output.
	sort.Strings(info.RequiredServices)
	sort.Strings(info.BoundToServices)

	// Generate the template and compile it.
	templ, err := template.New("systemd_service_file").Parse(containerTemplate)
	if err != nil {
		return "", errors.Wrap(err, "error parsing systemd service template")
	}

	var buf bytes.Buffer
	if err := templ.Execute(&buf, info); err != nil {
		return "", err
	}

	if !options.Files {
		return buf.String(), nil
	}

	buf.WriteByte('\n')
	cwd, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "error getting current working directory")
	}
	path := filepath.Join(cwd, fmt.Sprintf("%s.service", info.ServiceName))
	if err := ioutil.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return "", errors.Wrap(err, "error generating systemd unit")
	}
	return path, nil
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
