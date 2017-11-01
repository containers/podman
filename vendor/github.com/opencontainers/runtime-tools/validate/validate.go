package validate

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"

	"github.com/opencontainers/runtime-tools/specerror"
)

const specConfig = "config.json"

var (
	defaultRlimits = []string{
		"RLIMIT_AS",
		"RLIMIT_CORE",
		"RLIMIT_CPU",
		"RLIMIT_DATA",
		"RLIMIT_FSIZE",
		"RLIMIT_LOCKS",
		"RLIMIT_MEMLOCK",
		"RLIMIT_MSGQUEUE",
		"RLIMIT_NICE",
		"RLIMIT_NOFILE",
		"RLIMIT_NPROC",
		"RLIMIT_RSS",
		"RLIMIT_RTPRIO",
		"RLIMIT_RTTIME",
		"RLIMIT_SIGPENDING",
		"RLIMIT_STACK",
	}
)

// Validator represents a validator for runtime bundle
type Validator struct {
	spec         *rspec.Spec
	bundlePath   string
	HostSpecific bool
	platform     string
}

// NewValidator creates a Validator
func NewValidator(spec *rspec.Spec, bundlePath string, hostSpecific bool, platform string) Validator {
	if hostSpecific && platform != runtime.GOOS {
		platform = runtime.GOOS
	}
	return Validator{
		spec:         spec,
		bundlePath:   bundlePath,
		HostSpecific: hostSpecific,
		platform:     platform,
	}
}

// NewValidatorFromPath creates a Validator with specified bundle path
func NewValidatorFromPath(bundlePath string, hostSpecific bool, platform string) (Validator, error) {
	if hostSpecific && platform != runtime.GOOS {
		platform = runtime.GOOS
	}
	if bundlePath == "" {
		return Validator{}, fmt.Errorf("bundle path shouldn't be empty")
	}

	if _, err := os.Stat(bundlePath); err != nil {
		return Validator{}, err
	}

	configPath := filepath.Join(bundlePath, specConfig)
	content, err := ioutil.ReadFile(configPath)
	if err != nil {
		return Validator{}, specerror.NewError(specerror.ConfigFileExistence, err, rspec.Version)
	}
	if !utf8.Valid(content) {
		return Validator{}, fmt.Errorf("%q is not encoded in UTF-8", configPath)
	}
	var spec rspec.Spec
	if err = json.Unmarshal(content, &spec); err != nil {
		return Validator{}, err
	}

	return NewValidator(&spec, bundlePath, hostSpecific, platform), nil
}

// CheckAll checks all parts of runtime bundle
func (v *Validator) CheckAll() (errs error) {
	errs = multierror.Append(errs, v.CheckPlatform())
	errs = multierror.Append(errs, v.CheckRoot())
	errs = multierror.Append(errs, v.CheckMandatoryFields())
	errs = multierror.Append(errs, v.CheckSemVer())
	errs = multierror.Append(errs, v.CheckMounts())
	errs = multierror.Append(errs, v.CheckProcess())
	errs = multierror.Append(errs, v.CheckHooks())
	errs = multierror.Append(errs, v.CheckLinux())

	return
}

