package parse

// this package should contain functions that parse and validate
// user input and is shared either amongst buildah subcommands or
// would be useful to projects vendoring buildah

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/idtools"
	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/projectatomic/buildah"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	// SeccompDefaultPath defines the default seccomp path
	SeccompDefaultPath = "/usr/share/containers/seccomp.json"
	// SeccompOverridePath if this exists it overrides the default seccomp path
	SeccompOverridePath = "/etc/crio/seccomp.json"
)

// CommonBuildOptions parses the build options from the bud cli
func CommonBuildOptions(c *cli.Context) (*buildah.CommonBuildOptions, error) {
	var (
		memoryLimit int64
		memorySwap  int64
		err         error
	)
	if c.String("memory") != "" {
		memoryLimit, err = units.RAMInBytes(c.String("memory"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory")
		}
	}
	if c.String("memory-swap") != "" {
		memorySwap, err = units.RAMInBytes(c.String("memory-swap"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory-swap")
		}
	}
	if len(c.StringSlice("add-host")) > 0 {
		for _, host := range c.StringSlice("add-host") {
			if err := validateExtraHost(host); err != nil {
				return nil, errors.Wrapf(err, "invalid value for add-host")
			}
		}
	}
	if _, err := units.FromHumanSize(c.String("shm-size")); err != nil {
		return nil, errors.Wrapf(err, "invalid --shm-size")
	}
	if err := ParseVolumes(c.StringSlice("volume")); err != nil {
		return nil, err
	}

	commonOpts := &buildah.CommonBuildOptions{
		AddHost:      c.StringSlice("add-host"),
		CgroupParent: c.String("cgroup-parent"),
		CPUPeriod:    c.Uint64("cpu-period"),
		CPUQuota:     c.Int64("cpu-quota"),
		CPUSetCPUs:   c.String("cpuset-cpus"),
		CPUSetMems:   c.String("cpuset-mems"),
		CPUShares:    c.Uint64("cpu-shares"),
		Memory:       memoryLimit,
		MemorySwap:   memorySwap,
		ShmSize:      c.String("shm-size"),
		Ulimit:       c.StringSlice("ulimit"),
		Volumes:      c.StringSlice("volume"),
	}
	if err := parseSecurityOpts(c.StringSlice("security-opt"), commonOpts); err != nil {
		return nil, err
	}
	return commonOpts, nil
}

func parseSecurityOpts(securityOpts []string, commonOpts *buildah.CommonBuildOptions) error {
	for _, opt := range securityOpts {
		if opt == "no-new-privileges" {
			return errors.Errorf("no-new-privileges is not supported")
		}
		con := strings.SplitN(opt, "=", 2)
		if len(con) != 2 {
			return errors.Errorf("Invalid --security-opt name=value pair: %q", opt)
		}

		switch con[0] {
		case "label":
			commonOpts.LabelOpts = append(commonOpts.LabelOpts, con[1])
		case "apparmor":
			commonOpts.ApparmorProfile = con[1]
		case "seccomp":
			commonOpts.SeccompProfilePath = con[1]
		default:
			return errors.Errorf("Invalid --security-opt 2: %q", opt)
		}

	}

	if commonOpts.SeccompProfilePath == "" {
		if _, err := os.Stat(SeccompOverridePath); err == nil {
			commonOpts.SeccompProfilePath = SeccompOverridePath
		} else {
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "can't check if %q exists", SeccompOverridePath)
			}
			if _, err := os.Stat(SeccompDefaultPath); err != nil {
				if !os.IsNotExist(err) {
					return errors.Wrapf(err, "can't check if %q exists", SeccompDefaultPath)
				}
			} else {
				commonOpts.SeccompProfilePath = SeccompDefaultPath
			}
		}
	}
	return nil
}

