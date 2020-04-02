// +build systemd

package config

func defaultCgroupManager() string {
	return SystemdCgroupsManager
}
func defaultEventsLogger() string {
	return "journald"
}
