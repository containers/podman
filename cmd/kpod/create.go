package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

type mountType string

// Type constants
const (
	// TypeBind is the type for mounting host dir
	TypeBind mountType = "bind"
	// TypeVolume is the type for remote storage volumes
	// TypeVolume mountType = "volume"  // re-enable upon use
	// TypeTmpfs is the type for mounting tmpfs
	TypeTmpfs mountType = "tmpfs"
)

var (
	defaultEnvVariables = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "TERM=xterm"}
)

type createResourceConfig struct {
	blkioDevice       []string // blkio-weight-device
	blkioWeight       uint16   // blkio-weight
	cpuPeriod         uint64   // cpu-period
	cpuQuota          int64    // cpu-quota
	cpuRtPeriod       uint64   // cpu-rt-period
	cpuRtRuntime      int64    // cpu-rt-runtime
	cpuShares         uint64   // cpu-shares
	cpus              string   // cpus
	cpusetCpus        string
	cpusetMems        string   // cpuset-mems
	deviceReadBps     []string // device-read-bps
	deviceReadIops    []string // device-read-iops
	deviceWriteBps    []string // device-write-bps
	deviceWriteIops   []string // device-write-iops
	disableOomKiller  bool     // oom-kill-disable
	kernelMemory      int64    // kernel-memory
	memory            int64    //memory
	memoryReservation int64    // memory-reservation
	memorySwap        int64    //memory-swap
	memorySwapiness   uint64   // memory-swappiness
	oomScoreAdj       int      //oom-score-adj
	pidsLimit         int64    // pids-limit
	shmSize           string
	ulimit            []string //ulimit
}

type createConfig struct {
	args               []string
	capAdd             []string // cap-add
	capDrop            []string // cap-drop
	cidFile            string
	cgroupParent       string // cgroup-parent
	command            []string
	detach             bool         // detach
	devices            []*pb.Device // device
	dnsOpt             []string     //dns-opt
	dnsSearch          []string     //dns-search
	dnsServers         []string     //dns
	entrypoint         string       //entrypoint
	env                []string     //env
	expose             []string     //expose
	groupAdd           []uint32     // group-add
	hostname           string       //hostname
	image              string
	interactive        bool              //interactive
	ip6Address         string            //ipv6
	ipAddress          string            //ip
	labels             map[string]string //label
	linkLocalIP        []string          // link-local-ip
	logDriver          string            // log-driver
	logDriverOpt       []string          // log-opt
	macAddress         string            //mac-address
	name               string            //name
	network            string            //network
	networkAlias       []string          //network-alias
	nsIPC              string            // ipc
	nsNet              string            //net
	nsPID              string            //pid
	nsUser             string
	pod                string   //pod
	privileged         bool     //privileged
	publish            []string //publish
	publishAll         bool     //publish-all
	readOnlyRootfs     bool     //read-only
	resources          createResourceConfig
	rm                 bool              //rm
	sigProxy           bool              //sig-proxy
	stopSignal         string            // stop-signal
	stopTimeout        int64             // stop-timeout
	storageOpts        []string          //storage-opt
	sysctl             map[string]string //sysctl
	tmpfs              []string          // tmpfs
	tty                bool              //tty
	user               uint32            //user
	group              uint32            // group
	volumes            []string          //volume
	volumesFrom        []string          //volumes-from
	workDir            string            //workdir
	mountLabel         string            //SecurityOpts
	processLabel       string            //SecurityOpts
	noNewPrivileges    bool              //SecurityOpts
	apparmorProfile    string            //SecurityOpts
	seccompProfilePath string            //SecurityOpts
}

var createDescription = "Creates a new container from the given image or" +
	" storage and prepares it for running the specified command. The" +
	" container ID is then printed to stdout. You can then start it at" +
	" any time with the kpod start <container_id> command. The container" +
	" will be created with the initial state 'created'."

var createCommand = cli.Command{
	Name:           "create",
	Usage:          "create but do not start a container",
	Description:    createDescription,
	Flags:          createFlags,
	Action:         createCmd,
	ArgsUsage:      "IMAGE [COMMAND [ARG...]]",
	SkipArgReorder: true,
}

