package cgroups

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

type blkioHandler struct {
}

func getBlkioHandler() *blkioHandler {
	return &blkioHandler{}
}

// Apply set the specified constraints
func (c *blkioHandler) Apply(ctr *CgroupControl, res *spec.LinuxResources) error {
	if res.BlockIO == nil {
		return nil
	}
	return fmt.Errorf("blkio apply function not implemented yet")
}

// Create the cgroup
func (c *blkioHandler) Create(ctr *CgroupControl) (bool, error) {
	if ctr.cgroup2 {
		return false, fmt.Errorf("function not implemented yet")
	}
	return ctr.createCgroupDirectory(Blkio)
}

// Destroy the cgroup
func (c *blkioHandler) Destroy(ctr *CgroupControl) error {
	return os.Remove(ctr.getCgroupv1Path(Blkio))
}

// Stat fills a metrics structure with usage stats for the controller
func (c *blkioHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	var ioServiceBytesRecursive []BlkIOEntry

	if ctr.cgroup2 {
		return fmt.Errorf("function not implemented yet")
	}

	BlkioRoot := ctr.getCgroupv1Path(Blkio)

	p := filepath.Join(BlkioRoot, "blkio.throttle.io_service_bytes_recursive")
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var ioServiceBytesRecursive []BlkIOEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		d := strings.Split(parts[0], ":")
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

		op := parts[1]

		value, err := strconv.ParseUint(parts[2], 10, 0)
		if err != nil {
			return err
		}
		entry := BlkIOEntry{
			Op:    op,
			Major: major,
			Minor: minor,
			Value: value,
		}
		ioServiceBytesRecursive = append(ioServiceBytesRecursive, entry)
	}
	m.Blkio = BlkioMetrics{IoServiceBytesRecursive: ioServiceBytesRecursive}
	return nil
}
