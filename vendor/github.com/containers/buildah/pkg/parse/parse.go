package parse

// this package should contain functions that parse and validate
// user input and is shared either amongst buildah subcommands or
// would be useful to projects vendoring buildah

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/containerd/containerd/platforms"
	"github.com/containers/buildah/define"
	internalParse "github.com/containers/buildah/internal/parse"
	"github.com/containers/buildah/pkg/sshagent"
	"github.com/containers/common/pkg/parse"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/unshare"
	units "github.com/docker/go-units"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

const (
	// SeccompDefaultPath defines the default seccomp path
	SeccompDefaultPath = "/usr/share/containers/seccomp.json"
	// SeccompOverridePath if this exists it overrides the default seccomp path
	SeccompOverridePath = "/etc/crio/seccomp.json"
	// TypeBind is the type for mounting host dir
	TypeBind = "bind"
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs = "tmpfs"
	// TypeCache is the type for mounting a common persistent cache from host
	TypeCache = "cache"
	// mount=type=cache must create a persistent directory on host so its available for all consecutive builds.
	// Lifecycle of following directory will be inherited from how host machine treats temporary directory
	BuildahCacheDir = "buildah-cache"
)

// CommonBuildOptions parses the build options from the bud cli
func CommonBuildOptions(c *cobra.Command) (*define.CommonBuildOptions, error) {
	return CommonBuildOptionsFromFlagSet(c.Flags(), c.Flag)
}

// CommonBuildOptionsFromFlagSet parses the build options from the bud cli
func CommonBuildOptionsFromFlagSet(flags *pflag.FlagSet, findFlagFunc func(name string) *pflag.Flag) (*define.CommonBuildOptions, error) {
	var (
		memoryLimit int64
		memorySwap  int64
		noDNS       bool
		err         error
	)

	memVal, _ := flags.GetString("memory")
	if memVal != "" {
		memoryLimit, err = units.RAMInBytes(memVal)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory")
		}
	}

	memSwapValue, _ := flags.GetString("memory-swap")
	if memSwapValue != "" {
		if memSwapValue == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(memSwapValue)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid value for memory-swap")
			}
		}
	}

	noHosts, _ := flags.GetBool("no-hosts")

	addHost, _ := flags.GetStringSlice("add-host")
	if len(addHost) > 0 {
		if noHosts {
			return nil, errors.Errorf("--no-hosts and --add-host conflict, can not be used together")
		}
		for _, host := range addHost {
			if err := validateExtraHost(host); err != nil {
				return nil, errors.Wrapf(err, "invalid value for add-host")
			}
		}
	}

	noDNS = false
	dnsServers := []string{}
	if flags.Changed("dns") {
		dnsServers, _ = flags.GetStringSlice("dns")
		for _, server := range dnsServers {
			if strings.ToLower(server) == "none" {
				noDNS = true
			}
		}
		if noDNS && len(dnsServers) > 1 {
			return nil, errors.Errorf("invalid --dns, --dns=none may not be used with any other --dns options")
		}
	}

	dnsSearch := []string{}
	if flags.Changed("dns-search") {
		dnsSearch, _ = flags.GetStringSlice("dns-search")
		if noDNS && len(dnsSearch) > 0 {
			return nil, errors.Errorf("invalid --dns-search, --dns-search may not be used with --dns=none")
		}
	}

	dnsOptions := []string{}
	if flags.Changed("dns-option") {
		dnsOptions, _ = flags.GetStringSlice("dns-option")
		if noDNS && len(dnsOptions) > 0 {
			return nil, errors.Errorf("invalid --dns-option, --dns-option may not be used with --dns=none")
		}
	}

	if _, err := units.FromHumanSize(findFlagFunc("shm-size").Value.String()); err != nil {
		return nil, errors.Wrapf(err, "invalid --shm-size")
	}
	volumes, _ := flags.GetStringArray("volume")
	if err := Volumes(volumes); err != nil {
		return nil, err
	}
	cpuPeriod, _ := flags.GetUint64("cpu-period")
	cpuQuota, _ := flags.GetInt64("cpu-quota")
	cpuShares, _ := flags.GetUint64("cpu-shares")
	httpProxy, _ := flags.GetBool("http-proxy")

	ulimit := []string{}
	if flags.Changed("ulimit") {
		ulimit, _ = flags.GetStringSlice("ulimit")
	}

	secrets, _ := flags.GetStringArray("secret")
	sshsources, _ := flags.GetStringArray("ssh")

	commonOpts := &define.CommonBuildOptions{
		AddHost:      addHost,
		CPUPeriod:    cpuPeriod,
		CPUQuota:     cpuQuota,
		CPUSetCPUs:   findFlagFunc("cpuset-cpus").Value.String(),
		CPUSetMems:   findFlagFunc("cpuset-mems").Value.String(),
		CPUShares:    cpuShares,
		CgroupParent: findFlagFunc("cgroup-parent").Value.String(),
		DNSOptions:   dnsOptions,
		DNSSearch:    dnsSearch,
		DNSServers:   dnsServers,
		HTTPProxy:    httpProxy,
		Memory:       memoryLimit,
		MemorySwap:   memorySwap,
		NoHosts:      noHosts,
		ShmSize:      findFlagFunc("shm-size").Value.String(),
		Ulimit:       ulimit,
		Volumes:      volumes,
		Secrets:      secrets,
		SSHSources:   sshsources,
	}
	securityOpts, _ := flags.GetStringArray("security-opt")
	if err := parseSecurityOpts(securityOpts, commonOpts); err != nil {
		return nil, err
	}
	return commonOpts, nil
}