func createCmd(c *cli.Context) error {
	// TODO should allow user to create based off a directory on the host not just image
	// Need CLI support for this
	var imageName string
	if err := validateFlags(c, createFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}

	createConfig, err := parseCreateOpts(c, runtime)
	if err != nil {
		return err
	}

	// Deal with the image after all the args have been checked
	createImage := runtime.NewImage(createConfig.image)
	createImage.LocalName, _ = createImage.GetLocalImageName()
	if createImage.LocalName == "" {
		// The image wasnt found by the user input'd name or its fqname
		// Pull the image
		fmt.Printf("Trying to pull %s...", createImage.PullName)
		createImage.Pull()
	}

	runtimeSpec, err := createConfigToOCISpec(createConfig)
	if err != nil {
		return err
	}
	defer runtime.Shutdown(false)
	if createImage.LocalName != "" {
		imageName = createImage.LocalName
	} else {
		imageName, err = createImage.GetFQName()
	}
	if err != nil {
		return err
	}
	imageID, err := createImage.GetImageID()
	if err != nil {
		return err
	}
	options, err := createConfig.GetContainerCreateOptions()
	if err != nil {
		return errors.Wrapf(err, "unable to parse new container options")
	}
	// Gather up the options for NewContainer which consist of With... funcs
	options = append(options, libpod.WithRootFSFromImage(imageID, imageName, false))
	options = append(options, libpod.WithSELinuxMountLabel(createConfig.mountLabel))
	ctr, err := runtime.NewContainer(runtimeSpec, options...)
	if err != nil {
		return err
	}

	if c.String("cidfile") != "" {
		libpod.WriteFile(ctr.ID(), c.String("cidfile"))
	} else {
		fmt.Printf("%s\n", ctr.ID())
	}

	return nil
}

const seccompDefaultPath = "/etc/crio/seccomp.json"

func parseSecurityOpt(config *createConfig, securityOpts []string) error {
	var (
		labelOpts []string
		err       error
	)

	for _, opt := range securityOpts {
		if opt == "no-new-privileges" {
			config.noNewPrivileges = true
		} else {
			con := strings.SplitN(opt, "=", 2)
			if len(con) != 2 {
				return fmt.Errorf("Invalid --security-opt 1: %q", opt)
			}

			switch con[0] {
			case "label":
				labelOpts = append(labelOpts, con[1])
			case "apparmor":
				config.apparmorProfile = con[1]
			case "seccomp":
				config.seccompProfilePath = con[1]
			default:
				return fmt.Errorf("Invalid --security-opt 2: %q", opt)
			}
		}
	}

	if config.seccompProfilePath == "" {
		if _, err := os.Stat(seccompDefaultPath); err != nil {
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "can't check if %q exists", seccompDefaultPath)
			}
		} else {
			config.seccompProfilePath = seccompDefaultPath
		}
	}
	config.processLabel, config.mountLabel, err = label.InitLabels(labelOpts)
	return err
}