// CheckRoot checks status of v.spec.Root
func (v *Validator) CheckRoot() (errs error) {
	logrus.Debugf("check root")

	if v.platform == "windows" && v.spec.Windows != nil && v.spec.Windows.HyperV != nil {
		if v.spec.Root != nil {
			errs = multierror.Append(errs,
				specerror.NewError(specerror.RootOnHyperV, fmt.Errorf("for Hyper-V containers, Root must not be set"), rspec.Version))
			return
		}
		return
	} else if v.spec.Root == nil {
		errs = multierror.Append(errs,
			specerror.NewError(specerror.RootOnNonHyperV, fmt.Errorf("for non-Hyper-V containers, Root must be set"), rspec.Version))
		return
	}

	absBundlePath, err := filepath.Abs(v.bundlePath)
	if err != nil {
		errs = multierror.Append(errs, fmt.Errorf("unable to convert %q to an absolute path", v.bundlePath))
		return
	}

	if filepath.Base(v.spec.Root.Path) != "rootfs" {
		errs = multierror.Append(errs,
			specerror.NewError(specerror.PathName, fmt.Errorf("path name should be the conventional 'rootfs'"), rspec.Version))
	}

	var rootfsPath string
	var absRootPath string
	if filepath.IsAbs(v.spec.Root.Path) {
		rootfsPath = v.spec.Root.Path
		absRootPath = filepath.Clean(rootfsPath)
	} else {
		var err error
		rootfsPath = filepath.Join(v.bundlePath, v.spec.Root.Path)
		absRootPath, err = filepath.Abs(rootfsPath)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("unable to convert %q to an absolute path", rootfsPath))
			return
		}
	}

	if fi, err := os.Stat(rootfsPath); err != nil {
		errs = multierror.Append(errs,
			specerror.NewError(specerror.PathExistence, fmt.Errorf("cannot find the root path %q", rootfsPath), rspec.Version))
	} else if !fi.IsDir() {
		errs = multierror.Append(errs,
			specerror.NewError(specerror.PathExistence, fmt.Errorf("root.path %q is not a directory", rootfsPath), rspec.Version))
	}

	rootParent := filepath.Dir(absRootPath)
	if absRootPath == string(filepath.Separator) || rootParent != absBundlePath {
		errs = multierror.Append(errs,
			specerror.NewError(specerror.ArtifactsInSingleDir, fmt.Errorf("root.path is %q, but it MUST be a child of %q", v.spec.Root.Path, absBundlePath), rspec.Version))
	}

	if v.platform == "windows" {
		if v.spec.Root.Readonly {
			errs = multierror.Append(errs,
				specerror.NewError(specerror.ReadonlyOnWindows, fmt.Errorf("root.readonly field MUST be omitted or false when target platform is windows"), rspec.Version))
		}
	}

	return
}

// CheckSemVer checks v.spec.Version
func (v *Validator) CheckSemVer() (errs error) {
	logrus.Debugf("check semver")

	version := v.spec.Version
	_, err := semver.Parse(version)
	if err != nil {
		errs = multierror.Append(errs,
			specerror.NewError(specerror.SpecVersion, fmt.Errorf("%q is not valid SemVer: %s", version, err.Error()), rspec.Version))
	}
	if version != rspec.Version {
		errs = multierror.Append(errs, fmt.Errorf("validate currently only handles version %s, but the supplied configuration targets %s", rspec.Version, version))
	}

	return
}

// CheckHooks check v.spec.Hooks
func (v *Validator) CheckHooks() (errs error) {
	logrus.Debugf("check hooks")

	if v.spec.Hooks != nil {
		errs = multierror.Append(errs, checkEventHooks("pre-start", v.spec.Hooks.Prestart, v.HostSpecific))
		errs = multierror.Append(errs, checkEventHooks("post-start", v.spec.Hooks.Poststart, v.HostSpecific))
		errs = multierror.Append(errs, checkEventHooks("post-stop", v.spec.Hooks.Poststop, v.HostSpecific))
	}

	return
}

func checkEventHooks(hookType string, hooks []rspec.Hook, hostSpecific bool) (errs error) {
	for _, hook := range hooks {
		if !filepath.IsAbs(hook.Path) {
			errs = multierror.Append(errs, fmt.Errorf("the %s hook %v: is not absolute path", hookType, hook.Path))
		}

		if hostSpecific {
			fi, err := os.Stat(hook.Path)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("cannot find %s hook: %v", hookType, hook.Path))
			}
			if fi.Mode()&0111 == 0 {
				errs = multierror.Append(errs, fmt.Errorf("the %s hook %v: is not executable", hookType, hook.Path))
			}
		}

		for _, env := range hook.Env {
			if !envValid(env) {
				errs = multierror.Append(errs, fmt.Errorf("env %q for hook %v is in the invalid form", env, hook.Path))
			}
		}
	}

	return
}

