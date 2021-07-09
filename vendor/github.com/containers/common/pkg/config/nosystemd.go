// +build !systemd

package config

func defaultCgroupManager() string {
	return CgroupfsCgroupsManager
}

func defaultEventsLogger() string {
	return "file"
}

func defaultLogDriver() string {
	return DefaultLogDriver
}

func useSystemd() bool {
	return false
}