// ParseVolumes validates the host and container paths passed in to the --volume flag
func ParseVolumes(volumes []string) error {
	if len(volumes) == 0 {
		return nil
	}
	for _, volume := range volumes {
		arr := strings.SplitN(volume, ":", 3)
		if len(arr) < 2 {
			return errors.Errorf("incorrect volume format %q, should be host-dir:ctr-dir[:option]", volume)
		}
		if err := validateVolumeHostDir(arr[0]); err != nil {
			return err
		}
		if err := validateVolumeCtrDir(arr[1]); err != nil {
			return err
		}
		if len(arr) > 2 {
			if err := validateVolumeOpts(arr[2]); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateVolumeHostDir(hostDir string) error {
	if !filepath.IsAbs(hostDir) {
		return errors.Errorf("invalid host path, must be an absolute path %q", hostDir)
	}
	if _, err := os.Stat(hostDir); err != nil {
		return errors.Wrapf(err, "error checking path %q", hostDir)
	}
	return nil
}

func validateVolumeCtrDir(ctrDir string) error {
	if !filepath.IsAbs(ctrDir) {
		return errors.Errorf("invalid container path, must be an absolute path %q", ctrDir)
	}
	return nil
}

func validateVolumeOpts(option string) error {
	var foundRootPropagation, foundRWRO, foundLabelChange int
	options := strings.Split(option, ",")
	for _, opt := range options {
		switch opt {
		case "rw", "ro":
			if foundRWRO > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 'rw' or 'ro' option", option)
			}
			foundRWRO++
		case "z", "Z":
			if foundLabelChange > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 'z' or 'Z' option", option)
			}
			foundLabelChange++
		case "private", "rprivate", "shared", "rshared", "slave", "rslave", "unbindable", "runbindable":
			if foundRootPropagation > 1 {
				return errors.Errorf("invalid options %q, can only specify 1 '[r]shared', '[r]private', '[r]slave' or '[r]unbindable' option", option)
			}
			foundRootPropagation++
		default:
			return errors.Errorf("invalid option type %q", option)
		}
	}
	return nil
}

// validateExtraHost validates that the specified string is a valid extrahost and returns it.
// ExtraHost is in the form of name:ip where the ip has to be a valid ip (ipv4 or ipv6).
// for add-host flag
func validateExtraHost(val string) error {
	// allow for IPv6 addresses in extra hosts by only splitting on first ":"
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || len(arr[0]) == 0 {
		return fmt.Errorf("bad format for add-host: %q", val)
	}
	if _, err := validateIPAddress(arr[1]); err != nil {
		return fmt.Errorf("invalid IP address in add-host: %q", arr[1])
	}
	return nil
}

// validateIPAddress validates an Ip address.
// for dns, ip, and ip6 flags also
func validateIPAddress(val string) (string, error) {
	var ip = net.ParseIP(strings.TrimSpace(val))
	if ip != nil {
		return ip.String(), nil
	}
	return "", fmt.Errorf("%s is not an ip address", val)
}

// ValidateFlags searches for StringFlags or StringSlice flags that never had
// a value set.  This commonly occurs when the CLI mistakenly takes the next
// option and uses it as a value.
func ValidateFlags(c *cli.Context, flags []cli.Flag) error {
	re, err := regexp.Compile("^-.+")
	if err != nil {
		return errors.Wrap(err, "compiling regex failed")
	}

	// The --cmd flag can have a following command i.e. --cmd="--help".
	// Let's skip this check just for the --cmd flag.
	for _, flag := range flags {
		switch reflect.TypeOf(flag).String() {
		case "cli.StringSliceFlag":
			{
				f := flag.(cli.StringSliceFlag)
				name := strings.Split(f.Name, ",")
				if f.Name == "cmd" {
					continue
				}
				val := c.StringSlice(name[0])
				for _, v := range val {
					if ok := re.MatchString(v); ok {
						return errors.Errorf("option --%s requires a value", name[0])
					}
				}
			}
		case "cli.StringFlag":
			{
				f := flag.(cli.StringFlag)
				name := strings.Split(f.Name, ",")
				if f.Name == "cmd" {
					continue
				}
				val := c.String(name[0])
				if ok := re.MatchString(val); ok {
					return errors.Errorf("option --%s requires a value", name[0])
				}
			}
		}
	}
	return nil
}

// SystemContextFromOptions returns a SystemContext populated with values
// per the input parameters provided by the caller for the use in authentication.
func SystemContextFromOptions(c *cli.Context) (*types.SystemContext, error) {
	ctx := &types.SystemContext{
		DockerCertPath: c.String("cert-dir"),
	}
	if c.IsSet("tls-verify") {
		ctx.DockerInsecureSkipTLSVerify = !c.BoolT("tls-verify")
	}
	if c.IsSet("creds") {
		var err error
		ctx.DockerAuthConfig, err = getDockerAuth(c.String("creds"))
		if err != nil {
			return nil, err
		}
	}
	if c.IsSet("signature-policy") {
		ctx.SignaturePolicyPath = c.String("signature-policy")
	}
	if c.IsSet("authfile") {
		ctx.AuthFilePath = c.String("authfile")
	}
	if c.GlobalIsSet("registries-conf") {
		ctx.SystemRegistriesConfPath = c.GlobalString("registries-conf")
	}
	if c.GlobalIsSet("registries-conf-dir") {
		ctx.RegistriesDirPath = c.GlobalString("registries-conf-dir")
	}
	return ctx, nil
}

func parseCreds(creds string) (string, string) {
	if creds == "" {
		return "", ""
	}
	up := strings.SplitN(creds, ":", 2)
	if len(up) == 1 {
		return up[0], ""
	}
	if up[0] == "" {
		return "", up[1]
	}
	return up[0], up[1]
}

func getDockerAuth(creds string) (*types.DockerAuthConfig, error) {
	username, password := parseCreds(creds)
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scanln(&username)
	}
	if password == "" {
		fmt.Print("Password: ")
		termPassword, err := terminal.ReadPassword(0)
		if err != nil {
			return nil, errors.Wrapf(err, "could not read password from terminal")
		}
		password = string(termPassword)
	}

	return &types.DockerAuthConfig{
		Username: username,
		Password: password,
	}, nil
}

