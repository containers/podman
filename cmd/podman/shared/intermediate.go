package shared

import (
	"github.com/containers/libpod/cmd/podman/cliconfig"
	"github.com/sirupsen/logrus"
)

/*
attention

in this file you will see alot of struct duplication.  this was done because people wanted a strongly typed
varlink mechanism.  this resulted in us creating this intermediate layer that allows us to take the input
from the cli and make an intermediate layer which can be transferred as strongly typed structures over a varlink
interface.

we intentionally avoided heavy use of reflection here because we were concerned about performance impacts to the
non-varlink intermediate layer generation.
*/

// GenericCLIResult describes the overall interface for dealing with
// the create command cli in both local and remote uses
type GenericCLIResult interface {
	IsSet() bool
	Name() string
	Value() interface{}
}

// CRStringSlice describes a string slice cli struct
type CRStringSlice struct {
	Val []string
	createResult
}

// CRString describes a string cli struct
type CRString struct {
	Val string
	createResult
}

// CRUint64 describes a uint64 cli struct
type CRUint64 struct {
	Val uint64
	createResult
}

// CRFloat64 describes a float64 cli struct
type CRFloat64 struct {
	Val float64
	createResult
}

//CRBool describes a bool cli struct
type CRBool struct {
	Val bool
	createResult
}

// CRInt64 describes an int64 cli struct
type CRInt64 struct {
	Val int64
	createResult
}

// CRUint describes a uint cli struct
type CRUint struct {
	Val uint
	createResult
}

// CRInt describes an int cli struct
type CRInt struct {
	Val int
	createResult
}

// CRStringArray describes a stringarray cli struct
type CRStringArray struct {
	Val []string
	createResult
}

type createResult struct {
	Flag    string
	Changed bool
}

// GenericCLIResults in the intermediate object between the cobra cli
// and createconfig
type GenericCLIResults struct {
	results   map[string]GenericCLIResult
	InputArgs []string
}

// IsSet returns a bool if the flag was changed
func (f GenericCLIResults) IsSet(flag string) bool {
	r := f.findResult(flag)
	if r == nil {
		return false
	}
	return r.IsSet()
}

// Value returns the value of the cli flag
func (f GenericCLIResults) Value(flag string) interface{} {
	r := f.findResult(flag)
	if r == nil {
		return ""
	}
	return r.Value()
}

func (f GenericCLIResults) findResult(flag string) GenericCLIResult {
	val, ok := f.results[flag]
	if ok {
		return val
	}
	logrus.Debugf("unable to find flag %s", flag)
	return nil
}

// Bool is a wrapper to get a bool value from GenericCLIResults
func (f GenericCLIResults) Bool(flag string) bool {
	r := f.findResult(flag)
	if r == nil {
		return false
	}
	return r.Value().(bool)
}

// String is a wrapper to get a string value from GenericCLIResults
func (f GenericCLIResults) String(flag string) string {
	r := f.findResult(flag)
	if r == nil {
		return ""
	}
	return r.Value().(string)
}

// Uint is a wrapper to get an uint value from GenericCLIResults
func (f GenericCLIResults) Uint(flag string) uint {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(uint)
}

// StringSlice is a wrapper to get a stringslice value from GenericCLIResults
func (f GenericCLIResults) StringSlice(flag string) []string {
	r := f.findResult(flag)
	if r == nil {
		return []string{}
	}
	return r.Value().([]string)
}

// StringArray is a wrapper to get a stringslice value from GenericCLIResults
func (f GenericCLIResults) StringArray(flag string) []string {
	r := f.findResult(flag)
	if r == nil {
		return []string{}
	}
	return r.Value().([]string)
}

// Uint64 is a wrapper to get an uint64 value from GenericCLIResults
func (f GenericCLIResults) Uint64(flag string) uint64 {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(uint64)
}

// Int64 is a wrapper to get an int64 value from GenericCLIResults
func (f GenericCLIResults) Int64(flag string) int64 {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(int64)
}