func parseSecurityOpts(securityOpts []string, commonOpts *define.CommonBuildOptions) error {
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
				return errors.WithStack(err)
			}
			if _, err := os.Stat(SeccompDefaultPath); err != nil {
				if !os.IsNotExist(err) {
					return errors.WithStack(err)
				}
			} else {
				commonOpts.SeccompProfilePath = SeccompDefaultPath
			}
		}
	}
	return nil
}

// Split string into slice by colon. Backslash-escaped colon (i.e. "\:") will not be regarded as separator
func SplitStringWithColonEscape(str string) []string {
	return internalParse.SplitStringWithColonEscape(str)
}

// Volume parses the input of --volume
func Volume(volume string) (specs.Mount, error) {
	return internalParse.Volume(volume)
}

// Volumes validates the host and container paths passed in to the --volume flag
func Volumes(volumes []string) error {
	if len(volumes) == 0 {
		return nil
	}
	for _, volume := range volumes {
		if _, err := Volume(volume); err != nil {
			return err
		}
	}
	return nil
}

// ValidateVolumeHostDir validates a volume mount's source directory
func ValidateVolumeHostDir(hostDir string) error {
	return parse.ValidateVolumeHostDir(hostDir)
}

// ValidateVolumeCtrDir validates a volume mount's destination directory.
func ValidateVolumeCtrDir(ctrDir string) error {
	return parse.ValidateVolumeCtrDir(ctrDir)
}

// ValidateVolumeOpts validates a volume's options
func ValidateVolumeOpts(options []string) ([]string, error) {
	return parse.ValidateVolumeOpts(options)
}