//  IDMappingOptions parses the build options from user namespace
func IDMappingOptions(c *cli.Context) (usernsOptions buildah.NamespaceOptions, idmapOptions *buildah.IDMappingOptions, err error) {
	user := c.String("userns-uid-map-user")
	group := c.String("userns-gid-map-group")
	// If only the user or group was specified, use the same value for the
	// other, since we need both in order to initialize the maps using the
	// names.
	if user == "" && group != "" {
		user = group
	}
	if group == "" && user != "" {
		group = user
	}
	// Either start with empty maps or the name-based maps.
	mappings := idtools.NewIDMappingsFromMaps(nil, nil)
	if user != "" && group != "" {
		submappings, err := idtools.NewIDMappings(user, group)
		if err != nil {
			return nil, nil, err
		}
		mappings = submappings
	}
	// We'll parse the UID and GID mapping options the same way.
	buildIDMap := func(basemap []idtools.IDMap, option string) ([]specs.LinuxIDMapping, error) {
		outmap := make([]specs.LinuxIDMapping, 0, len(basemap))
		// Start with the name-based map entries.
		for _, m := range basemap {
			outmap = append(outmap, specs.LinuxIDMapping{
				ContainerID: uint32(m.ContainerID),
				HostID:      uint32(m.HostID),
				Size:        uint32(m.Size),
			})
		}
		// Parse the flag's value as one or more triples (if it's even
		// been set), and append them.
		idmap, err := parseIDMap(c.StringSlice(option))
		if err != nil {
			return nil, err
		}
		for _, m := range idmap {
			outmap = append(outmap, specs.LinuxIDMapping{
				ContainerID: m[0],
				HostID:      m[1],
				Size:        m[2],
			})
		}
		return outmap, nil
	}
	uidmap, err := buildIDMap(mappings.UIDs(), "userns-uid-map")
	if err != nil {
		return nil, nil, err
	}
	gidmap, err := buildIDMap(mappings.GIDs(), "userns-gid-map")
	if err != nil {
		return nil, nil, err
	}
	// If we only have one map or the other populated at this point, then
	// use the same mapping for both, since we know that no user or group
	// name was specified, but a specific mapping was for one or the other.
	if len(uidmap) == 0 && len(gidmap) != 0 {
		uidmap = gidmap
	}
	if len(gidmap) == 0 && len(uidmap) != 0 {
		gidmap = uidmap
	}
	// By default, having mappings configured means we use a user
	// namespace.  Otherwise, we don't.
	usernsOption := buildah.NamespaceOption{
		Name: string(specs.UserNamespace),
		Host: len(uidmap) == 0 && len(gidmap) == 0,
	}
	// If the user specifically requested that we either use or don't use
	// user namespaces, override that default.
	if c.IsSet("userns") {
		how := c.String("userns")
		switch how {
		case "", "container":
			usernsOption.Host = false
		case "host":
			usernsOption.Host = true
		default:
			if _, err := os.Stat(how); err != nil {
				return nil, nil, errors.Wrapf(err, "error checking for %s namespace at %q", string(specs.UserNamespace), how)
			}
			logrus.Debugf("setting %q namespace to %q", string(specs.UserNamespace), how)
			usernsOption.Path = how
		}
	}
	usernsOptions = buildah.NamespaceOptions{usernsOption}
	if !c.IsSet("net") {
		usernsOptions = append(usernsOptions, buildah.NamespaceOption{
			Name: string(specs.NetworkNamespace),
			Host: usernsOption.Host,
		})
	}
	// If the user requested that we use the host namespace, but also that
	// we use mappings, that's not going to work.
	if (len(uidmap) != 0 || len(gidmap) != 0) && usernsOption.Host {
		return nil, nil, errors.Errorf("can not specify ID mappings while using host's user namespace")
	}
	return usernsOptions, &buildah.IDMappingOptions{
		HostUIDMapping: usernsOption.Host,
		HostGIDMapping: usernsOption.Host,
		UIDMap:         uidmap,
		GIDMap:         gidmap,
	}, nil
}

