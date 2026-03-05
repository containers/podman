package libpod

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/containers/common/pkg/capabilities"
	"github.com/containers/podman/v4/pkg/lookup"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// ExecContainer executes a command in a running container
// NOTE: This function is defined in oci_conmon_exec_common.go - removed duplicate

// ExecContainerHTTP executes a new command in an existing container and
// forwards its standard streams over an attach
// NOTE: This function is defined in oci_conmon_exec_common.go - removed duplicate

// ExecContainerDetached executes a command in a running container, but does
// not attach to it.
// NOTE: This function is defined in oci_conmon_exec_common.go - removed duplicate

// ExecAttachResize resizes the TTY of the given exec session.
// NOTE: This function is defined in oci_conmon_exec_common.go - removed duplicate

// ExecStopContainer stops a given exec session in a running container.
// NOTE: This function is defined in oci_conmon_exec_common.go - removed duplicate

// ExecUpdateStatus checks if the given exec session is still running.
// NOTE: This function is defined in oci_conmon_exec_common.go - removed duplicate

// ExecAttachSocketPath is the path to a container's exec session attach socket.
// NOTE: This function is defined in oci_conmon_exec_common.go - removed duplicate

// execPipes.cleanup is defined in oci_conmon_exec_common.go - removed duplicate

// Start an exec session's conmon parent from the given options.
// NOTE: This function is defined in oci_conmon_exec_common.go - removed duplicate

// prepareProcessExec returns the path of the process.json used in runc exec -p
// caller is responsible to close the returned *os.File if needed.
func prepareProcessExec(c *Container, options *ExecOptions, env []string, sessionID string) (*os.File, error) {
	f, err := ioutil.TempFile(c.execBundlePath(sessionID), "exec-process-")
	if err != nil {
		return nil, err
	}
	pspec := new(spec.Process)
	if err := JSONDeepCopy(c.config.Spec.Process, pspec); err != nil {
		return nil, err
	}
	pspec.SelinuxLabel = c.config.ProcessLabel
	pspec.Args = options.Cmd

	// We need to default this to false else it will inherit terminal as true
	// from the container.
	pspec.Terminal = false
	if options.Terminal {
		pspec.Terminal = true
	}
	if len(env) > 0 {
		pspec.Env = append(pspec.Env, env...)
	}

	// Add secret envs if they exist
	manager, err := c.runtime.SecretsManager()
	if err != nil {
		return nil, err
	}
	for name, secr := range c.config.EnvSecrets {
		_, data, err := manager.LookupSecretData(secr.Name)
		if err != nil {
			return nil, err
		}
		pspec.Env = append(pspec.Env, fmt.Sprintf("%s=%s", name, string(data)))
	}

	if options.Cwd != "" {
		pspec.Cwd = options.Cwd
	}

	var addGroups []string
	var sgids []uint32

	// if the user is empty, we should inherit the user that the container is currently running with
	user := options.User
	if user == "" {
		logrus.Debugf("Set user to %s", c.config.User)
		user = c.config.User
		addGroups = c.config.Groups
	}

	overrides := c.getUserOverrides()
	execUser, err := lookup.GetUserGroupInfo(c.state.Mountpoint, user, overrides)
	if err != nil {
		return nil, err
	}

	if len(addGroups) > 0 {
		sgids, err = lookup.GetContainerGroups(addGroups, c.state.Mountpoint, overrides)
		if err != nil {
			return nil, fmt.Errorf("error looking up supplemental groups for container %s exec session %s: %w", c.ID(), sessionID, err)
		}
	}

	// If user was set, look it up in the container to get a UID to use on
	// the host
	if user != "" || len(sgids) > 0 {
		if user != "" {
			for _, sgid := range execUser.Sgids {
				sgids = append(sgids, uint32(sgid))
			}
		}
		processUser := spec.User{
			UID:            uint32(execUser.Uid),
			GID:            uint32(execUser.Gid),
			AdditionalGids: sgids,
		}

		pspec.User = processUser
	}

	ctrSpec, err := c.specFromState()
	if err != nil {
		return nil, err
	}

	allCaps, err := capabilities.BoundingSet()
	if err != nil {
		return nil, err
	}
	if options.Privileged {
		pspec.Capabilities.Bounding = allCaps
	} else {
		pspec.Capabilities.Bounding = ctrSpec.Process.Capabilities.Bounding
	}

	// Always unset the inheritable capabilities similarly to what the Linux kernel does
	// They are used only when using capabilities with uid != 0.
	pspec.Capabilities.Inheritable = []string{}

	if execUser.Uid == 0 {
		pspec.Capabilities.Effective = pspec.Capabilities.Bounding
		pspec.Capabilities.Permitted = pspec.Capabilities.Bounding
	} else if user == c.config.User {
		pspec.Capabilities.Effective = ctrSpec.Process.Capabilities.Effective
		pspec.Capabilities.Inheritable = ctrSpec.Process.Capabilities.Effective
		pspec.Capabilities.Permitted = ctrSpec.Process.Capabilities.Effective
		pspec.Capabilities.Ambient = ctrSpec.Process.Capabilities.Effective
	}

	hasHomeSet := false
	for _, s := range pspec.Env {
		if strings.HasPrefix(s, "HOME=") {
			hasHomeSet = true
			break
		}
	}
	if !hasHomeSet {
		pspec.Env = append(pspec.Env, fmt.Sprintf("HOME=%s", execUser.Home))
	}

	processJSON, err := json.Marshal(pspec)
	if err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(f.Name(), processJSON, 0644); err != nil {
		return nil, err
	}
	return f, nil
}