// validateExtraHost validates that the specified string is a valid extrahost and returns it.
// ExtraHost is in the form of name:ip where the ip has to be a valid ip (ipv4 or ipv6).
// for add-host flag
func validateExtraHost(val string) error {
	// allow for IPv6 addresses in extra hosts by only splitting on first ":"
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || len(arr[0]) == 0 {
		return errors.Errorf("bad format for add-host: %q", val)
	}
	if _, err := validateIPAddress(arr[1]); err != nil {
		return errors.Errorf("invalid IP address in add-host: %q", arr[1])
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
	return "", errors.Errorf("%s is not an ip address", val)
}

// SystemContextFromOptions returns a SystemContext populated with values
// per the input parameters provided by the caller for the use in authentication.
func SystemContextFromOptions(c *cobra.Command) (*types.SystemContext, error) {
	return SystemContextFromFlagSet(c.Flags(), c.Flag)
}

// SystemContextFromFlagSet returns a SystemContext populated with values
// per the input parameters provided by the caller for the use in authentication.
func SystemContextFromFlagSet(flags *pflag.FlagSet, findFlagFunc func(name string) *pflag.Flag) (*types.SystemContext, error) {
	certDir, err := flags.GetString("cert-dir")
	if err != nil {
		certDir = ""
	}
	ctx := &types.SystemContext{
		DockerCertPath: certDir,
	}
	tlsVerify, err := flags.GetBool("tls-verify")
	if err == nil && findFlagFunc("tls-verify").Changed {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!tlsVerify)
		ctx.OCIInsecureSkipTLSVerify = !tlsVerify
		ctx.DockerDaemonInsecureSkipTLSVerify = !tlsVerify
	}
	disableCompression, err := flags.GetBool("disable-compression")
	if err == nil {
		if disableCompression {
			ctx.OCIAcceptUncompressedLayers = true
		} else {
			ctx.DirForceCompress = true
		}
	}
	creds, err := flags.GetString("creds")
	if err == nil && findFlagFunc("creds").Changed {
		var err error
		ctx.DockerAuthConfig, err = AuthConfig(creds)
		if err != nil {
			return nil, err
		}
	}
	sigPolicy, err := flags.GetString("signature-policy")
	if err == nil && findFlagFunc("signature-policy").Changed {
		ctx.SignaturePolicyPath = sigPolicy
	}
	authfile, err := flags.GetString("authfile")
	if err == nil {
		ctx.AuthFilePath = getAuthFile(authfile)
	}
	regConf, err := flags.GetString("registries-conf")
	if err == nil && findFlagFunc("registries-conf").Changed {
		ctx.SystemRegistriesConfPath = regConf
	}
	regConfDir, err := flags.GetString("registries-conf-dir")
	if err == nil && findFlagFunc("registries-conf-dir").Changed {
		ctx.RegistriesDirPath = regConfDir
	}
	shortNameAliasConf, err := flags.GetString("short-name-alias-conf")
	if err == nil && findFlagFunc("short-name-alias-conf").Changed {
		ctx.UserShortNameAliasConfPath = shortNameAliasConf
	}
	ctx.DockerRegistryUserAgent = fmt.Sprintf("Buildah/%s", define.Version)
	if findFlagFunc("os") != nil && findFlagFunc("os").Changed {
		var os string
		if os, err = flags.GetString("os"); err != nil {
			return nil, err
		}
		ctx.OSChoice = os
	}
	if findFlagFunc("arch") != nil && findFlagFunc("arch").Changed {
		var arch string
		if arch, err = flags.GetString("arch"); err != nil {
			return nil, err
		}
		ctx.ArchitectureChoice = arch
	}
	if findFlagFunc("variant") != nil && findFlagFunc("variant").Changed {
		var variant string
		if variant, err = flags.GetString("variant"); err != nil {
			return nil, err
		}
		ctx.VariantChoice = variant
	}
	if findFlagFunc("platform") != nil && findFlagFunc("platform").Changed {
		var specs []string
		if specs, err = flags.GetStringSlice("platform"); err != nil {
			return nil, err
		}
		if len(specs) == 0 || specs[0] == "" {
			return nil, errors.Errorf("unable to parse --platform value %v", specs)
		}
		platform := specs[0]
		os, arch, variant, err := Platform(platform)
		if err != nil {
			return nil, err
		}
		if ctx.OSChoice != "" || ctx.ArchitectureChoice != "" || ctx.VariantChoice != "" {
			return nil, errors.Errorf("invalid --platform may not be used with --os, --arch, or --variant")
		}
		ctx.OSChoice = os
		ctx.ArchitectureChoice = arch
		ctx.VariantChoice = variant
	}

	ctx.BigFilesTemporaryDir = GetTempDir()
	return ctx, nil
}

func getAuthFile(authfile string) string {
	if authfile != "" {
		return authfile
	}
	return os.Getenv("REGISTRY_AUTH_FILE")
}

// PlatformFromOptions parses the operating system (os) and architecture (arch)
// from the provided command line options.  Deprecated in favor of
// PlatformsFromOptions(), but kept here because it's part of our API.
func PlatformFromOptions(c *cobra.Command) (os, arch string, err error) {
	platforms, err := PlatformsFromOptions(c)
	if err != nil {
		return "", "", err
	}
	if len(platforms) < 1 {
		return "", "", errors.Errorf("invalid platform syntax for --platform (use OS/ARCH[/VARIANT])")
	}
	return platforms[0].OS, platforms[0].Arch, nil
}

