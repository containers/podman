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

	"github.com/containers/buildah"
	"github.com/containers/buildah/pkg/unshare"
	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/idtools"
	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
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
)

var (
	errBadMntOption  = errors.Errorf("invalid mount option")
	errDuplicateDest = errors.Errorf("duplicate mount destination")
	optionArgError   = errors.Errorf("must provide an argument for option")
	noDestError      = errors.Errorf("must set volume destination")
)

// CommonBuildOptions parses the build options from the bud cli
func CommonBuildOptions(c *cobra.Command) (*buildah.CommonBuildOptions, error) {
	var (
		memoryLimit int64
		memorySwap  int64
		noDNS       bool
		err         error
	)

	defaultLimits := getDefaultProcessLimits()

	memVal, _ := c.Flags().GetString("memory")
	if memVal != "" {
		memoryLimit, err = units.RAMInBytes(memVal)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory")
		}
	}

	memSwapValue, _ := c.Flags().GetString("memory-swap")
	if memSwapValue != "" {
		memorySwap, err = units.RAMInBytes(memSwapValue)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory-swap")
		}
	}

	addHost, _ := c.Flags().GetStringSlice("add-host")
	if len(addHost) > 0 {
		for _, host := range addHost {
			if err := validateExtraHost(host); err != nil {
				return nil, errors.Wrapf(err, "invalid value for add-host")
			}
		}
	}

	noDNS = false
	dnsServers, _ := c.Flags().GetStringSlice("dns")
	for _, server := range dnsServers {
		if strings.ToLower(server) == "none" {
			noDNS = true
		}
	}
	if noDNS && len(dnsServers) > 1 {
		return nil, errors.Errorf("invalid --dns, --dns=none may not be used with any other --dns options")
	}

	dnsSearch, _ := c.Flags().GetStringSlice("dns-search")
	if noDNS && len(dnsSearch) > 0 {
		return nil, errors.Errorf("invalid --dns-search, --dns-search may not be used with --dns=none")
	}

	dnsOptions, _ := c.Flags().GetStringSlice("dns-option")
	if noDNS && len(dnsOptions) > 0 {
		return nil, errors.Errorf("invalid --dns-option, --dns-option may not be used with --dns=none")
	}

	if _, err := units.FromHumanSize(c.Flag("shm-size").Value.String()); err != nil {
		return nil, errors.Wrapf(err, "invalid --shm-size")
	}
	volumes, _ := c.Flags().GetStringSlice("volume")
	if err := ParseVolumes(volumes); err != nil {
		return nil, err
	}
	cpuPeriod, _ := c.Flags().GetUint64("cpu-period")
	cpuQuota, _ := c.Flags().GetInt64("cpu-quota")
	cpuShares, _ := c.Flags().GetUint64("cpu-shares")
	httpProxy, _ := c.Flags().GetBool("http-proxy")
	ulimit, _ := c.Flags().GetStringSlice("ulimit")
	commonOpts := &buildah.CommonBuildOptions{
		AddHost:      addHost,
		CgroupParent: c.Flag("cgroup-parent").Value.String(),
		CPUPeriod:    cpuPeriod,
		CPUQuota:     cpuQuota,
		CPUSetCPUs:   c.Flag("cpuset-cpus").Value.String(),
		CPUSetMems:   c.Flag("cpuset-mems").Value.String(),
		CPUShares:    cpuShares,
		DNSSearch:    dnsSearch,
		DNSServers:   dnsServers,
		DNSOptions:   dnsOptions,
		HTTPProxy:    httpProxy,
		Memory:       memoryLimit,
		MemorySwap:   memorySwap,
		ShmSize:      c.Flag("shm-size").Value.String(),
		Ulimit:       append(defaultLimits, ulimit...),
		Volumes:      volumes,
	}
	securityOpts, _ := c.Flags().GetStringArray("security-opt")
	if err := parseSecurityOpts(securityOpts, commonOpts); err != nil {
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

// ParseVolume parses the input of --volume
func ParseVolume(volume string) (specs.Mount, error) {
	mount := specs.Mount{}
	arr := strings.SplitN(volume, ":", 3)
	if len(arr) < 2 {
		return mount, errors.Errorf("incorrect volume format %q, should be host-dir:ctr-dir[:option]", volume)
	}
	if err := validateVolumeMountHostDir(arr[0]); err != nil {
		return mount, err
	}
	if err := ValidateVolumeCtrDir(arr[1]); err != nil {
		return mount, err
	}
	mountOptions := ""
	if len(arr) > 2 {
		mountOptions = arr[2]
		if _, err := ValidateVolumeOpts(strings.Split(arr[2], ",")); err != nil {
			return mount, err
		}
	}
	mountOpts := strings.Split(mountOptions, ",")
	mount.Source = arr[0]
	mount.Destination = arr[1]
	mount.Type = "rbind"
	mount.Options = mountOpts
	return mount, nil
}

// ParseVolumes validates the host and container paths passed in to the --volume flag
func ParseVolumes(volumes []string) error {
	if len(volumes) == 0 {
		return nil
	}
	for _, volume := range volumes {
		if _, err := ParseVolume(volume); err != nil {
			return err
		}
	}
	return nil
}

func getVolumeMounts(volumes []string) (map[string]specs.Mount, error) {
	finalVolumeMounts := make(map[string]specs.Mount)

	for _, volume := range volumes {
		volumeMount, err := ParseVolume(volume)
		if err != nil {
			return nil, err
		}
		if _, ok := finalVolumeMounts[volumeMount.Destination]; ok {
			return nil, errors.Wrapf(errDuplicateDest, volumeMount.Destination)
		}
		finalVolumeMounts[volumeMount.Destination] = volumeMount
	}
	return finalVolumeMounts, nil
}

// GetVolumes gets the volumes from --volume and --mount
func GetVolumes(volumes []string, mounts []string) ([]specs.Mount, error) {
	unifiedMounts, err := getMounts(mounts)
	if err != nil {
		return nil, err
	}
	volumeMounts, err := getVolumeMounts(volumes)
	if err != nil {
		return nil, err
	}
	for dest, mount := range volumeMounts {
		if _, ok := unifiedMounts[dest]; ok {
			return nil, errors.Wrapf(errDuplicateDest, dest)
		}
		unifiedMounts[dest] = mount
	}

	finalMounts := make([]specs.Mount, 0, len(unifiedMounts))
	for _, mount := range unifiedMounts {
		finalMounts = append(finalMounts, mount)
	}
	return finalMounts, nil
}

// getMounts takes user-provided input from the --mount flag and creates OCI
// spec mounts.
// buildah run --mount type=bind,src=/etc/resolv.conf,target=/etc/resolv.conf ...
// buildah run --mount type=tmpfs,target=/dev/shm ...
func getMounts(mounts []string) (map[string]specs.Mount, error) {
	finalMounts := make(map[string]specs.Mount)

	errInvalidSyntax := errors.Errorf("incorrect mount format: should be --mount type=<bind|tmpfs>,[src=<host-dir>,]target=<ctr-dir>[,options]")

	// TODO(vrothberg): the manual parsing can be replaced with a regular expression
	//                  to allow a more robust parsing of the mount format and to give
	//                  precise errors regarding supported format versus suppored options.
	for _, mount := range mounts {
		arr := strings.SplitN(mount, ",", 2)
		if len(arr) < 2 {
			return nil, errors.Wrapf(errInvalidSyntax, "%q", mount)
		}
		kv := strings.Split(arr[0], "=")
		// TODO: type is not explicitly required in Docker.
		// If not specified, it defaults to "volume".
		if len(kv) != 2 || kv[0] != "type" {
			return nil, errors.Wrapf(errInvalidSyntax, "%q", mount)
		}

		tokens := strings.Split(arr[1], ",")
		switch kv[1] {
		case TypeBind:
			mount, err := GetBindMount(tokens)
			if err != nil {
				return nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, errors.Wrapf(errDuplicateDest, mount.Destination)
			}
			finalMounts[mount.Destination] = mount
		case TypeTmpfs:
			mount, err := GetTmpfsMount(tokens)
			if err != nil {
				return nil, err
			}
			if _, ok := finalMounts[mount.Destination]; ok {
				return nil, errors.Wrapf(errDuplicateDest, mount.Destination)
			}
			finalMounts[mount.Destination] = mount
		default:
			return nil, errors.Errorf("invalid filesystem type %q", kv[1])
		}
	}

	return finalMounts, nil
}

// GetBindMount parses a single bind mount entry from the --mount flag.
func GetBindMount(args []string) (specs.Mount, error) {
	newMount := specs.Mount{
		Type: TypeBind,
	}

	setSource := false
	setDest := false

	for _, val := range args {
		kv := strings.Split(val, "=")
		switch kv[0] {
		case "bind-nonrecursive":
			newMount.Options = append(newMount.Options, "bind")
		case "ro", "nosuid", "nodev", "noexec":
			// TODO: detect duplication of these options.
			// (Is this necessary?)
			newMount.Options = append(newMount.Options, kv[0])
		case "shared", "rshared", "private", "rprivate", "slave", "rslave", "Z", "z":
			newMount.Options = append(newMount.Options, kv[0])
		case "bind-propagation":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			newMount.Options = append(newMount.Options, kv[1])
		case "src", "source":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			if err := ValidateVolumeHostDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Source = kv[1]
			setSource = true
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			if err := ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = kv[1]
			setDest = true
		default:
			return newMount, errors.Wrapf(errBadMntOption, kv[0])
		}
	}

	if !setDest {
		return newMount, noDestError
	}

	if !setSource {
		newMount.Source = newMount.Destination
	}

	opts, err := ValidateVolumeOpts(newMount.Options)
	if err != nil {
		return newMount, err
	}
	newMount.Options = opts

	return newMount, nil
}

// GetTmpfsMount parses a single tmpfs mount entry from the --mount flag
func GetTmpfsMount(args []string) (specs.Mount, error) {
	newMount := specs.Mount{
		Type:   TypeTmpfs,
		Source: TypeTmpfs,
	}

	setDest := false

	for _, val := range args {
		kv := strings.Split(val, "=")
		switch kv[0] {
		case "ro", "nosuid", "nodev", "noexec":
			newMount.Options = append(newMount.Options, kv[0])
		case "tmpfs-mode":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("mode=%s", kv[1]))
		case "tmpfs-size":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			newMount.Options = append(newMount.Options, fmt.Sprintf("size=%s", kv[1]))
		case "src", "source":
			return newMount, errors.Errorf("source is not supported with tmpfs mounts")
		case "target", "dst", "destination":
			if len(kv) == 1 {
				return newMount, errors.Wrapf(optionArgError, kv[0])
			}
			if err := ValidateVolumeCtrDir(kv[1]); err != nil {
				return newMount, err
			}
			newMount.Destination = kv[1]
			setDest = true
		default:
			return newMount, errors.Wrapf(errBadMntOption, kv[0])
		}
	}

	if !setDest {
		return newMount, noDestError
	}

	return newMount, nil
}