// CheckProcess checks v.spec.Process
func (v *Validator) CheckProcess() (errs error) {
	logrus.Debugf("check process")

	if v.spec.Process == nil {
		return
	}

	process := v.spec.Process
	if !filepath.IsAbs(process.Cwd) {
		errs = multierror.Append(errs, fmt.Errorf("cwd %q is not an absolute path", process.Cwd))
	}

	for _, env := range process.Env {
		if !envValid(env) {
			errs = multierror.Append(errs, fmt.Errorf("env %q should be in the form of 'key=value'. The left hand side must consist solely of letters, digits, and underscores '_'", env))
		}
	}

	if len(process.Args) == 0 {
		errs = multierror.Append(errs, fmt.Errorf("args must not be empty"))
	} else {
		if filepath.IsAbs(process.Args[0]) {
			var rootfsPath string
			if filepath.IsAbs(v.spec.Root.Path) {
				rootfsPath = v.spec.Root.Path
			} else {
				rootfsPath = filepath.Join(v.bundlePath, v.spec.Root.Path)
			}
			absPath := filepath.Join(rootfsPath, process.Args[0])
			fileinfo, err := os.Stat(absPath)
			if os.IsNotExist(err) {
				logrus.Warnf("executable %q is not available in rootfs currently", process.Args[0])
			} else if err != nil {
				errs = multierror.Append(errs, err)
			} else {
				m := fileinfo.Mode()
				if m.IsDir() || m&0111 == 0 {
					errs = multierror.Append(errs, fmt.Errorf("arg %q is not executable", process.Args[0]))
				}
			}
		}
	}

	if v.spec.Process.Capabilities != nil {
		errs = multierror.Append(errs, v.CheckCapabilities())
	}
	errs = multierror.Append(errs, v.CheckRlimits())

	if v.platform == "linux" {
		if len(process.ApparmorProfile) > 0 {
			profilePath := filepath.Join(v.bundlePath, v.spec.Root.Path, "/etc/apparmor.d", process.ApparmorProfile)
			_, err := os.Stat(profilePath)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
		}
	}

	return
}

// CheckCapabilities checks v.spec.Process.Capabilities
func (v *Validator) CheckCapabilities() (errs error) {
	process := v.spec.Process
	if v.platform == "linux" {
		var effective, permitted, inheritable, ambient bool
		caps := make(map[string][]string)

		for _, cap := range process.Capabilities.Bounding {
			caps[cap] = append(caps[cap], "bounding")
		}
		for _, cap := range process.Capabilities.Effective {
			caps[cap] = append(caps[cap], "effective")
		}
		for _, cap := range process.Capabilities.Inheritable {
			caps[cap] = append(caps[cap], "inheritable")
		}
		for _, cap := range process.Capabilities.Permitted {
			caps[cap] = append(caps[cap], "permitted")
		}
		for _, cap := range process.Capabilities.Ambient {
			caps[cap] = append(caps[cap], "ambient")
		}

		for capability, owns := range caps {
			if err := CapValid(capability, v.HostSpecific); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("capability %q is not valid, man capabilities(7)", capability))
			}

			effective, permitted, ambient, inheritable = false, false, false, false
			for _, set := range owns {
				if set == "effective" {
					effective = true
					continue
				}
				if set == "inheritable" {
					inheritable = true
					continue
				}
				if set == "permitted" {
					permitted = true
					continue
				}
				if set == "ambient" {
					ambient = true
					continue
				}
			}
			if effective && !permitted {
				errs = multierror.Append(errs, fmt.Errorf("effective capability %q is not allowed, as it's not permitted", capability))
			}
			if ambient && !(effective && inheritable) {
				errs = multierror.Append(errs, fmt.Errorf("ambient capability %q is not allowed, as it's not permitted and inheribate", capability))
			}
		}
	} else {
		logrus.Warnf("process.capabilities validation not yet implemented for OS %q", v.platform)
	}

	return
}

// CheckRlimits checks v.spec.Process.Rlimits
func (v *Validator) CheckRlimits() (errs error) {
	process := v.spec.Process
	for index, rlimit := range process.Rlimits {
		for i := index + 1; i < len(process.Rlimits); i++ {
			if process.Rlimits[index].Type == process.Rlimits[i].Type {
				errs = multierror.Append(errs, fmt.Errorf("rlimit can not contain the same type %q", process.Rlimits[index].Type))
			}
		}
		errs = multierror.Append(errs, v.rlimitValid(rlimit))
	}

	return
}