// PlatformsFromOptions parses the operating system (os) and architecture
// (arch) from the provided command line options.  If --platform used, it
// also returns the list of platforms that were passed in as its argument.
func PlatformsFromOptions(c *cobra.Command) (platforms []struct{ OS, Arch, Variant string }, err error) {
	var os, arch, variant string
	if c.Flag("os").Changed {
		if os, err = c.Flags().GetString("os"); err != nil {
			return nil, err
		}
	}
	if c.Flag("arch").Changed {
		if arch, err = c.Flags().GetString("arch"); err != nil {
			return nil, err
		}
	}
	if c.Flag("variant").Changed {
		if variant, err = c.Flags().GetString("variant"); err != nil {
			return nil, err
		}
	}
	platforms = []struct{ OS, Arch, Variant string }{{os, arch, variant}}
	if c.Flag("platform").Changed {
		platforms = nil
		platformSpecs, err := c.Flags().GetStringSlice("platform")
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse platform")
		}
		if os != "" || arch != "" || variant != "" {
			return nil, errors.Errorf("invalid --platform may not be used with --os, --arch, or --variant")
		}
		for _, pf := range platformSpecs {
			if os, arch, variant, err = Platform(pf); err != nil {
				return nil, errors.Wrapf(err, "unable to parse platform %q", pf)
			}
			platforms = append(platforms, struct{ OS, Arch, Variant string }{os, arch, variant})
		}
	}
	return platforms, nil
}

const platformSep = "/"

// DefaultPlatform returns the standard platform for the current system
func DefaultPlatform() string {
	return platforms.DefaultString()
}

// Platform separates the platform string into os, arch and variant,
// accepting any of $arch, $os/$arch, or $os/$arch/$variant.
func Platform(platform string) (os, arch, variant string, err error) {
	split := strings.Split(platform, platformSep)
	switch len(split) {
	case 3:
		variant = split[2]
		fallthrough
	case 2:
		arch = split[1]
		os = split[0]
		return
	case 1:
		if platform == "local" {
			return Platform(DefaultPlatform())
		}
	}
	return "", "", "", errors.Errorf("invalid platform syntax for %q (use OS/ARCH[/VARIANT][,...])", platform)
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

// AuthConfig parses the creds in format [username[:password] into an auth
// config.
func AuthConfig(creds string) (*types.DockerAuthConfig, error) {
	username, password := parseCreds(creds)
	if username == "" {
		fmt.Print("Username: ")
		fmt.Scanln(&username)
	}
	if password == "" {
		fmt.Print("Password: ")
		termPassword, err := term.ReadPassword(0)
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

// IDMappingOptions parses the build options related to user namespaces and ID mapping.
func IDMappingOptions(c *cobra.Command, isolation define.Isolation) (usernsOptions define.NamespaceOptions, idmapOptions *define.IDMappingOptions, err error) {
	return IDMappingOptionsFromFlagSet(c.Flags(), c.PersistentFlags(), c.Flag)
}

// IDMappingOptionsFromFlagSet parses the build options related to user namespaces and ID mapping.
func IDMappingOptionsFromFlagSet(flags *pflag.FlagSet, persistentFlags *pflag.FlagSet, findFlagFunc func(name string) *pflag.Flag) (usernsOptions define.NamespaceOptions, idmapOptions *define.IDMappingOptions, err error) {
	user := findFlagFunc("userns-uid-map-user").Value.String()
	group := findFlagFunc("userns-gid-map-group").Value.String()
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
	globalOptions := persistentFlags
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
		var spec []string
		if globalOptions.Lookup(option) != nil && globalOptions.Lookup(option).Changed {
			spec, _ = globalOptions.GetStringSlice(option)
		}
		if findFlagFunc(option).Changed {
			spec, _ = flags.GetStringSlice(option)
		}
		idmap, err := parseIDMap(spec)
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
	usernsOption := define.NamespaceOption{
		Name: string(specs.UserNamespace),
		Host: len(uidmap) == 0 && len(gidmap) == 0,
	}
	// If the user specifically requested that we either use or don't use
	// user namespaces, override that default.
	if findFlagFunc("userns").Changed {
		how := findFlagFunc("userns").Value.String()
		switch how {
		case "", "container", "private":
			usernsOption.Host = false
		case "host":
			usernsOption.Host = true
		default:
			how = strings.TrimPrefix(how, "ns:")
			if _, err := os.Stat(how); err != nil {
				return nil, nil, errors.Wrapf(err, "checking %s namespace", string(specs.UserNamespace))
			}
			logrus.Debugf("setting %q namespace to %q", string(specs.UserNamespace), how)
			usernsOption.Path = how
		}
	}
	usernsOptions = define.NamespaceOptions{usernsOption}

	// If the user requested that we use the host namespace, but also that
	// we use mappings, that's not going to work.
	if (len(uidmap) != 0 || len(gidmap) != 0) && usernsOption.Host {
		return nil, nil, errors.Errorf("can not specify ID mappings while using host's user namespace")
	}
	return usernsOptions, &define.IDMappingOptions{
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
			return nil, errors.Errorf("mapping %q is not in the form containerid:hostid:size[,...]", s)
		}
		for len(args) >= 3 {
			cid, err := strconv.ParseUint(args[0], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing container ID %q from mapping %q as a number", args[0], s)
			}
			hostid, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing host ID %q from mapping %q as a number", args[1], s)
			}
			size, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return nil, errors.Wrapf(err, "error parsing %q from mapping %q as a number", args[2], s)
			}
			m = append(m, [3]uint32{uint32(cid), uint32(hostid), uint32(size)})
			args = args[3:]
		}
	}
	return m, nil
}

