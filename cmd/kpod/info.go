package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/docker/docker/pkg/system"
	"github.com/kubernetes-incubator/cri-o/cmd/kpod/formats"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var (
	infoDescription = "display system information"
	infoCommand     = cli.Command{
		Name:        "info",
		Usage:       infoDescription,
		Description: `Information display here pertain to the host, current storage stats, and build of kpod. Useful for the user and when reporting issues.`,
		Flags:       infoFlags,
		Action:      infoCmd,
		ArgsUsage:   "",
	}
	infoFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, D",
			Usage: "display additional debug information",
		},
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output format to JSON or a Go template",
		},
	}
)

func infoCmd(c *cli.Context) error {
	if err := validateFlags(c, infoFlags); err != nil {
		return err
	}
	info := map[string]interface{}{}

	infoGivers := []infoGiverFunc{
		storeInfo,
		hostInfo,
	}

	if c.Bool("debug") {
		infoGivers = append(infoGivers, debugInfo)
	}

	for _, giver := range infoGivers {
		thisName, thisInfo, err := giver(c)
		if err != nil {
			info[thisName] = infoErr(err)
			continue
		}
		info[thisName] = thisInfo
	}

	var out formats.Writer
	infoOutputFormat := c.String("format")
	switch infoOutputFormat {
	case formats.JSONString:
		out = formats.JSONStruct{Output: info}
	case "":
		out = formats.YAMLStruct{Output: info}
	default:
		out = formats.StdoutTemplate{Output: info, Template: infoOutputFormat}
	}

	formats.Writer(out).Out()

	return nil
}

func infoErr(err error) map[string]interface{} {
	return map[string]interface{}{
		"error": err.Error(),
	}
}

type infoGiverFunc func(c *cli.Context) (name string, info map[string]interface{}, err error)

// top-level "debug" info
func debugInfo(c *cli.Context) (string, map[string]interface{}, error) {
	info := map[string]interface{}{}
	info["compiler"] = runtime.Compiler
	info["go version"] = runtime.Version()
	info["kpod version"] = c.App.Version
	info["git commit"] = gitCommit
	return "debug", info, nil
}

// top-level "host" info
func hostInfo(c *cli.Context) (string, map[string]interface{}, error) {
	// lets say OS, arch, number of cpus, amount of memory, maybe os distribution/version, hostname, kernel version, uptime
	info := map[string]interface{}{}
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["cpus"] = runtime.NumCPU()
	mi, err := system.ReadMemInfo()
	if err != nil {
		info["meminfo"] = infoErr(err)
	} else {
		// TODO this might be a place for github.com/dustin/go-humanize
		info["MemTotal"] = mi.MemTotal
		info["MemFree"] = mi.MemFree
		info["SwapTotal"] = mi.SwapTotal
		info["SwapFree"] = mi.SwapFree
	}
	if kv, err := readKernelVersion(); err != nil {
		info["kernel"] = infoErr(err)
	} else {
		info["kernel"] = kv
	}

	if up, err := readUptime(); err != nil {
		info["uptime"] = infoErr(err)
	} else {
		info["uptime"] = up
	}
	if host, err := os.Hostname(); err != nil {
		info["hostname"] = infoErr(err)
	} else {
		info["hostname"] = host
	}
	return "host", info, nil
}

// top-level "store" info
func storeInfo(c *cli.Context) (string, map[string]interface{}, error) {
	storeStr := "store"
	config, err := getConfig(c)
	if err != nil {
		return storeStr, nil, errors.Wrapf(err, "Could not get config")
	}
	store, err := getStore(config)
	if err != nil {
		return storeStr, nil, err
	}

	// lets say storage driver in use, number of images, number of containers
	info := map[string]interface{}{}
	info["GraphRoot"] = store.GraphRoot()
	info["RunRoot"] = store.RunRoot()
	info["GraphDriverName"] = store.GraphDriverName()
	info["GraphOptions"] = store.GraphOptions()
	statusPairs, err := store.Status()
	if err != nil {
		return storeStr, nil, err
	}
	status := map[string]string{}
	for _, pair := range statusPairs {
		status[pair[0]] = pair[1]
	}
	info["GraphStatus"] = status
	images, err := store.Images()
	if err != nil {
		info["ImageStore"] = infoErr(err)
	} else {
		info["ImageStore"] = map[string]interface{}{
			"number": len(images),
		}
	}
	containers, err := store.Containers()
	if err != nil {
		info["ContainerStore"] = infoErr(err)
	} else {
		info["ContainerStore"] = map[string]interface{}{
			"number": len(containers),
		}
	}
	return storeStr, info, nil
}

func readKernelVersion() (string, error) {
	buf, err := ioutil.ReadFile("/proc/version")
	if err != nil {
		return "", err
	}
	f := bytes.Fields(buf)
	if len(f) < 2 {
		return string(bytes.TrimSpace(buf)), nil
	}
	return string(f[2]), nil
}

func readUptime() (string, error) {
	buf, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return "", err
	}
	f := bytes.Fields(buf)
	if len(f) < 1 {
		return "", fmt.Errorf("invalid uptime")
	}
	return string(f[0]), nil
}