func supportedMountTypes(OS string, hostSpecific bool) (map[string]bool, error) {
	supportedTypes := make(map[string]bool)

	if OS != "linux" && OS != "windows" {
		logrus.Warnf("%v is not supported to check mount type", OS)
		return nil, nil
	} else if OS == "windows" {
		supportedTypes["ntfs"] = true
		return supportedTypes, nil
	}

	if hostSpecific {
		f, err := os.Open("/proc/filesystems")
		if err != nil {
			return nil, err
		}
		defer f.Close()

		s := bufio.NewScanner(f)
		for s.Scan() {
			if err := s.Err(); err != nil {
				return supportedTypes, err
			}

			text := s.Text()
			parts := strings.Split(text, "\t")
			if len(parts) > 1 {
				supportedTypes[parts[1]] = true
			} else {
				supportedTypes[parts[0]] = true
			}
		}

		supportedTypes["bind"] = true

		return supportedTypes, nil
	}
	logrus.Warn("Checking linux mount types without --host-specific is not supported yet")
	return nil, nil
}

// CheckMounts checks v.spec.Mounts
func (v *Validator) CheckMounts() (errs error) {
	logrus.Debugf("check mounts")

	supportedTypes, err := supportedMountTypes(v.platform, v.HostSpecific)
	if err != nil {
		errs = multierror.Append(errs, err)
		return
	}

	for i, mountA := range v.spec.Mounts {
		if supportedTypes != nil && !supportedTypes[mountA.Type] {
			errs = multierror.Append(errs, fmt.Errorf("unsupported mount type %q", mountA.Type))
		}
		if v.platform == "windows" {
			if err := pathValid(v.platform, mountA.Destination); err != nil {
				errs = multierror.Append(errs, err)
			}
			if err := pathValid(v.platform, mountA.Source); err != nil {
				errs = multierror.Append(errs, err)
			}
		} else {
			if err := pathValid(v.platform, mountA.Destination); err != nil {
				errs = multierror.Append(errs, err)
			}
		}
		for j, mountB := range v.spec.Mounts {
			if i == j {
				continue
			}
			// whether B.Desination is nested within A.Destination
			nested, err := nestedValid(v.platform, mountA.Destination, mountB.Destination)
			if err != nil {
				errs = multierror.Append(errs, err)
				continue
			}
			if nested {
				if v.platform == "windows" && i < j {
					errs = multierror.Append(errs, fmt.Errorf("on Windows, %v nested within %v is forbidden", mountB.Destination, mountA.Destination))
				}
				if i > j {
					logrus.Warnf("%v will be covered by %v", mountB.Destination, mountA.Destination)
				}
			}
		}
	}

	return
}

// CheckPlatform checks v.platform
func (v *Validator) CheckPlatform() (errs error) {
	logrus.Debugf("check platform")

	if v.platform != "linux" && v.platform != "solaris" && v.platform != "windows" {
		errs = multierror.Append(errs, fmt.Errorf("platform %q is not supported", v.platform))
		return
	}

	if v.platform == "windows" {
		if v.spec.Windows == nil {
			errs = multierror.Append(errs, errors.New("'windows' MUST be set when platform is `windows`"))
		}
	}

	return
}