// NamespaceOptions parses the build options for all namespaces except for user namespace.
func NamespaceOptions(c *cobra.Command) (namespaceOptions define.NamespaceOptions, networkPolicy define.NetworkConfigurationPolicy, err error) {
	return NamespaceOptionsFromFlagSet(c.Flags(), c.Flag)
}

// NamespaceOptionsFromFlagSet parses the build options for all namespaces except for user namespace.
func NamespaceOptionsFromFlagSet(flags *pflag.FlagSet, findFlagFunc func(name string) *pflag.Flag) (namespaceOptions define.NamespaceOptions, networkPolicy define.NetworkConfigurationPolicy, err error) {
	options := make(define.NamespaceOptions, 0, 7)
	policy := define.NetworkDefault
	for _, what := range []string{"cgroupns", string(specs.IPCNamespace), "network", string(specs.PIDNamespace), string(specs.UTSNamespace)} {
		if flags.Lookup(what) != nil && findFlagFunc(what).Changed {
			how := findFlagFunc(what).Value.String()
			switch what {
			case "cgroupns":
				what = string(specs.CgroupNamespace)
			}
			switch how {
			case "", "container", "private":
				logrus.Debugf("setting %q namespace to %q", what, "")
				policy = define.NetworkEnabled
				options.AddOrReplace(define.NamespaceOption{
					Name: what,
				})
			case "host":
				logrus.Debugf("setting %q namespace to host", what)
				policy = define.NetworkEnabled
				options.AddOrReplace(define.NamespaceOption{
					Name: what,
					Host: true,
				})
			default:
				if what == string(specs.NetworkNamespace) {
					if how == "none" {
						options.AddOrReplace(define.NamespaceOption{
							Name: what,
						})
						policy = define.NetworkDisabled
						logrus.Debugf("setting network to disabled")
						break
					}
				}
				how = strings.TrimPrefix(how, "ns:")
				// if not a path we assume it is a comma separated network list, see setupNamespaces() in run_linux.go
				if filepath.IsAbs(how) || what != string(specs.NetworkNamespace) {
					if _, err := os.Stat(how); err != nil {
						return nil, define.NetworkDefault, errors.Wrapf(err, "checking %s namespace", what)
					}
				}
				policy = define.NetworkEnabled
				logrus.Debugf("setting %q namespace to %q", what, how)
				options.AddOrReplace(define.NamespaceOption{
					Name: what,
					Path: how,
				})
			}
		}
	}
	return options, policy, nil
}

func defaultIsolation() (define.Isolation, error) {
	isolation, isSet := os.LookupEnv("BUILDAH_ISOLATION")
	if isSet {
		switch strings.ToLower(isolation) {
		case "oci":
			return define.IsolationOCI, nil
		case "rootless":
			return define.IsolationOCIRootless, nil
		case "chroot":
			return define.IsolationChroot, nil
		default:
			return 0, errors.Errorf("unrecognized $BUILDAH_ISOLATION value %q", isolation)
		}
	}
	if unshare.IsRootless() {
		return define.IsolationOCIRootless, nil
	}
	return define.IsolationDefault, nil
}

// IsolationOption parses the --isolation flag.
func IsolationOption(isolation string) (define.Isolation, error) {
	if isolation != "" {
		switch strings.ToLower(isolation) {
		case "oci", "default":
			return define.IsolationOCI, nil
		case "rootless":
			return define.IsolationOCIRootless, nil
		case "chroot":
			return define.IsolationChroot, nil
		default:
			return 0, errors.Errorf("unrecognized isolation type %q", isolation)
		}
	}
	return defaultIsolation()
}