// ValidateVolumeHostDir validates a volume mount's source directory
func ValidateVolumeHostDir(hostDir string) error {
	if len(hostDir) == 0 {
		return errors.Errorf("host directory cannot be empty")
	}
	if filepath.IsAbs(hostDir) {
		if _, err := os.Stat(hostDir); err != nil {
			return errors.Wrapf(err, "error checking path %q", hostDir)
		}
	}
	// If hostDir is not an absolute path, that means the user wants to create a
	// named volume. This will be done later on in the code.
	return nil
}

// validates the host path of buildah --volume
func validateVolumeMountHostDir(hostDir string) error {
	if !filepath.IsAbs(hostDir) {
		return errors.Errorf("invalid host path, must be an absolute path %q", hostDir)
	}
	if _, err := os.Stat(hostDir); err != nil {
		return errors.Wrapf(err, "error checking path %q", hostDir)
	}
	return nil
}

// ValidateVolumeCtrDir validates a volume mount's destination directory.
func ValidateVolumeCtrDir(ctrDir string) error {
	if len(ctrDir) == 0 {
		return errors.Errorf("container directory cannot be empty")
	}
	if !filepath.IsAbs(ctrDir) {
		return errors.Errorf("invalid container path %q, must be an absolute path", ctrDir)
	}
	return nil
}