// CheckLinux checks v.spec.Linux
func (v *Validator) CheckLinux() (errs error) {
	logrus.Debugf("check linux")

	if v.spec.Linux == nil {
		return
	}

	var nsTypeList = map[rspec.LinuxNamespaceType]struct {
		num      int
		newExist bool
	}{
		rspec.PIDNamespace:     {0, false},
		rspec.NetworkNamespace: {0, false},
		rspec.MountNamespace:   {0, false},
		rspec.IPCNamespace:     {0, false},
		rspec.UTSNamespace:     {0, false},
		rspec.UserNamespace:    {0, false},
		rspec.CgroupNamespace:  {0, false},
	}

	for index := 0; index < len(v.spec.Linux.Namespaces); index++ {
		ns := v.spec.Linux.Namespaces[index]
		if !namespaceValid(ns) {
			errs = multierror.Append(errs, fmt.Errorf("namespace %v is invalid", ns))
		}

		tmpItem := nsTypeList[ns.Type]
		tmpItem.num = tmpItem.num + 1
		if tmpItem.num > 1 {
			errs = multierror.Append(errs, fmt.Errorf("duplicated namespace %q", ns.Type))
		}

		if len(ns.Path) == 0 {
			tmpItem.newExist = true
		}
		nsTypeList[ns.Type] = tmpItem
	}

	if (len(v.spec.Linux.UIDMappings) > 0 || len(v.spec.Linux.GIDMappings) > 0) && !nsTypeList[rspec.UserNamespace].newExist {
		errs = multierror.Append(errs, errors.New("the UID/GID mappings requires a new User namespace to be specified as well"))
	} else if len(v.spec.Linux.UIDMappings) > 5 {
		errs = multierror.Append(errs, errors.New("only 5 UID mappings are allowed (linux kernel restriction)"))
	} else if len(v.spec.Linux.GIDMappings) > 5 {
		errs = multierror.Append(errs, errors.New("only 5 GID mappings are allowed (linux kernel restriction)"))
	}

	for k := range v.spec.Linux.Sysctl {
		if strings.HasPrefix(k, "net.") && !nsTypeList[rspec.NetworkNamespace].newExist {
			errs = multierror.Append(errs, fmt.Errorf("sysctl %v requires a new Network namespace to be specified as well", k))
		}
		if strings.HasPrefix(k, "fs.mqueue.") {
			if !nsTypeList[rspec.MountNamespace].newExist || !nsTypeList[rspec.IPCNamespace].newExist {
				errs = multierror.Append(errs, fmt.Errorf("sysctl %v requires a new IPC namespace and Mount namespace to be specified as well", k))
			}
		}
	}

	if v.platform == "linux" && !nsTypeList[rspec.UTSNamespace].newExist && v.spec.Hostname != "" {
		errs = multierror.Append(errs, fmt.Errorf("on Linux, hostname requires a new UTS namespace to be specified as well"))
	}

	// Linux devices validation
	devList := make(map[string]bool)
	devTypeList := make(map[string]bool)
	for index := 0; index < len(v.spec.Linux.Devices); index++ {
		device := v.spec.Linux.Devices[index]
		if !deviceValid(device) {
			errs = multierror.Append(errs, fmt.Errorf("device %v is invalid", device))
		}

		if _, exists := devList[device.Path]; exists {
			errs = multierror.Append(errs, fmt.Errorf("device %s is duplicated", device.Path))
		} else {
			var rootfsPath string
			if filepath.IsAbs(v.spec.Root.Path) {
				rootfsPath = v.spec.Root.Path
			} else {
				rootfsPath = filepath.Join(v.bundlePath, v.spec.Root.Path)
			}
			absPath := filepath.Join(rootfsPath, device.Path)
			fi, err := os.Stat(absPath)
			if os.IsNotExist(err) {
				devList[device.Path] = true
			} else if err != nil {
				errs = multierror.Append(errs, err)
			} else {
				fStat, ok := fi.Sys().(*syscall.Stat_t)
				if !ok {
					errs = multierror.Append(errs, fmt.Errorf("cannot determine state for device %s", device.Path))
					continue
				}
				var devType string
				switch fStat.Mode & syscall.S_IFMT {
				case syscall.S_IFCHR:
					devType = "c"
				case syscall.S_IFBLK:
					devType = "b"
				case syscall.S_IFIFO:
					devType = "p"
				default:
					devType = "unmatched"
				}
				if devType != device.Type || (devType == "c" && device.Type == "u") {
					errs = multierror.Append(errs, fmt.Errorf("unmatched %s already exists in filesystem", device.Path))
					continue
				}
				if devType != "p" {
					dev := fStat.Rdev
					major := (dev >> 8) & 0xfff
					minor := (dev & 0xff) | ((dev >> 12) & 0xfff00)
					if int64(major) != device.Major || int64(minor) != device.Minor {
						errs = multierror.Append(errs, fmt.Errorf("unmatched %s already exists in filesystem", device.Path))
						continue
					}
				}
				if device.FileMode != nil {
					expectedPerm := *device.FileMode & os.ModePerm
					actualPerm := fi.Mode() & os.ModePerm
					if expectedPerm != actualPerm {
						errs = multierror.Append(errs, fmt.Errorf("unmatched %s already exists in filesystem", device.Path))
						continue
					}
				}
				if device.UID != nil {
					if *device.UID != fStat.Uid {
						errs = multierror.Append(errs, fmt.Errorf("unmatched %s already exists in filesystem", device.Path))
						continue
					}
				}
				if device.GID != nil {
					if *device.GID != fStat.Gid {
						errs = multierror.Append(errs, fmt.Errorf("unmatched %s already exists in filesystem", device.Path))
						continue
					}
				}
			}
		}

		// unify u->c when comparing, they are synonyms
		var devID string
		if device.Type == "u" {
			devID = fmt.Sprintf("%s:%d:%d", "c", device.Major, device.Minor)
		} else {
			devID = fmt.Sprintf("%s:%d:%d", device.Type, device.Major, device.Minor)
		}

		if _, exists := devTypeList[devID]; exists {
			logrus.Warnf("type:%s, major:%d and minor:%d for linux devices is duplicated", device.Type, device.Major, device.Minor)
		} else {
			devTypeList[devID] = true
		}
	}

	if v.spec.Linux.Resources != nil {
		errs = multierror.Append(errs, v.CheckLinuxResources())
	}

	if v.spec.Linux.Seccomp != nil {
		errs = multierror.Append(errs, v.CheckSeccomp())
	}

	switch v.spec.Linux.RootfsPropagation {
	case "":
	case "private":
	case "rprivate":
	case "slave":
	case "rslave":
	case "shared":
	case "rshared":
	case "unbindable":
	case "runbindable":
	default:
		errs = multierror.Append(errs, errors.New("rootfsPropagation must be empty or one of \"private|rprivate|slave|rslave|shared|rshared|unbindable|runbindable\""))
	}

	for _, maskedPath := range v.spec.Linux.MaskedPaths {
		if !strings.HasPrefix(maskedPath, "/") {
			errs = multierror.Append(errs, fmt.Errorf("maskedPath %v is not an absolute path", maskedPath))
		}
	}

	for _, readonlyPath := range v.spec.Linux.ReadonlyPaths {
		if !strings.HasPrefix(readonlyPath, "/") {
			errs = multierror.Append(errs, fmt.Errorf("readonlyPath %v is not an absolute path", readonlyPath))
		}
	}

	return
}