// Device parses device mapping string to a src, dest & permissions string
// Valid values for device look like:
//    '/dev/sdc"
//    '/dev/sdc:/dev/xvdc"
//    '/dev/sdc:/dev/xvdc:rwm"
//    '/dev/sdc:rm"
func Device(device string) (string, string, string, error) {
	src := ""
	dst := ""
	permissions := "rwm"
	arr := strings.Split(device, ":")
	switch len(arr) {
	case 3:
		if !isValidDeviceMode(arr[2]) {
			return "", "", "", errors.Errorf("invalid device mode: %s", arr[2])
		}
		permissions = arr[2]
		fallthrough
	case 2:
		if isValidDeviceMode(arr[1]) {
			permissions = arr[1]
		} else {
			if len(arr[1]) == 0 || arr[1][0] != '/' {
				return "", "", "", errors.Errorf("invalid device mode: %s", arr[1])
			}
			dst = arr[1]
		}
		fallthrough
	case 1:
		if len(arr[0]) > 0 {
			src = arr[0]
			break
		}
		fallthrough
	default:
		return "", "", "", errors.Errorf("invalid device specification: %s", device)
	}

	if dst == "" {
		dst = src
	}
	return src, dst, permissions, nil
}

// isValidDeviceMode checks if the mode for device is valid or not.
// isValid mode is a composition of r (read), w (write), and m (mknod).
func isValidDeviceMode(mode string) bool {
	var legalDeviceMode = map[rune]bool{
		'r': true,
		'w': true,
		'm': true,
	}
	if mode == "" {
		return false
	}
	for _, c := range mode {
		if !legalDeviceMode[c] {
			return false
		}
		legalDeviceMode[c] = false
	}
	return true
}

func GetTempDir() string {
	if tmpdir, ok := os.LookupEnv("TMPDIR"); ok {
		return tmpdir
	}
	return "/var/tmp"
}

// Secrets parses the --secret flag
func Secrets(secrets []string) (map[string]define.Secret, error) {
	invalidSyntax := errors.Errorf("incorrect secret flag format: should be --secret id=foo,src=bar[,env=ENV,type=file|env]")
	parsed := make(map[string]define.Secret)
	for _, secret := range secrets {
		tokens := strings.Split(secret, ",")
		var id, src, typ string
		for _, val := range tokens {
			kv := strings.SplitN(val, "=", 2)
			switch kv[0] {
			case "id":
				id = kv[1]
			case "src":
				src = kv[1]
			case "env":
				src = kv[1]
				typ = "env"
			case "type":
				if kv[1] != "file" && kv[1] != "env" {
					return nil, errors.New("invalid secret type, must be file or env")
				}
				typ = kv[1]
			}
		}
		if id == "" {
			return nil, invalidSyntax
		}
		if src == "" {
			src = id
		}
		if typ == "" {
			if _, ok := os.LookupEnv(id); ok {
				typ = "env"
			} else {
				typ = "file"
			}
		}

		if typ == "file" {
			fullPath, err := filepath.Abs(src)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse secrets")
			}
			_, err = os.Stat(fullPath)
			if err != nil {
				return nil, errors.Wrap(err, "could not parse secrets")
			}
			src = fullPath
		}
		newSecret := define.Secret{
			Source:     src,
			SourceType: typ,
		}
		parsed[id] = newSecret

	}
	return parsed, nil
}

// SSH parses the --ssh flag
func SSH(sshSources []string) (map[string]*sshagent.Source, error) {
	parsed := make(map[string]*sshagent.Source)
	var paths []string
	for _, v := range sshSources {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) > 1 {
			paths = strings.Split(parts[1], ",")
		}

		source, err := sshagent.NewSource(paths)
		if err != nil {
			return nil, err
		}
		parsed[parts[0]] = source
	}
	return parsed, nil
}

func ContainerIgnoreFile(contextDir, path string) ([]string, string, error) {
	if path != "" {
		excludes, err := imagebuilder.ParseIgnore(path)
		return excludes, path, err
	}
	path = filepath.Join(contextDir, ".containerignore")
	excludes, err := imagebuilder.ParseIgnore(path)
	if os.IsNotExist(err) {
		path = filepath.Join(contextDir, ".dockerignore")
		excludes, err = imagebuilder.ParseIgnore(path)
	}
	if os.IsNotExist(err) {
		return excludes, "", nil
	}
	return excludes, path, err
}
