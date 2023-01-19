package parse

// this package should contain functions that parse and validate
// user input and is shared either amongst buildah subcommands or
// would be useful to projects vendoring buildah

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/containerd/containerd/platforms"
	"github.com/containers/buildah/define"
	securejoin "github.com/cyphar/filepath-securejoin"
	internalParse "github.com/containers/buildah/internal/parse"
	"github.com/containers/buildah/pkg/sshagent"
	"github.com/containers/common/pkg/config"
	"github.com/containers/common/pkg/parse"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage/pkg/idtools"
	"github.com/containers/storage/pkg/unshare"
	storageTypes "github.com/containers/storage/types"
	units "github.com/docker/go-units"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/openshift/imagebuilder"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

const (
	// SeccompDefaultPath defines the default seccomp path
	SeccompDefaultPath = config.SeccompDefaultPath
	// SeccompOverridePath if this exists it overrides the default seccomp path
	SeccompOverridePath = config.SeccompOverridePath
	// TypeBind is the type for mounting host dir
	TypeBind = "bind"
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs = "tmpfs"
	// TypeCache is the type for mounting a common persistent cache from host
	TypeCache = "cache"
	// mount=type=cache must create a persistent directory on host so it's available for all consecutive builds.
	// Lifecycle of following directory will be inherited from how host machine treats temporary directory
	BuildahCacheDir = "buildah-cache"
)

// RepoNamesToNamedReferences parse the raw string to Named reference
func RepoNamesToNamedReferences(destList []string) ([]reference.Named, error) {
	var result []reference.Named
	for _, dest := range destList {
		named, err := reference.ParseNormalizedNamed(dest)
		if err != nil {
			return nil, fmt.Errorf("invalid repo %q: must contain registry and repository: %w", dest, err)
		}
		if !reference.IsNameOnly(named) {
			return nil, fmt.Errorf("repository must contain neither a tag nor digest: %v", named)
		}
		result = append(result, named)
	}
	return result, nil
}

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
			return nil, fmt.Errorf("invalid value for memory: %w", err)
		}
	}

	memSwapValue, _ := flags.GetString("memory-swap")
	if memSwapValue != "" {
		if memSwapValue == "-1" {
			memorySwap = -1
		} else {
			memorySwap, err = units.RAMInBytes(memSwapValue)
			if err != nil {
				return nil, fmt.Errorf("invalid value for memory-swap: %w", err)
			}
		}
	}

	noHosts, _ := flags.GetBool("no-hosts")

	addHost, _ := flags.GetStringSlice("add-host")
	if len(addHost) > 0 {
		if noHosts {
			return nil, errors.New("--no-hosts and --add-host conflict, can not be used together")
		}
		for _, host := range addHost {
			if err := validateExtraHost(host); err != nil {
				return nil, fmt.Errorf("invalid value for add-host: %w", err)
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
			return nil, errors.New("invalid --dns, --dns=none may not be used with any other --dns options")
		}
	}

	dnsSearch := []string{}
	if flags.Changed("dns-search") {
		dnsSearch, _ = flags.GetStringSlice("dns-search")
		if noDNS && len(dnsSearch) > 0 {
			return nil, errors.New("invalid --dns-search, --dns-search may not be used with --dns=none")
		}
	}

	dnsOptions := []string{}
	if flags.Changed("dns-option") {
		dnsOptions, _ = flags.GetStringSlice("dns-option")
		if noDNS && len(dnsOptions) > 0 {
			return nil, errors.New("invalid --dns-option, --dns-option may not be used with --dns=none")
		}
	}

	if _, err := units.FromHumanSize(findFlagFunc("shm-size").Value.String()); err != nil {
		return nil, fmt.Errorf("invalid --shm-size: %w", err)
	}
	volumes, _ := flags.GetStringArray("volume")
	if err := Volumes(volumes); err != nil {
		return nil, err
	}
	cpuPeriod, _ := flags.GetUint64("cpu-period")
	cpuQuota, _ := flags.GetInt64("cpu-quota")
	cpuShares, _ := flags.GetUint64("cpu-shares")
	httpProxy, _ := flags.GetBool("http-proxy")
	identityLabel, _ := flags.GetBool("identity-label")
	omitHistory, _ := flags.GetBool("omit-history")

	ulimit := []string{}
	if flags.Changed("ulimit") {
		ulimit, _ = flags.GetStringSlice("ulimit")
	}

	secrets, _ := flags.GetStringArray("secret")
	sshsources, _ := flags.GetStringArray("ssh")
	ociHooks, _ := flags.GetStringArray("hooks-dir")

	commonOpts := &define.CommonBuildOptions{
		AddHost:       addHost,
		CPUPeriod:     cpuPeriod,
		CPUQuota:      cpuQuota,
		CPUSetCPUs:    findFlagFunc("cpuset-cpus").Value.String(),
		CPUSetMems:    findFlagFunc("cpuset-mems").Value.String(),
		CPUShares:     cpuShares,
		CgroupParent:  findFlagFunc("cgroup-parent").Value.String(),
		DNSOptions:    dnsOptions,
		DNSSearch:     dnsSearch,
		DNSServers:    dnsServers,
		HTTPProxy:     httpProxy,
		IdentityLabel: types.NewOptionalBool(identityLabel),
		Memory:        memoryLimit,
		MemorySwap:    memorySwap,
		NoHosts:       noHosts,
		OmitHistory:   omitHistory,
		ShmSize:       findFlagFunc("shm-size").Value.String(),
		Ulimit:        ulimit,
		Volumes:       volumes,
		Secrets:       secrets,
		SSHSources:    sshsources,
		OCIHooksDir:   ociHooks,
	}
	securityOpts, _ := flags.GetStringArray("security-opt")
	if err := parseSecurityOpts(securityOpts, commonOpts); err != nil {
		return nil, err
	}
	return commonOpts, nil
}