// CheckLinuxResources checks v.spec.Linux.Resources
func (v *Validator) CheckLinuxResources() (errs error) {
	logrus.Debugf("check linux resources")

	r := v.spec.Linux.Resources
	if r.Memory != nil {
		if r.Memory.Limit != nil && r.Memory.Swap != nil && uint64(*r.Memory.Limit) > uint64(*r.Memory.Swap) {
			errs = multierror.Append(errs, fmt.Errorf("minimum memoryswap should be larger than memory limit"))
		}
		if r.Memory.Limit != nil && r.Memory.Reservation != nil && uint64(*r.Memory.Reservation) > uint64(*r.Memory.Limit) {
			errs = multierror.Append(errs, fmt.Errorf("minimum memory limit should be larger than memory reservation"))
		}
	}
	if r.Network != nil && v.HostSpecific {
		var exist bool
		interfaces, err := net.Interfaces()
		if err != nil {
			errs = multierror.Append(errs, err)
			return
		}
		for _, prio := range r.Network.Priorities {
			exist = false
			for _, ni := range interfaces {
				if prio.Name == ni.Name {
					exist = true
					break
				}
			}
			if !exist {
				errs = multierror.Append(errs, fmt.Errorf("interface %s does not exist currently", prio.Name))
			}
		}
	}
	for index := 0; index < len(r.Devices); index++ {
		switch r.Devices[index].Type {
		case "a", "b", "c":
		default:
			errs = multierror.Append(errs, fmt.Errorf("type of devices %s is invalid", r.Devices[index].Type))
		}

		access := []byte(r.Devices[index].Access)
		for i := 0; i < len(access); i++ {
			switch access[i] {
			case 'r', 'w', 'm':
			default:
				errs = multierror.Append(errs, fmt.Errorf("access %s is invalid", r.Devices[index].Access))
				return
			}
		}
	}

	return
}

// CheckSeccomp checkc v.spec.Linux.Seccomp
func (v *Validator) CheckSeccomp() (errs error) {
	logrus.Debugf("check linux seccomp")

	s := v.spec.Linux.Seccomp
	if !seccompActionValid(s.DefaultAction) {
		errs = multierror.Append(errs, fmt.Errorf("seccomp defaultAction %q is invalid", s.DefaultAction))
	}
	for index := 0; index < len(s.Syscalls); index++ {
		if !syscallValid(s.Syscalls[index]) {
			errs = multierror.Append(errs, fmt.Errorf("syscall %v is invalid", s.Syscalls[index]))
		}
	}
	for index := 0; index < len(s.Architectures); index++ {
		switch s.Architectures[index] {
		case rspec.ArchX86:
		case rspec.ArchX86_64:
		case rspec.ArchX32:
		case rspec.ArchARM:
		case rspec.ArchAARCH64:
		case rspec.ArchMIPS:
		case rspec.ArchMIPS64:
		case rspec.ArchMIPS64N32:
		case rspec.ArchMIPSEL:
		case rspec.ArchMIPSEL64:
		case rspec.ArchMIPSEL64N32:
		case rspec.ArchPPC:
		case rspec.ArchPPC64:
		case rspec.ArchPPC64LE:
		case rspec.ArchS390:
		case rspec.ArchS390X:
		case rspec.ArchPARISC:
		case rspec.ArchPARISC64:
		default:
			errs = multierror.Append(errs, fmt.Errorf("seccomp architecture %q is invalid", s.Architectures[index]))
		}
	}

	return
}

