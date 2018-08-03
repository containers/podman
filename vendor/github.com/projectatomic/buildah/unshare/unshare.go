// +build linux

package unshare

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/containers/storage/pkg/reexec"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah/util"
)

// Cmd wraps an exec.Cmd created by the reexec package in unshare(), and
// handles setting ID maps and other related settings by triggering
// initialization code in the child.
type Cmd struct {
	*exec.Cmd
	UnshareFlags               int
	UseNewuidmap               bool
	UidMappings                []specs.LinuxIDMapping
	UseNewgidmap               bool
	GidMappings                []specs.LinuxIDMapping
	GidMappingsEnableSetgroups bool
	Setsid                     bool
	Setpgrp                    bool
	Ctty                       *os.File
	OOMScoreAdj                int
	Hook                       func(pid int) error
}

// Command creates a new Cmd which can be customized.
func Command(args ...string) *Cmd {
	cmd := reexec.Command(args...)
	return &Cmd{
		Cmd: cmd,
	}
}

func (c *Cmd) Start() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Set an environment variable to tell the child to synchronize its startup.
	if c.Env == nil {
		c.Env = os.Environ()
	}
	c.Env = append(c.Env, fmt.Sprintf("_Buildah-unshare=%d", c.UnshareFlags))

	// Create the pipe for reading the child's PID.
	pidRead, pidWrite, err := os.Pipe()
	if err != nil {
		return errors.Wrapf(err, "error creating pid pipe")
	}
	c.Env = append(c.Env, fmt.Sprintf("_Buildah-pid-pipe=%d", len(c.ExtraFiles)+3))
	c.ExtraFiles = append(c.ExtraFiles, pidWrite)

	// Create the pipe for letting the child know to proceed.
	continueRead, continueWrite, err := os.Pipe()
	if err != nil {
		pidRead.Close()
		pidWrite.Close()
		return errors.Wrapf(err, "error creating pid pipe")
	}
	c.Env = append(c.Env, fmt.Sprintf("_Buildah-continue-pipe=%d", len(c.ExtraFiles)+3))
	c.ExtraFiles = append(c.ExtraFiles, continueRead)

	// Pass along other instructions.
	if c.Setsid {
		c.Env = append(c.Env, "_Buildah-setsid=1")
	}
	if c.Setpgrp {
		c.Env = append(c.Env, "_Buildah-setpgrp=1")
	}
	if c.Ctty != nil {
		c.Env = append(c.Env, fmt.Sprintf("_Buildah-ctty=%d", len(c.ExtraFiles)+3))
		c.ExtraFiles = append(c.ExtraFiles, c.Ctty)
	}
	if c.GidMappingsEnableSetgroups {
		c.Env = append(c.Env, "_Buildah-allow-setgroups=1")
	} else {
		c.Env = append(c.Env, "_Buildah-allow-setgroups=0")
	}

	// Make sure we clean up our pipes.
	defer func() {
		if pidRead != nil {
			pidRead.Close()
		}
		if pidWrite != nil {
			pidWrite.Close()
		}
		if continueRead != nil {
			continueRead.Close()
		}
		if continueWrite != nil {
			continueWrite.Close()
		}
	}()

	// Start the new process.
	err = c.Cmd.Start()
	if err != nil {
		return err
	}

	// Close the ends of the pipes that the parent doesn't need.
	continueRead.Close()
	continueRead = nil
	pidWrite.Close()
	pidWrite = nil

	// Read the child's PID from the pipe.
	pidString := ""
	b := new(bytes.Buffer)
	io.Copy(b, pidRead)
	pidString = b.String()
	pid, err := strconv.Atoi(pidString)
	if err != nil {
		fmt.Fprintf(continueWrite, "error parsing PID %q: %v", pidString, err)
		return errors.Wrapf(err, "error parsing PID %q", pidString)
	}
	pidString = fmt.Sprintf("%d", pid)

	// If we created a new user namespace, set any specified mappings.
	if c.UnshareFlags&syscall.CLONE_NEWUSER != 0 {
		// Always set "setgroups".
		setgroups, err := os.OpenFile(fmt.Sprintf("/proc/%s/setgroups", pidString), os.O_TRUNC|os.O_WRONLY, 0)
		if err != nil {
			fmt.Fprintf(continueWrite, "error opening setgroups: %v", err)
			return errors.Wrapf(err, "error opening /proc/%s/setgroups", pidString)
		}
		defer setgroups.Close()
		if c.GidMappingsEnableSetgroups {
			if _, err := fmt.Fprintf(setgroups, "allow"); err != nil {
				fmt.Fprintf(continueWrite, "error writing \"allow\" to setgroups: %v", err)
				return errors.Wrapf(err, "error opening \"allow\" to /proc/%s/setgroups", pidString)
			}
		} else {
			if _, err := fmt.Fprintf(setgroups, "deny"); err != nil {
				fmt.Fprintf(continueWrite, "error writing \"deny\" to setgroups: %v", err)
				return errors.Wrapf(err, "error writing \"deny\" to /proc/%s/setgroups", pidString)
			}
		}

		if len(c.UidMappings) == 0 || len(c.GidMappings) == 0 {
			uidmap, gidmap, err := util.GetHostIDMappings("")
			if err != nil {
				fmt.Fprintf(continueWrite, "error reading ID mappings in parent: %v", err)
				return errors.Wrapf(err, "error reading ID mappings in parent")
			}
			if len(c.UidMappings) == 0 {
				c.UidMappings = uidmap
				for i := range c.UidMappings {
					c.UidMappings[i].HostID = c.UidMappings[i].ContainerID
				}
			}
			if len(c.GidMappings) == 0 {
				c.GidMappings = gidmap
				for i := range c.GidMappings {
					c.GidMappings[i].HostID = c.GidMappings[i].ContainerID
				}
			}
		}

		if len(c.GidMappings) > 0 {
			// Build the GID map, since writing to the proc file has to be done all at once.
			g := new(bytes.Buffer)
			for _, m := range c.GidMappings {
				fmt.Fprintf(g, "%d %d %d\n", m.ContainerID, m.HostID, m.Size)
			}
			// Set the GID map.
			if c.UseNewgidmap {
				cmd := exec.Command("newgidmap", append([]string{pidString}, strings.Fields(strings.Replace(g.String(), "\n", " ", -1))...)...)
				g.Reset()
				cmd.Stdout = g
				cmd.Stderr = g
				err := cmd.Run()
				if err != nil {
					fmt.Fprintf(continueWrite, "error running newgidmap: %v: %s", err, g.String())
					return errors.Wrapf(err, "error running newgidmap: %s", g.String())
				}
			} else {
				gidmap, err := os.OpenFile(fmt.Sprintf("/proc/%s/gid_map", pidString), os.O_TRUNC|os.O_WRONLY, 0)
				if err != nil {
					fmt.Fprintf(continueWrite, "error opening /proc/%s/gid_map: %v", pidString, err)
					return errors.Wrapf(err, "error opening /proc/%s/gid_map", pidString)
				}
				defer gidmap.Close()
				if _, err := fmt.Fprintf(gidmap, "%s", g.String()); err != nil {
					fmt.Fprintf(continueWrite, "error writing /proc/%s/gid_map: %v", pidString, err)
					return errors.Wrapf(err, "error writing /proc/%s/gid_map", pidString)
				}
			}
		}

		if len(c.UidMappings) > 0 {
			// Build the UID map, since writing to the proc file has to be done all at once.
			u := new(bytes.Buffer)
			for _, m := range c.UidMappings {
				fmt.Fprintf(u, "%d %d %d\n", m.ContainerID, m.HostID, m.Size)
			}
			// Set the GID map.
			if c.UseNewuidmap {
				cmd := exec.Command("newuidmap", append([]string{pidString}, strings.Fields(strings.Replace(u.String(), "\n", " ", -1))...)...)
				u.Reset()
				cmd.Stdout = u
				cmd.Stderr = u
				err := cmd.Run()
				if err != nil {
					fmt.Fprintf(continueWrite, "error running newuidmap: %v: %s", err, u.String())
					return errors.Wrapf(err, "error running newuidmap: %s", u.String())
				}
			} else {
				uidmap, err := os.OpenFile(fmt.Sprintf("/proc/%s/uid_map", pidString), os.O_TRUNC|os.O_WRONLY, 0)
				if err != nil {
					fmt.Fprintf(continueWrite, "error opening /proc/%s/uid_map: %v", pidString, err)
					return errors.Wrapf(err, "error opening /proc/%s/uid_map", pidString)
				}
				defer uidmap.Close()
				if _, err := fmt.Fprintf(uidmap, "%s", u.String()); err != nil {
					fmt.Fprintf(continueWrite, "error writing /proc/%s/uid_map: %v", pidString, err)
					return errors.Wrapf(err, "error writing /proc/%s/uid_map", pidString)
				}
			}
		}
	}

	// Adjust the process's OOM score.
	oomScoreAdj, err := os.OpenFile(fmt.Sprintf("/proc/%s/oom_score_adj", pidString), os.O_TRUNC|os.O_WRONLY, 0)
	if err != nil {
		fmt.Fprintf(continueWrite, "error opening oom_score_adj: %v", err)
		return errors.Wrapf(err, "error opening /proc/%s/oom_score_adj", pidString)
	}
	if _, err := fmt.Fprintf(oomScoreAdj, "%d\n", c.OOMScoreAdj); err != nil {
		fmt.Fprintf(continueWrite, "error writing \"%d\" to oom_score_adj: %v", c.OOMScoreAdj, err)
		return errors.Wrapf(err, "error writing \"%d\" to /proc/%s/oom_score_adj", c.OOMScoreAdj, pidString)
	}
	defer oomScoreAdj.Close()

	// Run any additional setup that we want to do before the child starts running proper.
	if c.Hook != nil {
		if err = c.Hook(pid); err != nil {
			fmt.Fprintf(continueWrite, "hook error: %v", err)
			return err
		}
	}

	return nil
}

func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

func (c *Cmd) CombinedOutput() ([]byte, error) {
	return nil, errors.New("unshare: CombinedOutput() not implemented")
}

func (c *Cmd) Output() ([]byte, error) {
	return nil, errors.New("unshare: Output() not implemented")
}