// Int is a wrapper to get an int value from GenericCLIResults
func (f GenericCLIResults) Int(flag string) int {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(int)
}

// Float64 is a wrapper to get an float64 value from GenericCLIResults
func (f GenericCLIResults) Float64(flag string) float64 {
	r := f.findResult(flag)
	if r == nil {
		return 0
	}
	return r.Value().(float64)
}

// Float64 is a wrapper to get an float64 value from GenericCLIResults
func (f GenericCLIResults) Changed(flag string) bool {
	r := f.findResult(flag)
	if r == nil {
		return false
	}
	return r.IsSet()
}

// IsSet ...
func (c CRStringSlice) IsSet() bool { return c.Changed }

// Name ...
func (c CRStringSlice) Name() string { return c.Flag }

// Value ...
func (c CRStringSlice) Value() interface{} { return c.Val }

// IsSet ...
func (c CRString) IsSet() bool { return c.Changed }

// Name ...
func (c CRString) Name() string { return c.Flag }

// Value ...
func (c CRString) Value() interface{} { return c.Val }

// IsSet ...
func (c CRUint64) IsSet() bool { return c.Changed }

// Name ...
func (c CRUint64) Name() string { return c.Flag }

// Value ...
func (c CRUint64) Value() interface{} { return c.Val }

// IsSet ...
func (c CRFloat64) IsSet() bool { return c.Changed }

// Name ...
func (c CRFloat64) Name() string { return c.Flag }

// Value ...
func (c CRFloat64) Value() interface{} { return c.Val }

// IsSet ...
func (c CRBool) IsSet() bool { return c.Changed }

// Name ...
func (c CRBool) Name() string { return c.Flag }

// Value ...
func (c CRBool) Value() interface{} { return c.Val }

// IsSet ...
func (c CRInt64) IsSet() bool { return c.Changed }

// Name ...
func (c CRInt64) Name() string { return c.Flag }

// Value ...
func (c CRInt64) Value() interface{} { return c.Val }

// IsSet ...
func (c CRUint) IsSet() bool { return c.Changed }

// Name ...
func (c CRUint) Name() string { return c.Flag }

// Value ...
func (c CRUint) Value() interface{} { return c.Val }

// IsSet ...
func (c CRInt) IsSet() bool { return c.Changed }

// Name ...
func (c CRInt) Name() string { return c.Flag }

// Value ...
func (c CRInt) Value() interface{} { return c.Val }

// IsSet ...
func (c CRStringArray) IsSet() bool { return c.Changed }

// Name ...
func (c CRStringArray) Name() string { return c.Flag }

// Value ...
func (c CRStringArray) Value() interface{} { return c.Val }

func newCreateResult(c *cliconfig.PodmanCommand, flag string) createResult {
	return createResult{
		Flag:    flag,
		Changed: c.IsSet(flag),
	}
}

func newCRStringSlice(c *cliconfig.PodmanCommand, flag string) CRStringSlice {
	return CRStringSlice{
		Val:          c.StringSlice(flag),
		createResult: newCreateResult(c, flag),
	}
}

func newCRString(c *cliconfig.PodmanCommand, flag string) CRString {
	return CRString{
		Val:          c.String(flag),
		createResult: newCreateResult(c, flag),
	}
}

func newCRUint64(c *cliconfig.PodmanCommand, flag string) CRUint64 {
	return CRUint64{
		Val:          c.Uint64(flag),
		createResult: newCreateResult(c, flag),
	}
}

func newCRFloat64(c *cliconfig.PodmanCommand, flag string) CRFloat64 {
	return CRFloat64{
		Val:          c.Float64(flag),
		createResult: newCreateResult(c, flag),
	}
}

func newCRBool(c *cliconfig.PodmanCommand, flag string) CRBool {
	return CRBool{
		Val:          c.Bool(flag),
		createResult: newCreateResult(c, flag),
	}
}