// CapValid checks whether a capability is valid
func CapValid(c string, hostSpecific bool) error {
	isValid := false

	if !strings.HasPrefix(c, "CAP_") {
		return fmt.Errorf("capability %s must start with CAP_", c)
	}
	for _, cap := range capability.List() {
		if c == fmt.Sprintf("CAP_%s", strings.ToUpper(cap.String())) {
			if hostSpecific && cap > LastCap() {
				return fmt.Errorf("%s is not supported on the current host", c)
			}
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid capability: %s", c)
	}
	return nil
}

// LastCap return last cap of system
func LastCap() capability.Cap {
	last := capability.CAP_LAST_CAP
	// hack for RHEL6 which has no /proc/sys/kernel/cap_last_cap
	if last == capability.Cap(63) {
		last = capability.CAP_BLOCK_SUSPEND
	}

	return last
}

func envValid(env string) bool {
	items := strings.Split(env, "=")
	if len(items) < 2 {
		return false
	}
	for i, ch := range strings.TrimSpace(items[0]) {
		if !unicode.IsDigit(ch) && !unicode.IsLetter(ch) && ch != '_' {
			return false
		}
		if i == 0 && unicode.IsDigit(ch) {
			logrus.Warnf("Env %v: variable name beginning with digit is not recommended.", env)
		}
	}
	return true
}

func (v *Validator) rlimitValid(rlimit rspec.POSIXRlimit) (errs error) {
	if rlimit.Hard < rlimit.Soft {
		errs = multierror.Append(errs, fmt.Errorf("hard limit of rlimit %s should not be less than soft limit", rlimit.Type))
	}

	if v.platform == "linux" {
		for _, val := range defaultRlimits {
			if val == rlimit.Type {
				return
			}
		}
		errs = multierror.Append(errs, fmt.Errorf("rlimit type %q is invalid", rlimit.Type))
	} else {
		logrus.Warnf("process.rlimits validation not yet implemented for platform %q", v.platform)
	}

	return
}

func namespaceValid(ns rspec.LinuxNamespace) bool {
	switch ns.Type {
	case rspec.PIDNamespace:
	case rspec.NetworkNamespace:
	case rspec.MountNamespace:
	case rspec.IPCNamespace:
	case rspec.UTSNamespace:
	case rspec.UserNamespace:
	case rspec.CgroupNamespace:
	default:
		return false
	}

	if ns.Path != "" && !filepath.IsAbs(ns.Path) {
		return false
	}

	return true
}

func pathValid(os, path string) error {
	if os == "windows" {
		matched, err := regexp.MatchString("^[a-zA-Z]:(\\\\[^\\\\/<>|:*?\"]+)+$", path)
		if err != nil {
			return err
		}
		if !matched {
			return fmt.Errorf("invalid windows path %v", path)
		}
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%v is not an absolute path", path)
	}
	return nil
}

// Check whether pathB is nested whithin pathA
func nestedValid(os, pathA, pathB string) (bool, error) {
	if pathA == pathB {
		return false, nil
	}
	if pathA == "/" && pathB != "" {
		return true, nil
	}

	var sep string
	if os == "windows" {
		sep = "\\"
	} else {
		sep = "/"
	}

	splitedPathA := strings.Split(filepath.Clean(pathA), sep)
	splitedPathB := strings.Split(filepath.Clean(pathB), sep)
	lenA := len(splitedPathA)
	lenB := len(splitedPathB)

	if lenA > lenB {
		if (lenA - lenB) == 1 {
			// if pathA is longer but not end with separator
			if splitedPathA[lenA-1] != "" {
				return false, nil
			}
			splitedPathA = splitedPathA[:lenA-1]
		} else {
			return false, nil
		}
	}

	for i, partA := range splitedPathA {
		if partA != splitedPathB[i] {
			return false, nil
		}
	}

	return true, nil
}

func deviceValid(d rspec.LinuxDevice) bool {
	switch d.Type {
	case "b", "c", "u":
		if d.Major <= 0 || d.Minor <= 0 {
			return false
		}
	case "p":
		if d.Major > 0 || d.Minor > 0 {
			return false
		}
	default:
		return false
	}
	return true
}

func seccompActionValid(secc rspec.LinuxSeccompAction) bool {
	switch secc {
	case "":
	case rspec.ActKill:
	case rspec.ActTrap:
	case rspec.ActErrno:
	case rspec.ActTrace:
	case rspec.ActAllow:
	default:
		return false
	}
	return true
}

func syscallValid(s rspec.LinuxSyscall) bool {
	if !seccompActionValid(s.Action) {
		return false
	}
	for index := 0; index < len(s.Args); index++ {
		arg := s.Args[index]
		switch arg.Op {
		case rspec.OpNotEqual:
		case rspec.OpLessThan:
		case rspec.OpLessEqual:
		case rspec.OpEqualTo:
		case rspec.OpGreaterEqual:
		case rspec.OpGreaterThan:
		case rspec.OpMaskedEqual:
		default:
			return false
		}
	}
	return true
}

func isStruct(t reflect.Type) bool {
	return t.Kind() == reflect.Struct
}

func isStructPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}

