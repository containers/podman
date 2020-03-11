package mount

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Parse /proc/self/mountinfo because comparing Dev and ino does not work from
// bind mounts
func parseMountTable() ([]*Info, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseInfoFile(f)
}

func parseInfoFile(r io.Reader) ([]*Info, error) {
	s := bufio.NewScanner(r)
	out := []*Info{}

	for s.Scan() {
		/*
		   36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue
		   (0)(1)(2)   (3)   (4)      (5)      (6)   (7) (8)   (9)        (10)

		   (0) mount ID:  unique identifier of the mount (may be reused after umount)
		   (1) parent ID:  ID of parent (or of self for the top of the mount tree)
		   (2) major:minor:  value of st_dev for files on filesystem
		   (3) root:  root of the mount within the filesystem
		   (4) mount point:  mount point relative to the process's root
		   (5) mount options:  per mount options
		   (6) optional fields:  zero or more fields of the form "tag[:value]"
		   (7) separator:  marks the end of the optional fields
		   (8) filesystem type:  name of filesystem of the form "type[.subtype]"
		   (9) mount source:  filesystem specific information or "none"
		   (10) super options:  per super block options
		*/
		text := s.Text()
		fields := strings.Split(text, " ")
		numFields := len(fields)
		if numFields < 10 {
			// should be at least 10 fields
			return nil, errors.Errorf("Parsing %q failed: not enough fields (%d)", text, numFields)
		}

		p := &Info{}
		// ignore any number parsing errors, there should not be any
		p.ID, _ = strconv.Atoi(fields[0])
		p.Parent, _ = strconv.Atoi(fields[1])
		mm := strings.Split(fields[2], ":")
		if len(mm) != 2 {
			return nil, fmt.Errorf("Parsing %q failed: unexpected minor:major pair %s", text, mm)
		}
		p.Major, _ = strconv.Atoi(mm[0])
		p.Minor, _ = strconv.Atoi(mm[1])
		p.Root = fields[3]
		p.Mountpoint = fields[4]
		p.Opts = fields[5]

		// one or more optional fields, when a separator (-)
		i := 6
		for ; i < numFields && fields[i] != "-"; i++ {
			switch i {
			case 6:
				p.Optional = string(fields[6])
			default:
				/* NOTE there might be more optional fields before the separator,
				   such as fields[7] or fields[8], although as of Linux kernel 5.5
				   the only known ones are mount propagation flags in fields[6].
				   The correct behavior is to ignore any unknown optional fields.
				*/
			}
		}
		if i == numFields {
			return nil, fmt.Errorf("Parsing %q failed: missing - separator", text)
		}

		// There should be 3 fields after the separator...
		if i+4 > numFields {
			return nil, fmt.Errorf("Parsing %q failed: not enough fields after a - separator", text)
		}
		// ... but in Linux <= 3.9 mounting a cifs with spaces in a share name
		// (like "//serv/My Documents") _may_ end up having a space in the last field
		// of mountinfo (like "unc=//serv/My Documents"). Since kernel 3.10-rc1, cifs
		// option unc= is ignored,  so a space should not appear. In here we ignore
		// those "extra" fields caused by extra spaces.
		p.Fstype = fields[i+1]
		p.Source = fields[i+2]
		p.VfsOpts = fields[i+3]

		out = append(out, p)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// PidMountInfo collects the mounts for a specific process ID. If the process
// ID is unknown, it is better to use `GetMounts` which will inspect
// "/proc/self/mountinfo" instead.
func PidMountInfo(pid int) ([]*Info, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/mountinfo", pid))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseInfoFile(f)
}