// Parses CLI options related to container creation into a config which can be
// parsed into an OCI runtime spec
func parseCreateOpts(c *cli.Context, runtime *libpod.Runtime) (*createConfig, error) {
	var command []string
	var memoryLimit, memoryReservation, memorySwap, memoryKernel int64
	var blkioWeight uint16
	var uid, gid uint32

	if len(c.Args()) < 1 {
		return nil, errors.Errorf("image name or ID is required")
	}
	image := c.Args()[0]

	if len(c.Args()) > 1 {
		command = c.Args()[1:]
	}

	// LABEL VARIABLES
	labels, err := getAllLabels(c.StringSlice("label-file"), c.StringSlice("labels"))
	if err != nil {
		return &createConfig{}, errors.Wrapf(err, "unable to process labels")
	}
	// ENVIRONMENT VARIABLES
	env, err := getAllEnvironmentVariables(c.StringSlice("env-file"), c.StringSlice("env"))
	if err != nil {
		return &createConfig{}, errors.Wrapf(err, "unable to process environment variables")
	}

	sysctl, err := convertStringSliceToMap(c.StringSlice("sysctl"), "=")
	if err != nil {
		return &createConfig{}, errors.Wrapf(err, "sysctl values must be in the form of KEY=VALUE")
	}

	groupAdd, err := stringSlicetoUint32Slice(c.StringSlice("group-add"))
	if err != nil {
		return &createConfig{}, errors.Wrapf(err, "invalid value for groups provided")
	}

	if c.String("user") != "" {
		// TODO
		// We need to mount the imagefs and get the uid/gid
		// For now, user zeros
		uid = 0
		gid = 0
	}

	if c.String("memory") != "" {
		memoryLimit, err = units.RAMInBytes(c.String("memory"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory")
		}
	}
	if c.String("memory-reservation") != "" {
		memoryReservation, err = units.RAMInBytes(c.String("memory-reservation"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory-reservation")
		}
	}
	if c.String("memory-swap") != "" {
		memorySwap, err = units.RAMInBytes(c.String("memory-swap"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for memory-swap")
		}
	}
	if c.String("kernel-memory") != "" {
		memoryKernel, err = units.RAMInBytes(c.String("kernel-memory"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for kernel-memory")
		}
	}
	if c.String("blkio-weight") != "" {
		u, err := strconv.ParseUint(c.String("blkio-weight"), 10, 16)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid value for blkio-weight")
		}
		blkioWeight = uint16(u)
	}

	// Because we cannot do a non-terminal attach, we need to set tty to true
	// if detach is not false
	// TODO Allow non-terminal attach
	tty := c.Bool("tty")
	if !c.Bool("detach") && !tty {
		tty = true
	}

	config := &createConfig{
		capAdd:         c.StringSlice("cap-add"),
		capDrop:        c.StringSlice("cap-drop"),
		cgroupParent:   c.String("cgroup-parent"),
		command:        command,
		detach:         c.Bool("detach"),
		dnsOpt:         c.StringSlice("dns-opt"),
		dnsSearch:      c.StringSlice("dns-search"),
		dnsServers:     c.StringSlice("dns"),
		entrypoint:     c.String("entrypoint"),
		env:            env,
		expose:         c.StringSlice("env"),
		groupAdd:       groupAdd,
		hostname:       c.String("hostname"),
		image:          image,
		interactive:    c.Bool("interactive"),
		ip6Address:     c.String("ipv6"),
		ipAddress:      c.String("ip"),
		labels:         labels,
		linkLocalIP:    c.StringSlice("link-local-ip"),
		logDriver:      c.String("log-driver"),
		logDriverOpt:   c.StringSlice("log-opt"),
		macAddress:     c.String("mac-address"),
		name:           c.String("name"),
		network:        c.String("network"),
		networkAlias:   c.StringSlice("network-alias"),
		nsIPC:          c.String("ipc"),
		nsNet:          c.String("net"),
		nsPID:          c.String("pid"),
		pod:            c.String("pod"),
		privileged:     c.Bool("privileged"),
		publish:        c.StringSlice("publish"),
		publishAll:     c.Bool("publish-all"),
		readOnlyRootfs: c.Bool("read-only"),
		resources: createResourceConfig{
			blkioWeight:       blkioWeight,
			blkioDevice:       c.StringSlice("blkio-weight-device"),
			cpuShares:         c.Uint64("cpu-shares"),
			cpuPeriod:         c.Uint64("cpu-period"),
			cpusetCpus:        c.String("cpu-period"),
			cpusetMems:        c.String("cpuset-mems"),
			cpuQuota:          c.Int64("cpu-quota"),
			cpuRtPeriod:       c.Uint64("cpu-rt-period"),
			cpuRtRuntime:      c.Int64("cpu-rt-runtime"),
			cpus:              c.String("cpus"),
			deviceReadBps:     c.StringSlice("device-read-bps"),
			deviceReadIops:    c.StringSlice("device-read-iops"),
			deviceWriteBps:    c.StringSlice("device-write-bps"),
			deviceWriteIops:   c.StringSlice("device-write-iops"),
			disableOomKiller:  c.Bool("oom-kill-disable"),
			shmSize:           c.String("shm-size"),
			memory:            memoryLimit,
			memoryReservation: memoryReservation,
			memorySwap:        memorySwap,
			memorySwapiness:   c.Uint64("memory-swapiness"),
			kernelMemory:      memoryKernel,
			oomScoreAdj:       c.Int("oom-score-adj"),

			pidsLimit: c.Int64("pids-limit"),
			ulimit:    c.StringSlice("ulimit"),
		},
		rm:          c.Bool("rm"),
		sigProxy:    c.Bool("sig-proxy"),
		stopSignal:  c.String("stop-signal"),
		stopTimeout: c.Int64("stop-timeout"),
		storageOpts: c.StringSlice("storage-opt"),
		sysctl:      sysctl,
		tmpfs:       c.StringSlice("tmpfs"),
		tty:         tty,
		user:        uid,
		group:       gid,
		volumes:     c.StringSlice("volume"),
		volumesFrom: c.StringSlice("volumes-from"),
		workDir:     c.String("workdir"),
	}

	if !config.privileged {
		if err := parseSecurityOpt(config, c.StringSlice("security-opt")); err != nil {
			return nil, err
		}
	}

	return config, nil
}