func checkMandatoryUnit(field reflect.Value, tagField reflect.StructField, parent string) (errs error) {
	mandatory := !strings.Contains(tagField.Tag.Get("json"), "omitempty")
	switch field.Kind() {
	case reflect.Ptr:
		if mandatory && field.IsNil() {
			errs = multierror.Append(errs, fmt.Errorf("'%s.%s' should not be empty", parent, tagField.Name))
		}
	case reflect.String:
		if mandatory && (field.Len() == 0) {
			errs = multierror.Append(errs, fmt.Errorf("'%s.%s' should not be empty", parent, tagField.Name))
		}
	case reflect.Slice:
		if mandatory && (field.IsNil() || field.Len() == 0) {
			errs = multierror.Append(errs, fmt.Errorf("'%s.%s' should not be empty", parent, tagField.Name))
			return
		}
		for index := 0; index < field.Len(); index++ {
			mValue := field.Index(index)
			if mValue.CanInterface() {
				errs = multierror.Append(errs, checkMandatory(mValue.Interface()))
			}
		}
	case reflect.Map:
		if mandatory && (field.IsNil() || field.Len() == 0) {
			errs = multierror.Append(errs, fmt.Errorf("'%s.%s' should not be empty", parent, tagField.Name))
			return
		}
		keys := field.MapKeys()
		for index := 0; index < len(keys); index++ {
			mValue := field.MapIndex(keys[index])
			if mValue.CanInterface() {
				errs = multierror.Append(errs, checkMandatory(mValue.Interface()))
			}
		}
	default:
	}

	return
}

func checkMandatory(obj interface{}) (errs error) {
	objT := reflect.TypeOf(obj)
	objV := reflect.ValueOf(obj)
	if isStructPtr(objT) {
		objT = objT.Elem()
		objV = objV.Elem()
	} else if !isStruct(objT) {
		return
	}

	for i := 0; i < objT.NumField(); i++ {
		t := objT.Field(i).Type
		if isStructPtr(t) && objV.Field(i).IsNil() {
			if !strings.Contains(objT.Field(i).Tag.Get("json"), "omitempty") {
				errs = multierror.Append(errs, fmt.Errorf("'%s.%s' should not be empty", objT.Name(), objT.Field(i).Name))
			}
		} else if (isStruct(t) || isStructPtr(t)) && objV.Field(i).CanInterface() {
			errs = multierror.Append(errs, checkMandatory(objV.Field(i).Interface()))
		} else {
			errs = multierror.Append(errs, checkMandatoryUnit(objV.Field(i), objT.Field(i), objT.Name()))
		}

	}
	return
}

// CheckMandatoryFields checks mandatory field of container's config file
func (v *Validator) CheckMandatoryFields() error {
	logrus.Debugf("check mandatory fields")

	return checkMandatory(v.spec)
}
