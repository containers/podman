// +build remoteclient

package main

import (
	"github.com/spf13/cobra"
)

const remoteclient = true

// commands that only the remoteclient implements
func getMainCommands() []*cobra.Command {
	return []*cobra.Command{}
}

// commands that only the remoteclient implements
func getAppCommands() []*cobra.Command { // nolint:varcheck,deadcode,unused
	return []*cobra.Command{}
}

// commands that only the remoteclient implements
func getImageSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

// commands that only the remoteclient implements
func getContainerSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

// commands that only the remoteclient implements
func getGenerateSubCommands() []*cobra.Command { // nolint:varcheck,deadcode,unused
	return []*cobra.Command{}
}

// commands that only the remoteclient implements
func getPlaySubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

// commands that only the remoteclient implements
func getTrustSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

// commands that only the remoteclient implements
func getSystemSubCommands() []*cobra.Command {
	return []*cobra.Command{}
}

func getDefaultSecurityOptions() []string {
	return []string{}
}

// getDefaultSysctls
func getDefaultSysctls() []string {
	return []string{}
}

// getDefaultDevices
func getDefaultDevices() []string {
	return []string{}
}

func getDefaultVolumes() []string {
	return []string{}
}

func getDefaultDNSServers() []string {
	return []string{}
}

func getDefaultDNSSearches() []string {
	return []string{}
}

func getDefaultDNSOptions() []string {
	return []string{}
}

func getDefaultEnv() []string {
	return []string{}
}

func getDefaultInitPath() string {
	return ""
}

func getDefaultIPCNS() string {
	return ""
}

func getDefaultPidNS() string {
	return ""
}

func getDefaultNetNS() string {
	return ""
}

func getDefaultCgroupNS() string {
	return ""
}

func getDefaultUTSNS() string {
	return ""
}

func getDefaultShmSize() string {
	return ""
}

func getDefaultUlimits() []string {
	return []string{}
}

func getDefaultUserNS() string {
	return ""
}

func getDefaultPidsLimit() int64 {
	return -1
}

func getDefaultPidsDescription() string {
	return "Tune container pids limit (set 0 for unlimited, -1 for server defaults)"
}

func getDefaultShareNetwork() string { // nolint:varcheck,deadcode,unused
	return ""
}

func getDefaultDetachKeys() string {
	return ""
}
