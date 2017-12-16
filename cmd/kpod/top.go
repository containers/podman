package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectatomic/libpod/cmd/kpod/formats"
	"github.com/projectatomic/libpod/libpod"
	"github.com/urfave/cli"
)

var (
	topFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "format",
			Usage: "Change the output to JSON",
		},
	}
	topDescription = `
   kpod top

	Display the running processes of the container.
`

	topCommand = cli.Command{
		Name:           "top",
		Usage:          "Display the running processes of a container",
		Description:    topDescription,
		Flags:          topFlags,
		Action:         topCmd,
		ArgsUsage:      "CONTAINER-NAME",
		SkipArgReorder: true,
	}
)

func topCmd(c *cli.Context) error {
	doJSON := false
	if c.IsSet("format") {
		if strings.ToUpper(c.String("format")) == "JSON" {
			doJSON = true
		} else {
			return errors.Errorf("only 'json' is supported for a format option")
		}
	}
	args := c.Args()
	var psArgs []string
	psOpts := []string{"-o", "uid,pid,ppid,c,stime,tname,time,cmd"}
	if len(args) < 1 {
		return errors.Errorf("you must provide the name or id of a running container")
	}
	if err := validateFlags(c, topFlags); err != nil {
		return err
	}

	runtime, err := getRuntime(c)
	if err != nil {
		return errors.Wrapf(err, "error creating libpod runtime")
	}
	defer runtime.Shutdown(false)
	if len(args) > 1 {
		psOpts = args[1:]
	}

	container, err := runtime.LookupContainer(args[0])
	if err != nil {
		return errors.Wrapf(err, "unable to lookup %s", args[0])
	}
	conStat, err := container.State()
	if err != nil {
		return errors.Wrapf(err, "unable to look up state for %s", args[0])
	}
	if conStat != libpod.ContainerStateRunning {
		return errors.Errorf("top can only be used on running containers")
	}

	psArgs = append(psArgs, psOpts...)

	results, err := container.GetContainerPidInformation(psArgs)
	if err != nil {
		return err
	}
	headers := getHeaders(results[0])
	format := genTopFormat(headers)
	var out formats.Writer
	psParams, err := psDataToPSParams(results[1:], headers)
	if err != nil {
		return errors.Wrap(err, "unable to convert ps data to proper structure")
	}
	if doJSON {
		out = formats.JSONStructArray{Output: topToGeneric(psParams)}
	} else {
		out = formats.StdoutTemplateArray{Output: topToGeneric(psParams), Template: format, Fields: createTopHeaderMap(headers)}
	}
	formats.Writer(out).Out()
	return nil
}

func getHeaders(s string) []string {
	var headers []string
	tmpHeaders := strings.Fields(s)
	for _, header := range tmpHeaders {
		headers = append(headers, strings.Replace(header, "%", "", -1))
	}
	return headers
}

func genTopFormat(headers []string) string {
	format := "table "
	for _, header := range headers {
		format = fmt.Sprintf("%s{{.%s}}\t", format, header)
	}
	return format
}

// imagesToGeneric creates an empty array of interfaces for output
func topToGeneric(templParams []PSParams) (genericParams []interface{}) {
	for _, v := range templParams {
		genericParams = append(genericParams, interface{}(v))
	}
	return
}

// generate the header based on the template provided
func createTopHeaderMap(v []string) map[string]string {
	values := make(map[string]string)
	for _, key := range v {
		value := key
		if value == "CPU" {
			value = "%CPU"
		} else if value == "MEM" {
			value = "%MEM"
		}
		values[key] = strings.ToUpper(splitCamelCase(value))
	}
	return values
}

// PSDataToParams converts a string array of data and its headers to an
// arra if PSParams
func psDataToPSParams(data []string, headers []string) ([]PSParams, error) {
	var params []PSParams
	for _, line := range data {
		tmpMap := make(map[string]string)
		tmpArray := strings.Fields(line)
		if len(tmpArray) == 0 {
			continue
		}
		for index, v := range tmpArray {
			header := headers[index]
			tmpMap[header] = v
		}
		jsonData, _ := json.Marshal(tmpMap)
		var r PSParams
		err := json.Unmarshal(jsonData, &r)
		if err != nil {
			return []PSParams{}, err
		}
		params = append(params, r)
	}
	return params, nil
}

//PSParams is a list of options that the command line ps recognizes
type PSParams struct {
	CPU     string
	MEM     string
	COMMAND string
	BLOCKED string
	START   string
	TIME    string
	C       string
	CAUGHT  string
	CGROUP  string
	CLSCLS  string
	CLS     string
	CMD     string
	CP      string
	DRS     string
	EGID    string
	EGROUP  string
	EIP     string
	ESP     string
	ELAPSED string
	EUIDE   string
	USER    string
	F       string
	FGID    string
	FGROUP  string
	FUID    string
	FUSER   string
	GID     string
	GROUP   string
	IGNORED string
	IPCNS   string
	LABEL   string
	STARTED string
	SESSION string
	LWP     string
	MACHINE string
	MAJFLT  string
	MINFLT  string
	MNTNS   string
	NETNS   string
	NI      string
	NLWP    string
	OWNER   string
	PENDING string
	PGID    string
	PGRP    string
	PID     string
	PIDNS   string
	POL     string
	PPID    string
	PRI     string
	PSR     string
	RGID    string
	RGROUP  string
	RSS     string
	RSZ     string
	RTPRIO  string
	RUID    string
	RUSER   string
	S       string
	SCH     string
	SEAT    string
	SESS    string
	P       string
	SGID    string
	SGROUP  string
	SID     string
	SIZE    string
	SLICE   string
	SPID    string
	STACKP  string
	STIME   string
	SUID    string
	SUPGID  string
	SUPGRP  string
	SUSER   string
	SVGID   string
	SZ      string
	TGID    string
	THCNT   string
	TID     string
	TTY     string
	TPGID   string
	TRS     string
	TT      string
	UID     string
	UNIT    string
	USERNS  string
	UTSNS   string
	UUNIT   string
	VSZ     string
	WCHAN   string
}