// ValidateVolumeOpts validates a volume's options
func ValidateVolumeOpts(options []string) ([]string, error) {
	var foundRootPropagation, foundRWRO, foundLabelChange, bindType int
	finalOpts := make([]string, 0, len(options))
	for _, opt := range options {
		switch opt {
		case "rw", "ro":
			if foundRWRO > 1 {
				return nil, errors.Errorf("invalid options %q, can only specify 1 'rw' or 'ro' option", strings.Join(options, ", "))
			}
			foundRWRO++
		case "z", "Z", "O":
			if opt == "O" && unshare.IsRootless() {
				return nil, errors.Errorf("invalid options %q, overlay mounts not supported in rootless mode", strings.Join(options, ", "))
			}
			if foundLabelChange > 1 {
				return nil, errors.Errorf("invalid options %q, can only specify 1 'z', 'Z', or 'O' option", strings.Join(options, ", "))
			}
			foundLabelChange++
		case "private", "rprivate", "shared", "rshared", "slave", "rslave", "unbindable", "runbindable":
			if foundRootPropagation > 1 {
				return nil, errors.Errorf("invalid options %q, can only specify 1 '[r]shared', '[r]private', '[r]slave' or '[r]unbindable' option", strings.Join(options, ", "))
			}
			foundRootPropagation++
		case "bind", "rbind":
			bindType++
			if bindType > 1 {
				return nil, errors.Errorf("invalid options %q, can only specify 1 '[r]bind' option", strings.Join(options, ", "))
			}
		case "cached", "delegated":
			// The discarded ops are OS X specific volume options
			// introduced in a recent Docker version.
			// They have no meaning on Linux, so here we silently
			// drop them. This matches Docker's behavior (the options
			// are intended to be always safe to use, even not on OS
			// X).
			continue
		default:
			return nil, errors.Errorf("invalid option type %q", opt)
		}
		finalOpts = append(finalOpts, opt)
	}
	return finalOpts, nil
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
	certDir, err := c.Flags().GetString("cert-dir")
	if err != nil {
		certDir = ""
	}
	ctx := &types.SystemContext{
		DockerCertPath: certDir,
	}
	tlsVerify, err := c.Flags().GetBool("tls-verify")
	if err == nil && c.Flag("tls-verify").Changed {
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!tlsVerify)
		ctx.OCIInsecureSkipTLSVerify = !tlsVerify
		ctx.DockerDaemonInsecureSkipTLSVerify = !tlsVerify
	}
	creds, err := c.Flags().GetString("creds")
	if err == nil && c.Flag("creds").Changed {
		var err error
		ctx.DockerAuthConfig, err = getDockerAuth(creds)
		if err != nil {
			return nil, err
		}
	}
	sigPolicy, err := c.Flags().GetString("signature-policy")
	if err == nil && c.Flag("signature-policy").Changed {
		ctx.SignaturePolicyPath = sigPolicy
	}
	authfile, err := c.Flags().GetString("authfile")
	if err == nil {
		ctx.AuthFilePath = getAuthFile(authfile)
	}
	regConf, err := c.Flags().GetString("registries-conf")
	if err == nil && c.Flag("registries-conf").Changed {
		ctx.SystemRegistriesConfPath = regConf
	}
	regConfDir, err := c.Flags().GetString("registries-conf-dir")
	if err == nil && c.Flag("registries-conf-dir").Changed {
		ctx.RegistriesDirPath = regConfDir
	}
	ctx.DockerRegistryUserAgent = fmt.Sprintf("Buildah/%s", buildah.Version)
	return ctx, nil
}

