package cni

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/containers/storage/pkg/reexec"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
)

const (
	// InfraCmd - always use a absolute path we should not rely on $PATH
	// this also has to be a user writable location and /run is writable for the user
	InfraCmd            = "/run/rootless-cni-infra-exe"
	infraCreateNetNSCmd = "rootless-cni-infra-create-netns"
	basePath            = "/run/rootless-cni-infra"
	// Version - you should bump the Version if you do breaking changes to this script
	Version = 6
)

// Config passed via stdin as json
type Config struct {
	// ID - container ID
	ID string
	// Network - network name
	Network string
	// CNIPodName - name used for the dns entry by the dnsname plugin
	CNIPodName string
	// IP - static IP address
	IP string
	// MAC - static mac address
	MAC string
	// Aliases - network aliases, further dns entries for the dnsname plugin
	Aliases map[string][]string
	// InterfaceName - network interface name in the container for this network (e.g eth0)
	InterfaceName string
	// PluginPaths - search paths for the cni plugins
	PluginPaths []string
	// NetConfPath - path where the cni config files are located
	NetConfPath string
}

// PrintNetnsPath is returned by print-netns-path as json
type PrintNetnsPath struct {
	Path string `json:"path"`
}

// IsIdle is returned by is-idle as json
type IsIdle struct {
	Idle bool `json:"idle"`
}

func printErrorf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}

func printJSONResult(v interface{}) {
	b, err := json.Marshal(v)
	if err != nil {
		printErrorf("%s", err)
	}
	fmt.Println(string(b))
}

func init() {
	reexec.Register(InfraCmd, func() {
		if len(os.Args) < 2 {
			exit(errors.Errorf("%s requires at least one arg", InfraCmd))
		}

		switch os.Args[1] {
		case "alloc":
			alloc()

		case "dealloc":
			dealloc()

		case "is-idle":
			idle := IsIdle{
				Idle: false,
			}
			empty, err := dirIsEmpty(basePath)
			if os.IsNotExist(err) || empty {
				idle.Idle = true
			} else if err != nil {
				printErrorf("%s", err)
			}
			printJSONResult(idle)

		case "print-netns-path":
			if len(os.Args) != 3 {
				exit(errors.Errorf("%s print-netns-path requires one arg", InfraCmd))
			}
			pidfile := path.Join(basePath, os.Args[2], "pid")
			path, err := getNetNamespacePath(pidfile)
			if err != nil {
				exit(err)
			}
			var netns PrintNetnsPath
			netns.Path = path
			printJSONResult(netns)

		case "sleep":
			// sleep subcommand used to keep the namespace alive
			// sleep max duration
			time.Sleep(time.Duration(1<<63 - 1))

		default:
			exit(errors.Errorf("Unknown command: %s %s", InfraCmd, os.Args[1]))
		}
	})

	reexec.Register(infraCreateNetNSCmd, func() {
		if len(os.Args) != 2 {
			exit(errors.Errorf("%s requires one arg", infraCreateNetNSCmd))
		}
		pidfile := os.Args[1]
		if err := os.MkdirAll(path.Dir(pidfile), 0700); err != nil {
			exit(err)
		}
		// create new net namespace
		if err := syscall.Unshare(syscall.CLONE_NEWNET); err != nil {
			exit(err)
		}

		// background process to keep the net namespace alive
		sleep := reexec.Command(InfraCmd, "sleep")
		if err := sleep.Start(); err != nil {
			exit(err)
		}
		pid := sleep.Process.Pid
		stringPid := strconv.Itoa(pid)

		if err := ioutil.WriteFile(pidfile, []byte(stringPid), 0700); err != nil {
			exit(errors.Wrap(err, "failed to write pid file"))
		}

		// set the loopback adapter up
		lo, err := netlink.LinkByName("lo")
		if err != nil {
			exit(errors.Wrap(err, "failed to get the loopback adapter"))
		}
		if err = netlink.LinkSetUp(lo); err != nil {
			exit(errors.Wrap(err, "failed to set the loopback adapter up"))
		}
	})
}

// exit with ec 0 if error is nil otherwise exit with ec 1 and log the error to stderr
func exit(err error) {
	if err != nil {
		printErrorf("%s", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func dirIsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	names, err := f.Readdirnames(1)
	// Readdirnames returns EOF error if it is empty
	if len(names) == 0 && err == io.EOF {
		return true, nil
	}
	return false, err
}

// readConfigFromStdin reads the config from stdin
func readConfigFromStdin() (*Config, error) {
	var config Config
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read from stdin")
	}
	if stat.Mode()&os.ModeNamedPipe == 0 {
		return nil, errors.New("nothing to read from stdin")
	}
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read RootlessCNIConfig json")
	}
	return &config, nil
}

func getNetNamespacePath(pidfile string) (string, error) {
	b, err := ioutil.ReadFile(pidfile)
	if err != nil {
		return "", errors.Wrap(err, "failed to read pid file")
	}
	pid := string(b)
	return path.Join("/proc", pid, "ns", "net"), err
}