func parseIDMap(spec []string) (m [][3]uint32, err error) {
	for _, s := range spec {
		args := strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsDigit(r) })
		if len(args)%3 != 0 {
			return nil, fmt.Errorf("mapping %q is not in the form containerid:hostid:size[,...]", s)
		}
		for len(args) >= 3 {
			cid, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("error parsing container ID %q from mapping %q as a number: %v", args[0], s, err)
			}
			hostid, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("error parsing host ID %q from mapping %q as a number: %v", args[1], s, err)
			}
			size, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("error parsing %q from mapping %q as a number: %v", args[2], s, err)
			}
			m = append(m, [3]uint32{uint32(cid), uint32(hostid), uint32(size)})
			args = args[3:]
		}
	}
	return m, nil
}

//  NamesapceOptions parses the build options from all namespaces except user namespace
func NamespaceOptions(c *cli.Context) (namespaceOptions buildah.NamespaceOptions, networkPolicy buildah.NetworkConfigurationPolicy, err error) {
	options := make(buildah.NamespaceOptions, 0, 7)
	policy := buildah.NetworkDefault
	for _, what := range []string{string(specs.IPCNamespace), "net", string(specs.PIDNamespace), string(specs.UTSNamespace)} {
		if c.IsSet(what) {
			how := c.String(what)
			switch what {
			case "net", "network":
				what = string(specs.NetworkNamespace)
			}
			switch how {
			case "", "container":
				logrus.Debugf("setting %q namespace to %q", what, "")
				options.AddOrReplace(buildah.NamespaceOption{
					Name: what,
				})
			case "host":
				logrus.Debugf("setting %q namespace to host", what)
				options.AddOrReplace(buildah.NamespaceOption{
					Name: what,
					Host: true,
				})
			default:
				if what == specs.NetworkNamespace {
					if how == "none" {
						options.AddOrReplace(buildah.NamespaceOption{
							Name: what,
						})
						policy = buildah.NetworkDisabled
						logrus.Debugf("setting network to disabled")
						break
					}
					if !filepath.IsAbs(how) {
						options.AddOrReplace(buildah.NamespaceOption{
							Name: what,
							Path: how,
						})
						policy = buildah.NetworkEnabled
						logrus.Debugf("setting network configuration to %q", how)
						break
					}
				}
				if _, err := os.Stat(how); err != nil {
					return nil, buildah.NetworkDefault, errors.Wrapf(err, "error checking for %s namespace at %q", what, how)
				}
				logrus.Debugf("setting %q namespace to %q", what, how)
				options.AddOrReplace(buildah.NamespaceOption{
					Name: what,
					Path: how,
				})
			}
		}
	}
	return options, policy, nil
}