func newCRInt64(c *cliconfig.PodmanCommand, flag string) CRInt64 {
	return CRInt64{
		Val:          c.Int64(flag),
		createResult: newCreateResult(c, flag),
	}
}

func newCRUint(c *cliconfig.PodmanCommand, flag string) CRUint {
	return CRUint{
		Val:          c.Uint(flag),
		createResult: newCreateResult(c, flag),
	}
}

func newCRInt(c *cliconfig.PodmanCommand, flag string) CRInt {
	return CRInt{
		Val:          c.Int(flag),
		createResult: newCreateResult(c, flag),
	}
}

func newCRStringArray(c *cliconfig.PodmanCommand, flag string) CRStringArray {
	return CRStringArray{
		Val:          c.StringArray(flag),
		createResult: newCreateResult(c, flag),
	}
}

// NewIntermediateLayer creates a GenericCLIResults from a create or run cli-command
func NewIntermediateLayer(c *cliconfig.PodmanCommand, remote bool) GenericCLIResults {
	m := make(map[string]GenericCLIResult)

	m["add-host"] = newCRStringSlice(c, "add-host")
	m["annotation"] = newCRStringSlice(c, "annotation")
	m["attach"] = newCRStringSlice(c, "attach")
	m["blkio-weight"] = newCRString(c, "blkio-weight")
	m["blkio-weight-device"] = newCRStringSlice(c, "blkio-weight-device")
	m["cap-add"] = newCRStringSlice(c, "cap-add")
	m["cap-drop"] = newCRStringSlice(c, "cap-drop")
	m["cgroupns"] = newCRString(c, "cgroupns")
	m["cgroups"] = newCRString(c, "cgroups")
	m["cgroup-parent"] = newCRString(c, "cgroup-parent")
	m["cidfile"] = newCRString(c, "cidfile")
	m["conmon-pidfile"] = newCRString(c, "conmon-pidfile")
	m["cpu-period"] = newCRUint64(c, "cpu-period")
	m["cpu-quota"] = newCRInt64(c, "cpu-quota")
	m["cpu-rt-period"] = newCRUint64(c, "cpu-rt-period")
	m["cpu-rt-runtime"] = newCRInt64(c, "cpu-rt-runtime")
	m["cpu-shares"] = newCRUint64(c, "cpu-shares")
	m["cpus"] = newCRFloat64(c, "cpus")
	m["cpuset-cpus"] = newCRString(c, "cpuset-cpus")
	m["cpuset-mems"] = newCRString(c, "cpuset-mems")
	m["detach"] = newCRBool(c, "detach")
	m["detach-keys"] = newCRString(c, "detach-keys")
	m["device"] = newCRStringSlice(c, "device")
	m["device-read-bps"] = newCRStringSlice(c, "device-read-bps")
	m["device-read-iops"] = newCRStringSlice(c, "device-read-iops")
	m["device-write-bps"] = newCRStringSlice(c, "device-write-bps")
	m["device-write-iops"] = newCRStringSlice(c, "device-write-iops")
	m["dns"] = newCRStringSlice(c, "dns")
	m["dns-opt"] = newCRStringSlice(c, "dns-opt")
	m["dns-search"] = newCRStringSlice(c, "dns-search")
	m["entrypoint"] = newCRString(c, "entrypoint")
	m["env"] = newCRStringArray(c, "env")
	m["env-file"] = newCRStringSlice(c, "env-file")
	m["expose"] = newCRStringSlice(c, "expose")
	m["gidmap"] = newCRStringSlice(c, "gidmap")
	m["group-add"] = newCRStringSlice(c, "group-add")
	m["help"] = newCRBool(c, "help")
	m["healthcheck-command"] = newCRString(c, "health-cmd")
	m["healthcheck-interval"] = newCRString(c, "health-interval")
	m["healthcheck-retries"] = newCRUint(c, "health-retries")
	m["healthcheck-start-period"] = newCRString(c, "health-start-period")
	m["healthcheck-timeout"] = newCRString(c, "health-timeout")
	m["hostname"] = newCRString(c, "hostname")
	m["image-volume"] = newCRString(c, "image-volume")
	m["init"] = newCRBool(c, "init")
	m["init-path"] = newCRString(c, "init-path")
	m["interactive"] = newCRBool(c, "interactive")
	m["ip"] = newCRString(c, "ip")
	m["ipc"] = newCRString(c, "ipc")
	m["kernel-memory"] = newCRString(c, "kernel-memory")
	m["label"] = newCRStringArray(c, "label")
	m["label-file"] = newCRStringSlice(c, "label-file")
	m["log-driver"] = newCRString(c, "log-driver")
	m["log-opt"] = newCRStringSlice(c, "log-opt")
	m["mac-address"] = newCRString(c, "mac-address")
	m["memory"] = newCRString(c, "memory")
	m["memory-reservation"] = newCRString(c, "memory-reservation")
	m["memory-swap"] = newCRString(c, "memory-swap")
	m["memory-swappiness"] = newCRInt64(c, "memory-swappiness")
	m["name"] = newCRString(c, "name")
	m["net"] = newCRString(c, "net")
	m["network"] = newCRString(c, "network")
	m["no-hosts"] = newCRBool(c, "no-hosts")
	m["oom-kill-disable"] = newCRBool(c, "oom-kill-disable")
	m["oom-score-adj"] = newCRInt(c, "oom-score-adj")
	m["pid"] = newCRString(c, "pid")
	m["pids-limit"] = newCRInt64(c, "pids-limit")
	m["pod"] = newCRString(c, "pod")
	m["privileged"] = newCRBool(c, "privileged")
	m["publish"] = newCRStringSlice(c, "publish")
	m["publish-all"] = newCRBool(c, "publish-all")
	m["pull"] = newCRString(c, "pull")
	m["quiet"] = newCRBool(c, "quiet")
	m["read-only"] = newCRBool(c, "read-only")
	m["read-only-tmpfs"] = newCRBool(c, "read-only-tmpfs")
	m["restart"] = newCRString(c, "restart")
	m["rm"] = newCRBool(c, "rm")
	m["rootfs"] = newCRBool(c, "rootfs")
	m["security-opt"] = newCRStringArray(c, "security-opt")
	m["shm-size"] = newCRString(c, "shm-size")
	m["stop-signal"] = newCRString(c, "stop-signal")
	m["stop-timeout"] = newCRUint(c, "stop-timeout")
	m["storage-opt"] = newCRStringSlice(c, "storage-opt")
	m["subgidname"] = newCRString(c, "subgidname")
	m["subuidname"] = newCRString(c, "subuidname")
	m["sysctl"] = newCRStringSlice(c, "sysctl")
	m["systemd"] = newCRBool(c, "systemd")
	m["tmpfs"] = newCRStringArray(c, "tmpfs")
	m["tty"] = newCRBool(c, "tty")
	m["uidmap"] = newCRStringSlice(c, "uidmap")
	m["ulimit"] = newCRStringSlice(c, "ulimit")
	m["user"] = newCRString(c, "user")
	m["userns"] = newCRString(c, "userns")
	m["uts"] = newCRString(c, "uts")
	m["mount"] = newCRStringArray(c, "mount")
	m["volume"] = newCRStringArray(c, "volume")
	m["volumes-from"] = newCRStringSlice(c, "volumes-from")
	m["workdir"] = newCRString(c, "workdir")
	// global flag
	if !remote {
		m["authfile"] = newCRString(c, "authfile")
		m["cgroupns"] = newCRString(c, "cgroupns")
		m["env-host"] = newCRBool(c, "env-host")
		m["http-proxy"] = newCRBool(c, "http-proxy")
		m["trace"] = newCRBool(c, "trace")
		m["syslog"] = newCRBool(c, "syslog")
	}

	return GenericCLIResults{m, c.InputArgs}
}
