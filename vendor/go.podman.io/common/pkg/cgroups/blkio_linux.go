//go:build linux

package cgroups

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs"
	"github.com/opencontainers/cgroups/fs2"
)

type linuxBlkioHandler struct {
	Blkio fs.BlkioGroup
}

func getBlkioHandler() *linuxBlkioHandler {
	return &linuxBlkioHandler{}
}

// Apply set the specified constraints.
func (c *linuxBlkioHandler) Apply(ctr *CgroupControl, res *cgroups.Resources) error {
	man, err := fs2.NewManager(ctr.config, filepath.Join(cgroupRoot, ctr.config.Path))
	if err != nil {
		return err
	}
	return man.Set(res)
}

// Stat fills a metrics structure with usage stats for the controller.
func (c *linuxBlkioHandler) Stat(ctr *CgroupControl, m *cgroups.Stats) error {
	var ioServiceBytesRecursive []cgroups.BlkioStatEntry

	// more details on the io.stat file format:X https://facebookmicrosites.github.io/cgroup2/docs/io-controller.html
	values, err := readCgroup2MapFile(ctr, "io.stat")
	if err != nil {
		return err
	}
	for k, v := range values {
		d := strings.Split(k, ":")
		if len(d) != 2 {
			continue
		}
		minor, err := strconv.ParseUint(d[0], 10, 0)
		if err != nil {
			return err
		}
		major, err := strconv.ParseUint(d[1], 10, 0)
		if err != nil {
			return err
		}

		for _, item := range v {
			d := strings.Split(item, "=")
			if len(d) != 2 {
				continue
			}
			op := d[0]

			// Accommodate the cgroup v1 naming
			switch op {
			case "rbytes":
				op = "read"
			case "wbytes":
				op = "write"
			}

			value, err := strconv.ParseUint(d[1], 10, 0)
			if err != nil {
				return err
			}

			entry := cgroups.BlkioStatEntry{
				Op:    op,
				Major: major,
				Minor: minor,
				Value: value,
			}
			ioServiceBytesRecursive = append(ioServiceBytesRecursive, entry)
		}
	}
	m.BlkioStats.IoServiceBytesRecursive = ioServiceBytesRecursive
	return nil
}
