package dnsservice

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/storage"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// GenerateDNSFileNames returns the path for the dns service lockfile and hosts dir
// based on the input network name
func GenerateDNSFileNames(network string) (string, string) {
	lockfilePath := filepath.Join(dnsServiceDir, fmt.Sprintf("dns-service-%s.lock", network))
	dnsHostsFileDirPath := filepath.Join(dnsServiceDir, network)
	return lockfilePath, dnsHostsFileDirPath
}

// CreateDNSHostsFileDir creates the directory needed for the dns-service
// if the directory does not exist.  The directory path should be in the form
// of /run/libpod/dns/<network_name> i.e. podman, podman1
func CreateDNSHostsFileDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		logrus.Debugf("creating dns-service hosts dir %s", path)
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// AddHostDNSRecord adds a record to the dns-service file
func AddHostDNSRecord(ips []string, host, cid, network string) error {
	var records string
	dnsServiceLockFile, dnsHostsFileDirPath := GenerateDNSFileNames(network)
	lockfile, err := storage.GetLockfile(dnsServiceLockFile)
	if err != nil {
		return errors.Wrap(err, "unable to get lockfile writing dns hosts")
	}
	lockfile.Lock()
	defer lockfile.Unlock()

	f, err := os.OpenFile(filepath.Join(dnsHostsFileDirPath, "hosts"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		record := fmt.Sprintf("%s %s #%s\n", ip, host, cid)
		logrus.Debug("Adding host record: %s", record)
		records = records + record
	}

	if _, err := f.Write([]byte(records)); err != nil {
		return errors.Wrapf(err, "unable to write host record for %s", cid)
	}
	return f.Close()
}

// RemoveHostDNSRecord removes a record from the dns-service file
func RemoveHostDNSRecord(cid, network string) error {
	var output []string
	dnsServiceLockFile, dnsHostsFileDirPath := GenerateDNSFileNames(network)
	lockfile, err := storage.GetLockfile(dnsServiceLockFile)
	if err != nil {
		return errors.Wrapf(err, "unable to get lockfile for container %s", cid)
	}
	lockfile.Lock()
	defer lockfile.Unlock()

	file, err := os.OpenFile(filepath.Join(dnsHostsFileDirPath, "hosts"), os.O_RDWR, 0755)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if !strings.Contains(scanner.Text(), fmt.Sprintf("#%s", cid)) {
			output = append(output, fmt.Sprintf("%s\n", scanner.Text()))
		}
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}

	_, err = file.WriteString(strings.Join(output, ""))
	return err
}
