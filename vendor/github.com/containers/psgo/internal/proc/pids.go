package proc

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// GetPIDs extracts and returns all PIDs from /proc.
func GetPIDs() ([]string, error) {
	procDir, err := os.Open("/proc/")
	if err != nil {
		return nil, err
	}
	defer procDir.Close()

	// extract string slice of all directories in procDir
	pidDirs, err := procDir.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	pids := []string{}
	for _, pidDir := range pidDirs {
		_, err := strconv.Atoi(pidDir)
		if err != nil {
			// skip non-numerical entries (e.g., `/proc/softirqs`)
			continue
		}
		pids = append(pids, pidDir)
	}

	return pids, nil
}

// pidCgroupPath returns the path to the pid's pids cgroup.
func pidCgroupPath(pid string) (string, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%s/cgroup", pid))
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) != 3 {
			continue
		}
		if fields[1] == "pids" {
			return fmt.Sprintf("/sys/fs/cgroup/pids/%s/cgroup.procs", fields[2]), nil
		}
	}
	return "", fmt.Errorf("couldn't find pids group for PID %s", pid)
}

// GetPIDsFromCgroup returns a strings slice of all pids listesd in pid's pids
// cgroup.
func GetPIDsFromCgroup(pid string) ([]string, error) {
	cgroupPath, err := pidCgroupPath(pid)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(cgroupPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	pids := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		pids = append(pids, scanner.Text())
	}
	return pids, nil
}
