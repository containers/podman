package process

import (
	"os"
	"strconv"
	"time"

	"github.com/containers/psgo/internal/host"
	"github.com/containers/psgo/internal/proc"
	"github.com/containers/psgo/internal/types"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/pkg/errors"
)

// Process includes process-related from the /proc FS.
type Process struct {
	// PID is the process ID.
	Pid string
	// Stat contains data from /proc/$pid/stat.
	Stat proc.Stat
	// Status containes data from /proc/$pid/status.
	Status proc.Status
	// CmdLine containes data from /proc/$pid/cmdline.
	CmdLine []string
	// Label containers data from /proc/$pid/attr/current.
	Label string
	// PidNS contains data from /proc/$pid/ns/pid.
	PidNS string
	// Huser is the effective host user of a container process.
	Huser string
	// Hgroup is the effective host group of a container process.
	Hgroup string
}

// LookupGID returns the textual group ID, if it can be optained, or the
// decimal representation otherwise.
func LookupGID(gid string) (string, error) {
	gidNum, err := strconv.Atoi(gid)
	if err != nil {
		return "", errors.Wrap(err, "error parsing group ID")
	}
	g, err := user.LookupGid(gidNum)
	if err != nil {
		return gid, nil
	}
	return g.Name, nil
}

// LookupUID return the textual user ID, if it can be optained, or the decimal
// representation otherwise.
func LookupUID(uid string) (string, error) {
	uidNum, err := strconv.Atoi(uid)
	if err != nil {
		return "", errors.Wrap(err, "error parsing user ID")
	}
	u, err := user.LookupUid(uidNum)
	if err != nil {
		return uid, nil
	}
	return u.Name, nil
}

// New returns a new Process with the specified pid and parses the relevant
// data from /proc and /dev.
func New(ctx *types.PsContext, pid string) (*Process, error) {
	p := Process{Pid: pid}

	if err := p.parseStat(); err != nil {
		return nil, err
	}
	if err := p.parseStatus(ctx); err != nil {
		return nil, err
	}
	if err := p.parseCmdLine(); err != nil {
		return nil, err
	}
	if err := p.parsePIDNamespace(); err != nil {
		// Ignore permission errors as those occur for some pids when
		// the caller has limited permissions.
		if !os.IsPermission(err) {
			return nil, err
		}
	}
	if err := p.parseLabel(); err != nil {
		return nil, err
	}

	return &p, nil
}

// FromPIDs creates a new Process for each pid.
func FromPIDs(ctx *types.PsContext, pids []string) ([]*Process, error) {
	processes := []*Process{}
	for _, pid := range pids {
		p, err := New(ctx, pid)
		if err != nil {
			if os.IsNotExist(err) {
				// proc parsing is racy
				// Let's ignore "does not exist" errors
				continue
			}
			return nil, err
		}
		processes = append(processes, p)
	}
	return processes, nil
}

// parseStat parses /proc/$pid/stat.
func (p *Process) parseStat() error {
	s, err := proc.ParseStat(p.Pid)
	if err != nil {
		return err
	}
	p.Stat = *s
	return nil
}

// parseStatus parses /proc/$pid/status.
func (p *Process) parseStatus(ctx *types.PsContext) error {
	s, err := proc.ParseStatus(ctx, p.Pid)
	if err != nil {
		return err
	}
	p.Status = *s
	return nil
}

// parseCmdLine parses /proc/$pid/cmdline.
func (p *Process) parseCmdLine() error {
	s, err := proc.ParseCmdLine(p.Pid)
	if err != nil {
		return err
	}
	p.CmdLine = s
	return nil
}

// parsePIDNamespace sets the PID namespace.
func (p *Process) parsePIDNamespace() error {
	pidNS, err := proc.ParsePIDNamespace(p.Pid)
	if err != nil {
		return err
	}
	p.PidNS = pidNS
	return nil
}

// parseLabel parses the security label.
func (p *Process) parseLabel() error {
	label, err := proc.ParseAttrCurrent(p.Pid)
	if err != nil {
		return err
	}
	p.Label = label
	return nil
}

// SetHostData sets all host-related data fields.
func (p *Process) SetHostData() error {
	var err error

	p.Huser, err = LookupUID(p.Status.Uids[1])
	if err != nil {
		return err
	}

	p.Hgroup, err = LookupGID(p.Status.Gids[1])
	if err != nil {
		return err
	}

	return nil
}

// ElapsedTime returns the time.Duration since process p was created.
func (p *Process) ElapsedTime() (time.Duration, error) {
	sinceBoot, err := strconv.ParseInt(p.Stat.Starttime, 10, 64)
	if err != nil {
		return 0, err
	}

	sinceBoot = sinceBoot / host.ClockTicks()

	bootTime, err := host.BootTime()
	if err != nil {
		return 0, err
	}
	created := time.Unix(sinceBoot+bootTime, 0)
	return (time.Now()).Sub(created), nil
}

// CPUTime returns the cumlative CPU time of process p as a time.Duration.
func (p *Process) CPUTime() (time.Duration, error) {
	user, err := strconv.ParseInt(p.Stat.Utime, 10, 64)
	if err != nil {
		return 0, err
	}
	system, err := strconv.ParseInt(p.Stat.Stime, 10, 64)
	if err != nil {
		return 0, err
	}
	secs := (user + system) / host.ClockTicks()
	cpu := time.Unix(secs, 0)
	return cpu.Sub(time.Unix(0, 0)), nil
}