func createNetNamespace(pidfile string) (string, error) {
	rcmd := reexec.Command(infraCreateNetNSCmd, pidfile)
	rcmd.Stderr = os.Stderr
	rcmd.Stdout = os.Stdout
	if err := rcmd.Run(); err != nil {
		return "", errors.Wrap(err, "failed to create network namespace")
	}
	return getNetNamespacePath(pidfile)
}

func createCNIconfigs(cfg *Config) (*libcni.CNIConfig, *libcni.NetworkConfigList, *libcni.RuntimeConf) {
	args := [][2]string{
		{"IgnoreUnknown", "1"},
		{"K8S_POD_NAME", cfg.CNIPodName},
	}
	// add static ip if given
	if cfg.IP != "" {
		args = append(args, [2]string{"IP", cfg.IP})
	}
	// add static mac if given
	if cfg.MAC != "" {
		args = append(args, [2]string{"MAC", cfg.MAC})
	}

	// add aliases
	capabilityArgs := make(map[string]interface{})
	if len(cfg.Aliases) > 0 {
		capabilityArgs["aliases"] = cfg.Aliases
	}

	rt := &libcni.RuntimeConf{
		ContainerID:    cfg.ID,
		IfName:         cfg.InterfaceName,
		Args:           args,
		CapabilityArgs: capabilityArgs,
	}

	netconf, err := libcni.LoadConfList(cfg.NetConfPath, cfg.Network)
	if err != nil {
		cleanupErr := cleanupFiles(getPaths(cfg.ID, cfg.Network))
		printErrorf("%v", cleanupErr)
		exit(err)
	}

	cninet := libcni.NewCNIConfig(cfg.PluginPaths, nil)

	return cninet, netconf, rt
}

func getPaths(cid, net string) (string, string) {
	base := path.Join(basePath, cid)
	pidfile := path.Join(base, "pid")
	netfile := path.Join(base, "networks", net)
	return pidfile, netfile
}

func alloc() {
	conf, err := readConfigFromStdin()
	if err != nil {
		exit(err)
	}
	pidfile, netfile := getPaths(conf.ID, conf.Network)
	ns, err := getNetNamespacePath(pidfile)
	if err != nil && !os.IsNotExist(errors.Cause(err)) {
		exit(err)
	}
	// if namespace path does not exists create new namespace
	if os.IsNotExist(errors.Cause(err)) {
		ns, err = createNetNamespace(pidfile)
		if err != nil {
			exit(err)
		}
	}

	if err := os.MkdirAll(path.Dir(netfile), 0700); err != nil {
		exit(err)
	}
	// create a file to keep track of the attached networks
	_, err = os.Create(netfile)
	if err != nil {
		exit(err)
	}

	// prepare the cni configs
	cninet, netconf, rt := createCNIconfigs(conf)
	rt.NetNS = ns

	// call cni to add the network
	res, err := cninet.AddNetworkList(context.TODO(), netconf, rt)
	if err != nil {
		// cleanup to make sure we don't have dangling files
		// this is important to detect is-idle correctly
		cleanupErr := cleanupFiles(pidfile, netfile)
		if cleanupErr != nil {
			printErrorf("%v", cleanupErr)
		}
		exit(errors.Wrapf(err, "failed to attach to cni network %s", conf.Network))
	}
	// print res to stdout
	res.Print()
}

func dealloc() {
	conf, err := readConfigFromStdin()
	if err != nil {
		exit(err)
	}
	pidfile, netfile := getPaths(conf.ID, conf.Network)
	ns, err := getNetNamespacePath(pidfile)
	if err != nil && !os.IsNotExist(err) {
		exit(err)
	}
	if os.IsNotExist(err) {
		// if the file does not exists the namespace is probably already deleted
		// exit without error
		exit(nil)
	}

	// prepare the cni configs
	cninet, netconf, rt := createCNIconfigs(conf)
	rt.NetNS = ns

	// call cni to remove the network
	err = cninet.DelNetworkList(context.TODO(), netconf, rt)
	if err != nil {
		exit(errors.Wrapf(err, "failed to detach cni network %s", conf.Network))
	}

	err = cleanupFiles(pidfile, netfile)
	if err != nil {
		exit(err)
	}

	// print empty json result
	// we have no information to return
	fmt.Println("{}")
}

func cleanupFiles(pidfile, netfile string) error {
	// remove the config file
	err := os.Remove(netfile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// check if the config directory is empty
	empty, err := dirIsEmpty(path.Dir(netfile))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if empty {
		// if it is empty no more networks are attached to this container
		// therefore kill the net namespace
		var piderr error
		b, err := ioutil.ReadFile(pidfile)
		if err == nil {
			pid, err := strconv.Atoi(string(b))
			if err == nil {
				// kill the pause process which keeps the net ns alive
				err = syscall.Kill(pid, syscall.SIGKILL)
				if err != nil {
					piderr = errors.Wrap(err, "ailed to kill the pause process")
				}
			} else {
				piderr = errors.Wrap(err, "failed to parse the pid")
			}
		} else {
			piderr = errors.Wrap(err, "failed to read the pid file")
		}
		// remove all remaining configuration files for this container
		// always remove even if the pidfile parsing failed to ensure we do not have dangling files
		err = os.RemoveAll(path.Dir(pidfile))
		if err != nil {
			return err
		}
		return piderr
	}
	return nil
}
