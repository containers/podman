//nolint
// most of these validate and parse functions have been taken from projectatomic/docker
// and modified for cri-o
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	units "github.com/docker/go-units"
	"github.com/pkg/errors"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// Note: for flags that are in the form <number><unit>, use the RAMInBytes function
// from the units package in docker/go-units/size.go

var (
	whiteSpaces  = " \t"
	alphaRegexp  = regexp.MustCompile(`[a-zA-Z]`)
	domainRegexp = regexp.MustCompile(`^(:?(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9]))(:?\.(:?[a-zA-Z0-9]|(:?[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])))*)\.?\s*$`)
)

// validateExtraHost validates that the specified string is a valid extrahost and returns it.
// ExtraHost is in the form of name:ip where the ip has to be a valid ip (ipv4 or ipv6).
// for add-host flag
func validateExtraHost(val string) (string, error) { //nolint
	// allow for IPv6 addresses in extra hosts by only splitting on first ":"
	arr := strings.SplitN(val, ":", 2)
	if len(arr) != 2 || len(arr[0]) == 0 {
		return "", fmt.Errorf("bad format for add-host: %q", val)
	}
	if _, err := validateIPAddress(arr[1]); err != nil {
		return "", fmt.Errorf("invalid IP address in add-host: %q", arr[1])
	}
	return val, nil
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

// validateAttach validates that the specified string is a valid attach option.
// for attach flag
func validateAttach(val string) (string, error) { //nolint
	s := strings.ToLower(val)
	for _, str := range []string{"stdin", "stdout", "stderr"} {
		if s == str {
			return s, nil
		}
	}
	return val, fmt.Errorf("valid streams are STDIN, STDOUT and STDERR")
}

// validate the blkioWeight falls in the range of 10 to 1000
// for blkio-weight flag
func validateBlkioWeight(val int64) (int64, error) { //nolint
	if val >= 10 && val <= 1000 {
		return val, nil
	}
	return -1, errors.Errorf("invalid blkio weight %q, should be between 10 and 1000", val)
}

// weightDevice is a structure that holds device:weight pair
type weightDevice struct {
	path   string
	weight uint16
}

func (w *weightDevice) String() string {
	return fmt.Sprintf("%s:%d", w.path, w.weight)
}

// validateweightDevice validates that the specified string has a valid device-weight format
// for blkio-weight-device flag
func validateweightDevice(val string) (*weightDevice, error) {
	split := strings.SplitN(val, ":", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("bad format: %s", val)
	}
	if !strings.HasPrefix(split[0], "/dev/") {
		return nil, fmt.Errorf("bad format for device path: %s", val)
	}
	weight, err := strconv.ParseUint(split[1], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("invalid weight for device: %s", val)
	}
	if weight > 0 && (weight < 10 || weight > 1000) {
		return nil, fmt.Errorf("invalid weight for device: %s", val)
	}

	return &weightDevice{
		path:   split[0],
		weight: uint16(weight),
	}, nil
}

// parseDevice parses a device mapping string to a container.DeviceMapping struct
// for device flag
func parseDevice(device string) (*pb.Device, error) { //nolint
	_, err := validateDevice(device)
	if err != nil {
		return nil, errors.Wrapf(err, "device string not valid %q", device)
	}

	src := ""
	dst := ""
	permissions := "rwm"
	arr := strings.Split(device, ":")
	switch len(arr) {
	case 3:
		permissions = arr[2]
		fallthrough
	case 2:
		if validDeviceMode(arr[1]) {
			permissions = arr[1]
		} else {
			dst = arr[1]
		}
		fallthrough
	case 1:
		src = arr[0]
	default:
		return nil, fmt.Errorf("invalid device specification: %s", device)
	}

	if dst == "" {
		dst = src
	}

	deviceMapping := &pb.Device{
		ContainerPath: dst,
		HostPath:      src,
		Permissions:   permissions,
	}
	return deviceMapping, nil
}

// validDeviceMode checks if the mode for device is valid or not.
// Valid mode is a composition of r (read), w (write), and m (mknod).
func validDeviceMode(mode string) bool {
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

// validateDevice validates a path for devices
// It will make sure 'val' is in the form:
//    [host-dir:]container-path[:mode]
// It also validates the device mode.
func validateDevice(val string) (string, error) {
	return validatePath(val, validDeviceMode)
}

func validatePath(val string, validator func(string) bool) (string, error) {
	var containerPath string
	var mode string

	if strings.Count(val, ":") > 2 {
		return val, fmt.Errorf("bad format for path: %s", val)
	}

	split := strings.SplitN(val, ":", 3)
	if split[0] == "" {
		return val, fmt.Errorf("bad format for path: %s", val)
	}
	switch len(split) {
	case 1:
		containerPath = split[0]
		val = path.Clean(containerPath)
	case 2:
		if isValid := validator(split[1]); isValid {
			containerPath = split[0]
			mode = split[1]
			val = fmt.Sprintf("%s:%s", path.Clean(containerPath), mode)
		} else {
			containerPath = split[1]
			val = fmt.Sprintf("%s:%s", split[0], path.Clean(containerPath))
		}
	case 3:
		containerPath = split[1]
		mode = split[2]
		if isValid := validator(split[2]); !isValid {
			return val, fmt.Errorf("bad mode specified: %s", mode)
		}
		val = fmt.Sprintf("%s:%s:%s", split[0], containerPath, mode)
	}

	if !path.IsAbs(containerPath) {
		return val, fmt.Errorf("%s is not an absolute path", containerPath)
	}
	return val, nil
}

// throttleDevice is a structure that holds device:rate_per_second pair
type throttleDevice struct {
	path string
	rate uint64
}

func (t *throttleDevice) String() string {
	return fmt.Sprintf("%s:%d", t.path, t.rate)
}

// validateBpsDevice validates that the specified string has a valid device-rate format
// for device-read-bps and device-write-bps flags
func validateBpsDevice(val string) (*throttleDevice, error) {
	split := strings.SplitN(val, ":", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("bad format: %s", val)
	}
	if !strings.HasPrefix(split[0], "/dev/") {
		return nil, fmt.Errorf("bad format for device path: %s", val)
	}
	rate, err := units.RAMInBytes(split[1])
	if err != nil {
		return nil, fmt.Errorf("invalid rate for device: %s. The correct format is <device-path>:<number>[<unit>]. Number must be a positive integer. Unit is optional and can be kb, mb, or gb", val)
	}
	if rate < 0 {
		return nil, fmt.Errorf("invalid rate for device: %s. The correct format is <device-path>:<number>[<unit>]. Number must be a positive integer. Unit is optional and can be kb, mb, or gb", val)
	}

	return &throttleDevice{
		path: split[0],
		rate: uint64(rate),
	}, nil
}

// validateIOpsDevice validates that the specified string has a valid device-rate format
// for device-write-iops and device-read-iops flags
func validateIOpsDevice(val string) (*throttleDevice, error) { //nolint
	split := strings.SplitN(val, ":", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("bad format: %s", val)
	}
	if !strings.HasPrefix(split[0], "/dev/") {
		return nil, fmt.Errorf("bad format for device path: %s", val)
	}
	rate, err := strconv.ParseUint(split[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid rate for device: %s. The correct format is <device-path>:<number>. Number must be a positive integer", val)
	}
	if rate < 0 {
		return nil, fmt.Errorf("invalid rate for device: %s. The correct format is <device-path>:<number>. Number must be a positive integer", val)
	}

	return &throttleDevice{
		path: split[0],
		rate: uint64(rate),
	}, nil
}

// validateDNSSearch validates domain for resolvconf search configuration.
// A zero length domain is represented by a dot (.).
// for dns-search flag
func validateDNSSearch(val string) (string, error) { //nolint
	if val = strings.Trim(val, " "); val == "." {
		return val, nil
	}
	return validateDomain(val)
}

func validateDomain(val string) (string, error) {
	if alphaRegexp.FindString(val) == "" {
		return "", fmt.Errorf("%s is not a valid domain", val)
	}
	ns := domainRegexp.FindSubmatch([]byte(val))
	if len(ns) > 0 && len(ns[1]) < 255 {
		return string(ns[1]), nil
	}
	return "", fmt.Errorf("%s is not a valid domain", val)
}

// validateEnv validates an environment variable and returns it.
// If no value is specified, it returns the current value using os.Getenv.
// for env flag
func validateEnv(val string) (string, error) { //nolint
	arr := strings.Split(val, "=")
	if len(arr) > 1 {
		return val, nil
	}
	if !doesEnvExist(val) {
		return val, nil
	}
	return fmt.Sprintf("%s=%s", val, os.Getenv(val)), nil
}

func doesEnvExist(name string) bool {
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if parts[0] == name {
			return true
		}
	}
	return false
}

// reads a file of line terminated key=value pairs, and overrides any keys
// present in the file with additional pairs specified in the override parameter
// for env-file and labels-file flags
func readKVStrings(env map[string]string, files []string, override []string) error {
	for _, ef := range files {
		if err := parseEnvFile(env, ef); err != nil {
			return err
		}
	}
	for _, line := range override {
		if err := parseEnv(env, line); err != nil {
			return err
		}
	}
	return nil
}

func parseEnv(env map[string]string, line string) error {
	data := strings.SplitN(line, "=", 2)

	// trim the front of a variable, but nothing else
	name := strings.TrimLeft(data[0], whiteSpaces)
	if strings.ContainsAny(name, whiteSpaces) {
		return errors.Errorf("name %q has white spaces, poorly formatted name", name)
	}

	if len(data) > 1 {
		env[name] = data[1]
	} else {
		// if only a pass-through variable is given, clean it up.
		val, exists := os.LookupEnv(name)
		if !exists {
			return errors.Errorf("environment variable %q does not exist", name)
		}
		env[name] = val
	}
	return nil
}

// parseEnvFile reads a file with environment variables enumerated by lines
func parseEnvFile(env map[string]string, filename string) error {
	fh, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		// trim the line from all leading whitespace first
		line := strings.TrimLeft(scanner.Text(), whiteSpaces)
		// line is not empty, and not starting with '#'
		if len(line) > 0 && !strings.HasPrefix(line, "#") {
			if err := parseEnv(env, line); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// NsIpc represents the container ipc stack.
// for ipc flag
type NsIpc string

// IsPrivate indicates whether the container uses its private ipc stack.
func (n NsIpc) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// IsHost indicates whether the container uses the host's ipc stack.
func (n NsIpc) IsHost() bool {
	return n == "host"
}

// IsContainer indicates whether the container uses a container's ipc stack.
func (n NsIpc) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

// Valid indicates whether the ipc stack is valid.
func (n NsIpc) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	case "container":
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// Container returns the name of the container ipc stack is going to be used.
func (n NsIpc) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// validateLabel validates that the specified string is a valid label, and returns it.
// Labels are in the form on key=value.
// for label flag
func validateLabel(val string) (string, error) { //nolint
	if strings.Count(val, "=") < 1 {
		return "", fmt.Errorf("bad attribute format: %s", val)
	}
	return val, nil
}

// validateMACAddress validates a MAC address.
// for mac-address flag
func validateMACAddress(val string) (string, error) { //nolint
	_, err := net.ParseMAC(strings.TrimSpace(val))
	if err != nil {
		return "", err
	}
	return val, nil
}

// parseLoggingOpts validates the logDriver and logDriverOpts
// for log-opt and log-driver flags
func parseLoggingOpts(logDriver string, logDriverOpt []string) (map[string]string, error) { //nolint
	logOptsMap := convertKVStringsToMap(logDriverOpt)
	if logDriver == "none" && len(logDriverOpt) > 0 {
		return map[string]string{}, errors.Errorf("invalid logging opts for driver %s", logDriver)
	}
	return logOptsMap, nil
}

// NsPid represents the pid namespace of the container.
//for pid flag
type NsPid string

// IsPrivate indicates whether the container uses its own new pid namespace.
func (n NsPid) IsPrivate() bool {
	return !(n.IsHost() || n.IsContainer())
}

// IsHost indicates whether the container uses the host's pid namespace.
func (n NsPid) IsHost() bool {
	return n == "host"
}

// IsContainer indicates whether the container uses a container's pid namespace.
func (n NsPid) IsContainer() bool {
	parts := strings.SplitN(string(n), ":", 2)
	return len(parts) > 1 && parts[0] == "container"
}

// Valid indicates whether the pid namespace is valid.
func (n NsPid) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	case "container":
		if len(parts) != 2 || parts[1] == "" {
			return false
		}
	default:
		return false
	}
	return true
}

// Container returns the name of the container whose pid namespace is going to be used.
func (n NsPid) Container() string {
	parts := strings.SplitN(string(n), ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// parsePortSpecs receives port specs in the format of ip:public:private/proto and parses
// these in to the internal types
// for publish, publish-all, and expose flags
func parsePortSpecs(ports []string) ([]*pb.PortMapping, error) { //nolint
	var portMappings []*pb.PortMapping
	for _, rawPort := range ports {
		portMapping, err := parsePortSpec(rawPort)
		if err != nil {
			return nil, err
		}

		portMappings = append(portMappings, portMapping...)
	}
	return portMappings, nil
}

func validateProto(proto string) bool {
	for _, availableProto := range []string{"tcp", "udp"} {
		if availableProto == proto {
			return true
		}
	}
	return false
}

// parsePortSpec parses a port specification string into a slice of PortMappings
func parsePortSpec(rawPort string) ([]*pb.PortMapping, error) {
	var proto string
	rawIP, hostPort, containerPort := splitParts(rawPort)
	proto, containerPort = splitProtoPort(containerPort)

	// Strip [] from IPV6 addresses
	ip, _, err := net.SplitHostPort(rawIP + ":")
	if err != nil {
		return nil, fmt.Errorf("Invalid ip address %v: %s", rawIP, err)
	}
	if ip != "" && net.ParseIP(ip) == nil {
		return nil, fmt.Errorf("Invalid ip address: %s", ip)
	}
	if containerPort == "" {
		return nil, fmt.Errorf("No port specified: %s<empty>", rawPort)
	}

	startPort, endPort, err := parsePortRange(containerPort)
	if err != nil {
		return nil, fmt.Errorf("Invalid containerPort: %s", containerPort)
	}

	var startHostPort, endHostPort uint64 = 0, 0
	if len(hostPort) > 0 {
		startHostPort, endHostPort, err = parsePortRange(hostPort)
		if err != nil {
			return nil, fmt.Errorf("Invalid hostPort: %s", hostPort)
		}
	}

	if hostPort != "" && (endPort-startPort) != (endHostPort-startHostPort) {
		// Allow host port range iff containerPort is not a range.
		// In this case, use the host port range as the dynamic
		// host port range to allocate into.
		if endPort != startPort {
			return nil, fmt.Errorf("Invalid ranges specified for container and host Ports: %s and %s", containerPort, hostPort)
		}
	}

	if !validateProto(strings.ToLower(proto)) {
		return nil, fmt.Errorf("invalid proto: %s", proto)
	}

	protocol := pb.Protocol_TCP
	if strings.ToLower(proto) == "udp" {
		protocol = pb.Protocol_UDP
	}

	var ports []*pb.PortMapping
	for i := uint64(0); i <= (endPort - startPort); i++ {
		containerPort = strconv.FormatUint(startPort+i, 10)
		if len(hostPort) > 0 {
			hostPort = strconv.FormatUint(startHostPort+i, 10)
		}
		// Set hostPort to a range only if there is a single container port
		// and a dynamic host port.
		if startPort == endPort && startHostPort != endHostPort {
			hostPort = fmt.Sprintf("%s-%s", hostPort, strconv.FormatUint(endHostPort, 10))
		}

		ctrPort, err := strconv.ParseInt(containerPort, 10, 32)
		if err != nil {
			return nil, err
		}
		hPort, err := strconv.ParseInt(hostPort, 10, 32)
		if err != nil {
			return nil, err
		}

		port := &pb.PortMapping{
			Protocol:      protocol,
			ContainerPort: int32(ctrPort),
			HostPort:      int32(hPort),
			HostIp:        ip,
		}

		ports = append(ports, port)
	}
	return ports, nil
}

// parsePortRange parses and validates the specified string as a port-range (8000-9000)
func parsePortRange(ports string) (uint64, uint64, error) {
	if ports == "" {
		return 0, 0, fmt.Errorf("empty string specified for ports")
	}
	if !strings.Contains(ports, "-") {
		start, err := strconv.ParseUint(ports, 10, 16)
		end := start
		return start, end, err
	}

	parts := strings.Split(ports, "-")
	start, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, 0, err
	}
	end, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return 0, 0, err
	}
	if end < start {
		return 0, 0, fmt.Errorf("Invalid range specified for the Port: %s", ports)
	}
	return start, end, nil
}

// splitParts separates the different parts of rawPort
func splitParts(rawport string) (string, string, string) {
	parts := strings.Split(rawport, ":")
	n := len(parts)
	containerport := parts[n-1]

	switch n {
	case 1:
		return "", "", containerport
	case 2:
		return "", parts[0], containerport
	case 3:
		return parts[0], parts[1], containerport
	default:
		return strings.Join(parts[:n-2], ":"), parts[n-2], containerport
	}
}

// splitProtoPort splits a port in the format of port/proto
func splitProtoPort(rawPort string) (string, string) {
	parts := strings.Split(rawPort, "/")
	l := len(parts)
	if len(rawPort) == 0 || l == 0 || len(parts[0]) == 0 {
		return "", ""
	}
	if l == 1 {
		return "tcp", rawPort
	}
	if len(parts[1]) == 0 {
		return "tcp", parts[0]
	}
	return parts[1], parts[0]
}

// takes a local seccomp file and reads its file contents
// for security-opt flag
func parseSecurityOpts(securityOpts []string) ([]string, error) { //nolint
	for key, opt := range securityOpts {
		con := strings.SplitN(opt, "=", 2)
		if len(con) == 1 && con[0] != "no-new-privileges" {
			if strings.Index(opt, ":") != -1 {
				con = strings.SplitN(opt, ":", 2)
			} else {
				return securityOpts, fmt.Errorf("Invalid --security-opt: %q", opt)
			}
		}
		if con[0] == "seccomp" && con[1] != "unconfined" {
			f, err := ioutil.ReadFile(con[1])
			if err != nil {
				return securityOpts, fmt.Errorf("opening seccomp profile (%s) failed: %v", con[1], err)
			}
			b := bytes.NewBuffer(nil)
			if err := json.Compact(b, f); err != nil {
				return securityOpts, fmt.Errorf("compacting json for seccomp profile (%s) failed: %v", con[1], err)
			}
			securityOpts[key] = fmt.Sprintf("seccomp=%s", b.Bytes())
		}
	}

	return securityOpts, nil
}

// parses storage options per container into a map
// for storage-opt flag
func parseStorageOpts(storageOpts []string) (map[string]string, error) { //nolint
	m := make(map[string]string)
	for _, option := range storageOpts {
		if strings.Contains(option, "=") {
			opt := strings.SplitN(option, "=", 2)
			m[opt[0]] = opt[1]
		} else {
			return nil, errors.Errorf("invalid storage option %q", option)
		}
	}
	return m, nil
}

// convertKVStringsToMap converts ["key=value"] to {"key":"value"}
func convertKVStringsToMap(values []string) map[string]string {
	result := make(map[string]string, len(values))
	for _, value := range values {
		kv := strings.SplitN(value, "=", 2)
		if len(kv) == 1 {
			result[kv[0]] = ""
		} else {
			result[kv[0]] = kv[1]
		}
	}

	return result
}

// NsUser represents userns mode in the container.
// for userns flag
type NsUser string

// IsHost indicates whether the container uses the host's userns.
func (n NsUser) IsHost() bool {
	return n == "host"
}

// IsPrivate indicates whether the container uses the a private userns.
func (n NsUser) IsPrivate() bool {
	return !(n.IsHost())
}

// Valid indicates whether the userns is valid.
func (n NsUser) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	default:
		return false
	}
	return true
}

// NsUts represents the UTS namespace of the container.
// for uts flag
type NsUts string

// IsPrivate indicates whether the container uses its private UTS namespace.
func (n NsUts) IsPrivate() bool {
	return !(n.IsHost())
}

// IsHost indicates whether the container uses the host's UTS namespace.
func (n NsUts) IsHost() bool {
	return n == "host"
}

// Valid indicates whether the UTS namespace is valid.
func (n NsUts) Valid() bool {
	parts := strings.Split(string(n), ":")
	switch mode := parts[0]; mode {
	case "", "host":
	default:
		return false
	}
	return true
}

// Takes a stringslice and converts to a uint32slice
func stringSlicetoUint32Slice(inputSlice []string) ([]uint32, error) {
	var outputSlice []uint32
	for _, v := range inputSlice {
		u, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return outputSlice, err
		}
		outputSlice = append(outputSlice, uint32(u))
	}
	return outputSlice, nil
}
