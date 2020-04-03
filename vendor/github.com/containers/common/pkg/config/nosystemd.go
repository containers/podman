// +build !systemd

package config

func defaultCgroupManager() string {
	return "cgroupfs"
}

func defaultEventsLogger() string {
	return "file"
}