func getAuthFile(authfile string) string {
	if authfile != "" {
		return authfile
	}
	return os.Getenv("REGISTRY_AUTH_FILE")
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

// IDMappingOptions parses the build options related to user namespaces and ID mapping.
func IDMappingOptions(c *cobra.Command, isolation buildah.Isolation) (usernsOptions buildah.NamespaceOptions, idmapOptions *buildah.IDMappingOptions, err error) {
	user := c.Flag("userns-uid-map-user").Value.String()
	group := c.Flag("userns-gid-map-group").Value.String()
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
	globalOptions := c.PersistentFlags()
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
		if c.Flag(option).Changed {
			spec, _ = c.Flags().GetStringSlice(option)
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
	usernsOption := buildah.NamespaceOption{
		Name: string(specs.UserNamespace),
		Host: len(uidmap) == 0 && len(gidmap) == 0,
	}
	// If the user specifically requested that we either use or don't use
	// user namespaces, override that default.
	if c.Flag("userns").Changed {
		how := c.Flag("userns").Value.String()
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

	// Because --net and --network are technically two different flags, we need
	// to check each for nil and .Changed
	usernet := c.Flags().Lookup("net")
	usernetwork := c.Flags().Lookup("network")
	if (usernet != nil && usernetwork != nil) && (!usernet.Changed && !usernetwork.Changed) {
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

// NamespaceOptions parses the build options for all namespaces except for user namespace.
func NamespaceOptions(c *cobra.Command) (namespaceOptions buildah.NamespaceOptions, networkPolicy buildah.NetworkConfigurationPolicy, err error) {
	options := make(buildah.NamespaceOptions, 0, 7)
	policy := buildah.NetworkDefault
	for _, what := range []string{string(specs.IPCNamespace), "net", "network", string(specs.PIDNamespace), string(specs.UTSNamespace)} {
		if c.Flags().Lookup(what) != nil && c.Flag(what).Changed {
			how := c.Flag(what).Value.String()
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
				if what == string(specs.NetworkNamespace) {
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

func defaultIsolation() (buildah.Isolation, error) {
	isolation, isSet := os.LookupEnv("BUILDAH_ISOLATION")
	if isSet {
		switch strings.ToLower(isolation) {
		case "oci":
			return buildah.IsolationOCI, nil
		case "rootless":
			return buildah.IsolationOCIRootless, nil
		case "chroot":
			return buildah.IsolationChroot, nil
		default:
			return 0, errors.Errorf("unrecognized $BUILDAH_ISOLATION value %q", isolation)
		}
	}
	return buildah.IsolationDefault, nil
}

// IsolationOption parses the --isolation flag.
func IsolationOption(c *cobra.Command) (buildah.Isolation, error) {
	isolation, _ := c.Flags().GetString("isolation")
	if isolation != "" {
		switch strings.ToLower(isolation) {
		case "oci":
			return buildah.IsolationOCI, nil
		case "rootless":
			return buildah.IsolationOCIRootless, nil
		case "chroot":
			return buildah.IsolationChroot, nil
		default:
			return 0, errors.Errorf("unrecognized isolation type %q", isolation)
		}
	}
	return defaultIsolation()
}

// ScrubServer removes 'http://' or 'https://' from the front of the
// server/registry string if either is there.  This will be mostly used
// for user input from 'buildah login' and 'buildah logout'.
func ScrubServer(server string) string {
	server = strings.TrimPrefix(server, "https://")
	return strings.TrimPrefix(server, "http://")
}

// RegistryFromFullName gets the registry from the input. If the input is of the form
// quay.io/myuser/myimage, it will parse it and just return quay.io
// It also returns true if a full image name was given
func RegistryFromFullName(input string) string {
	split := strings.Split(input, "/")
	if len(split) > 1 {
		return split[0]
	}
	return split[0]
}