// GetAdditionalBuildContext consumes raw string and returns parsed AdditionalBuildContext
func GetAdditionalBuildContext(value string) (define.AdditionalBuildContext, error) {
	ret := define.AdditionalBuildContext{IsURL: false, IsImage: false, Value: value}
	if strings.HasPrefix(value, "docker-image://") {
		ret.IsImage = true
		ret.Value = strings.TrimPrefix(value, "docker-image://")
	} else if strings.HasPrefix(value, "container-image://") {
		ret.IsImage = true
		ret.Value = strings.TrimPrefix(value, "container-image://")
	} else if strings.HasPrefix(value, "docker://") {
		ret.IsImage = true
		ret.Value = strings.TrimPrefix(value, "docker://")
	} else if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		ret.IsImage = false
		ret.IsURL = true
	} else {
		path, err := filepath.Abs(value)
		if err != nil {
			return define.AdditionalBuildContext{}, fmt.Errorf("unable to convert additional build-context %q path to absolute: %w", value, err)
		}
		ret.Value = path
	}
	return ret, nil
}

func parseSecurityOpts(securityOpts []string, commonOpts *define.CommonBuildOptions) error {
	for _, opt := range securityOpts {
		if opt == "no-new-privileges" {
			commonOpts.NoNewPrivileges = true
			continue
		}

		con := strings.SplitN(opt, "=", 2)
		if len(con) != 2 {
			return fmt.Errorf("invalid --security-opt name=value pair: %q", opt)
		}
		switch con[0] {
		case "label":
			commonOpts.LabelOpts = append(commonOpts.LabelOpts, con[1])
		case "apparmor":
			commonOpts.ApparmorProfile = con[1]
		case "seccomp":
			commonOpts.SeccompProfilePath = con[1]
		default:
			return fmt.Errorf("invalid --security-opt 2: %q", opt)
		}

	}

	if commonOpts.SeccompProfilePath == "" {
		if _, err := os.Stat(SeccompOverridePath); err == nil {
			commonOpts.SeccompProfilePath = SeccompOverridePath
		} else {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if _, err := os.Stat(SeccompDefaultPath); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return err
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
	insecure, err := flags.GetBool("insecure")
	if err == nil && findFlagFunc("insecure").Changed {
		if ctx.DockerInsecureSkipTLSVerify != types.OptionalBoolUndefined {
			return nil, errors.New("--insecure may not be used with --tls-verify")
		}
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(insecure)
		ctx.OCIInsecureSkipTLSVerify = insecure
		ctx.DockerDaemonInsecureSkipTLSVerify = insecure
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
			return nil, fmt.Errorf("unable to parse --platform value %v", specs)
		}
		platform := specs[0]
		os, arch, variant, err := Platform(platform)
		if err != nil {
			return nil, err
		}
		if ctx.OSChoice != "" || ctx.ArchitectureChoice != "" || ctx.VariantChoice != "" {
			return nil, errors.New("invalid --platform may not be used with --os, --arch, or --variant")
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
		return "", "", errors.New("invalid platform syntax for --platform (use OS/ARCH[/VARIANT])")
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
			return nil, fmt.Errorf("unable to parse platform: %w", err)
		}
		if os != "" || arch != "" || variant != "" {
			return nil, fmt.Errorf("invalid --platform may not be used with --os, --arch, or --variant")
		}
		for _, pf := range platformSpecs {
			if os, arch, variant, err = Platform(pf); err != nil {
				return nil, fmt.Errorf("unable to parse platform %q: %w", pf, err)
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
	return "", "", "", fmt.Errorf("invalid platform syntax for %q (use OS/ARCH[/VARIANT][,...])", platform)
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
			return nil, fmt.Errorf("could not read password from terminal: %w", err)
		}
		password = string(termPassword)
	}

	return &types.DockerAuthConfig{
		Username: username,
		Password: password,
	}, nil
}

// GetBuildOutput is responsible for parsing custom build output argument i.e `build --output` flag.
// Takes `buildOutput` as string and returns BuildOutputOption
func GetBuildOutput(buildOutput string) (define.BuildOutputOption, error) {
	if len(buildOutput) == 1 && buildOutput == "-" {
		// Feature parity with buildkit, output tar to stdout
		// Read more here: https://docs.docker.com/engine/reference/commandline/build/#custom-build-outputs
		return define.BuildOutputOption{Path: "",
			IsDir:    false,
			IsStdout: true}, nil
	}
	if !strings.Contains(buildOutput, ",") {
		// expect default --output <dirname>
		return define.BuildOutputOption{Path: buildOutput,
			IsDir:    true,
			IsStdout: false}, nil
	}
	isDir := true
	isStdout := false
	typeSelected := false
	pathSelected := false
	path := ""
	tokens := strings.Split(buildOutput, ",")
	for _, option := range tokens {
		arr := strings.SplitN(option, "=", 2)
		if len(arr) != 2 {
			return define.BuildOutputOption{}, fmt.Errorf("invalid build output options %q, expected format key=value", buildOutput)
		}
		switch arr[0] {
		case "type":
			if typeSelected {
				return define.BuildOutputOption{}, fmt.Errorf("duplicate %q not supported", arr[0])
			}
			typeSelected = true
			if arr[1] == "local" {
				isDir = true
			} else if arr[1] == "tar" {
				isDir = false
			} else {
				return define.BuildOutputOption{}, fmt.Errorf("invalid type %q selected for build output options %q", arr[1], buildOutput)
			}
		case "dest":
			if pathSelected {
				return define.BuildOutputOption{}, fmt.Errorf("duplicate %q not supported", arr[0])
			}
			pathSelected = true
			path = arr[1]
		default:
			return define.BuildOutputOption{}, fmt.Errorf("unrecognized key %q in build output option: %q", arr[0], buildOutput)
		}
	}

	if !typeSelected || !pathSelected {
		return define.BuildOutputOption{}, fmt.Errorf("invalid build output option %q, accepted keys are type and dest must be present", buildOutput)
	}

	if path == "-" {
		if isDir {
			return define.BuildOutputOption{}, fmt.Errorf("invalid build output option %q, type=local and dest=- is not supported", buildOutput)
		}
		return define.BuildOutputOption{Path: "",
			IsDir:    false,
			IsStdout: true}, nil
	}

	return define.BuildOutputOption{Path: path, IsDir: isDir, IsStdout: isStdout}, nil
}

// IDMappingOptions parses the build options related to user namespaces and ID mapping.
func IDMappingOptions(c *cobra.Command, isolation define.Isolation) (usernsOptions define.NamespaceOptions, idmapOptions *define.IDMappingOptions, err error) {
	return IDMappingOptionsFromFlagSet(c.Flags(), c.PersistentFlags(), c.Flag)
}

// GetAutoOptions returns a AutoUserNsOptions with the settings to setup automatically
// a user namespace.
func GetAutoOptions(base string) (*storageTypes.AutoUserNsOptions, error) {
	parts := strings.SplitN(base, ":", 2)
	if parts[0] != "auto" {
		return nil, errors.New("wrong user namespace mode")
	}
	options := storageTypes.AutoUserNsOptions{}
	if len(parts) == 1 {
		return &options, nil
	}
	for _, o := range strings.Split(parts[1], ",") {
		v := strings.SplitN(o, "=", 2)
		if len(v) != 2 {
			return nil, fmt.Errorf("invalid option specified: %q", o)
		}
		switch v[0] {
		case "size":
			s, err := strconv.ParseUint(v[1], 10, 32)
			if err != nil {
				return nil, err
			}
			options.Size = uint32(s)
		case "uidmapping":
			mapping, err := storageTypes.ParseIDMapping([]string{v[1]}, nil, "", "")
			if err != nil {
				return nil, err
			}
			options.AdditionalUIDMappings = append(options.AdditionalUIDMappings, mapping.UIDMap...)
		case "gidmapping":
			mapping, err := storageTypes.ParseIDMapping(nil, []string{v[1]}, "", "")
			if err != nil {
				return nil, err
			}
			options.AdditionalGIDMappings = append(options.AdditionalGIDMappings, mapping.GIDMap...)
		default:
			return nil, fmt.Errorf("unknown option specified: %q", v[0])
		}
	}
	return &options, nil
}

// IDMappingOptionsFromFlagSet parses the build options related to user namespaces and ID mapping.
func IDMappingOptionsFromFlagSet(flags *pflag.FlagSet, persistentFlags *pflag.FlagSet, findFlagFunc func(name string) *pflag.Flag) (usernsOptions define.NamespaceOptions, idmapOptions *define.IDMappingOptions, err error) {
	isAuto := false
	autoOpts := &storageTypes.AutoUserNsOptions{}
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
		if strings.HasPrefix(how, "auto") {
			autoOpts, err = GetAutoOptions(how)
			if err != nil {
				return nil, nil, err
			}
			isAuto = true
			usernsOption.Host = false
		} else {
			switch how {
			case "", "container", "private":
				usernsOption.Host = false
			case "host":
				usernsOption.Host = true
			default:
				how = strings.TrimPrefix(how, "ns:")
				if _, err := os.Stat(how); err != nil {
					return nil, nil, fmt.Errorf("checking %s namespace: %w", string(specs.UserNamespace), err)
				}
				logrus.Debugf("setting %q namespace to %q", string(specs.UserNamespace), how)
				usernsOption.Path = how
			}
		}
	}
	usernsOptions = define.NamespaceOptions{usernsOption}

	// If the user requested that we use the host namespace, but also that
	// we use mappings, that's not going to work.
	if (len(uidmap) != 0 || len(gidmap) != 0) && usernsOption.Host {
		return nil, nil, fmt.Errorf("can not specify ID mappings while using host's user namespace")
	}
	return usernsOptions, &define.IDMappingOptions{
		HostUIDMapping: usernsOption.Host,
		HostGIDMapping: usernsOption.Host,
		UIDMap:         uidmap,
		GIDMap:         gidmap,
		AutoUserNs:     isAuto,
		AutoUserNsOpts: *autoOpts,
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
				return nil, fmt.Errorf("parsing container ID %q from mapping %q as a number: %w", args[0], s, err)
			}
			hostid, err := strconv.ParseUint(args[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("parsing host ID %q from mapping %q as a number: %w", args[1], s, err)
			}
			size, err := strconv.ParseUint(args[2], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("parsing %q from mapping %q as a number: %w", args[2], s, err)
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
						return nil, define.NetworkDefault, fmt.Errorf("checking %s namespace: %w", what, err)
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
			return 0, fmt.Errorf("unrecognized $BUILDAH_ISOLATION value %q", isolation)
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
			return 0, fmt.Errorf("unrecognized isolation type %q", isolation)
		}
	}
	return defaultIsolation()
}

// Device parses device mapping string to a src, dest & permissions string
// Valid values for device look like:
//
//	'/dev/sdc"
//	'/dev/sdc:/dev/xvdc"
//	'/dev/sdc:/dev/xvdc:rwm"
//	'/dev/sdc:rm"
func Device(device string) (string, string, string, error) {
	src := ""
	dst := ""
	permissions := "rwm"
	arr := strings.Split(device, ":")
	switch len(arr) {
	case 3:
		if !isValidDeviceMode(arr[2]) {
			return "", "", "", fmt.Errorf("invalid device mode: %s", arr[2])
		}
		permissions = arr[2]
		fallthrough
	case 2:
		if isValidDeviceMode(arr[1]) {
			permissions = arr[1]
		} else {
			if len(arr[1]) == 0 || arr[1][0] != '/' {
				return "", "", "", fmt.Errorf("invalid device mode: %s", arr[1])
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
		return "", "", "", fmt.Errorf("invalid device specification: %s", device)
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
	invalidSyntax := fmt.Errorf("incorrect secret flag format: should be --secret id=foo,src=bar[,env=ENV,type=file|env]")
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
				return nil, fmt.Errorf("could not parse secrets: %w", err)
			}
			_, err = os.Stat(fullPath)
			if err != nil {
				return nil, fmt.Errorf("could not parse secrets: %w", err)
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

// ContainerIgnoreFile consumes path to `dockerignore` or `containerignore`
// and returns list of files to exclude along with the path to processed ignore
// file. Deprecated since this might become internal only, please avoid relying
// on this function.
func ContainerIgnoreFile(contextDir, path string, containerFiles []string) ([]string, string, error) {
	if path != "" {
		excludes, err := imagebuilder.ParseIgnore(path)
		return excludes, path, err
	}
	// If path was not supplied give priority to `<containerfile>.containerignore` first.
	for _, containerfile := range containerFiles {
		if !filepath.IsAbs(containerfile) {
			containerfile = filepath.Join(contextDir, containerfile)
		}
		containerfileIgnore := ""
		if _, err := os.Stat(containerfile + ".containerignore"); err == nil {
			containerfileIgnore = containerfile + ".containerignore"
		}
		if _, err := os.Stat(containerfile + ".dockerignore"); err == nil {
			containerfileIgnore = containerfile + ".dockerignore"
		}
		if containerfileIgnore != "" {
			excludes, err := imagebuilder.ParseIgnore(containerfileIgnore)
			return excludes, containerfileIgnore, err
		}
	}
	path, symlinkErr := securejoin.SecureJoin(contextDir, ".containerignore")
	if symlinkErr != nil {
		return nil, "", symlinkErr
	}
	excludes, err := imagebuilder.ParseIgnore(path)
	if errors.Is(err, os.ErrNotExist) {
		path, symlinkErr = securejoin.SecureJoin(contextDir, ".dockerignore")
		if symlinkErr != nil {
			return nil, "", symlinkErr
		}
		excludes, err = imagebuilder.ParseIgnore(path)
	}
	if errors.Is(err, os.ErrNotExist) {
		return excludes, "", nil
	}
	return excludes, path, err
}
