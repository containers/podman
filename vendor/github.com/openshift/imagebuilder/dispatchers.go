package imagebuilder

// This file contains the dispatchers for each command. Note that
// `nullDispatch` is not actually a command, but support for commands we parse
// but do nothing with.
//
// See evaluator.go for a higher level discussion of the whole evaluator
// package.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/containerd/containerd/platforms"
	"github.com/containers/storage/pkg/regexp"
	"github.com/openshift/imagebuilder/signal"
	"github.com/openshift/imagebuilder/strslice"
)

var (
	obRgex = regexp.Delayed(`(?i)^\s*ONBUILD\s*`)
)

var localspec = platforms.DefaultSpec()

// https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
var builtinBuildArgs = map[string]string{
	"TARGETPLATFORM": localspec.OS + "/" + localspec.Architecture,
	"TARGETOS":       localspec.OS,
	"TARGETARCH":     localspec.Architecture,
	"TARGETVARIANT":  localspec.Variant,
	"BUILDPLATFORM":  localspec.OS + "/" + localspec.Architecture,
	"BUILDOS":        localspec.OS,
	"BUILDARCH":      localspec.Architecture,
	"BUILDVARIANT":   localspec.Variant,
}

func init() {
	if localspec.Variant != "" {
		builtinBuildArgs["TARGETPLATFORM"] = builtinBuildArgs["TARGETPLATFORM"] + "/" + localspec.Variant
		builtinBuildArgs["BUILDPLATFORM"] = builtinBuildArgs["BUILDPLATFORM"] + "/" + localspec.Variant
	}
}

// ENV foo bar
//
// Sets the environment variable foo to bar, also makes interpolation
// in the dockerfile available from the next statement on via ${foo}.
func env(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("ENV")
	}

	if len(args)%2 != 0 {
		// should never get here, but just in case
		return errTooManyArguments("ENV")
	}

	// TODO/FIXME/NOT USED
	// Just here to show how to use the builder flags stuff within the
	// context of a builder command. Will remove once we actually add
	// a builder command to something!
	/*
		flBool1 := b.flags.AddBool("bool1", false)
		flStr1 := b.flags.AddString("str1", "HI")

		if err := b.flags.Parse(); err != nil {
			return err
		}

		fmt.Printf("Bool1:%v\n", flBool1)
		fmt.Printf("Str1:%v\n", flStr1)
	*/

	for j := 0; j < len(args); j++ {
		// name  ==> args[j]
		// value ==> args[j+1]
		newVar := []string{args[j] + "=" + args[j+1]}
		b.RunConfig.Env = mergeEnv(b.RunConfig.Env, newVar)
		b.Env = mergeEnv(b.Env, newVar)
		j++
	}

	return nil
}

// MAINTAINER some text <maybe@an.email.address>
//
// Sets the maintainer metadata.
func maintainer(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("MAINTAINER")
	}
	b.Author = args[0]
	return nil
}

// LABEL some json data describing the image
//
// Sets the Label variable foo to bar,
func label(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("LABEL")
	}
	if len(args)%2 != 0 {
		// should never get here, but just in case
		return errTooManyArguments("LABEL")
	}

	if b.RunConfig.Labels == nil {
		b.RunConfig.Labels = map[string]string{}
	}

	for j := 0; j < len(args); j++ {
		// name  ==> args[j]
		// value ==> args[j+1]
		b.RunConfig.Labels[args[j]] = args[j+1]
		j++
	}
	return nil
}

// ADD foo /path
//
// Add the file 'foo' to '/path'. Tarball and Remote URL (git, http) handling
// exist here. If you do not wish to have this automatic handling, use COPY.
func add(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) < 2 {
		return errAtLeastTwoArgument("ADD")
	}
	var chown string
	var chmod string
	last := len(args) - 1
	dest := makeAbsolute(args[last], b.RunConfig.WorkingDir)
	filteredUserArgs := make(map[string]string)
	for k, v := range b.Args {
		if _, ok := b.AllowedArgs[k]; ok {
			filteredUserArgs[k] = v
		}
	}
	userArgs := mergeEnv(envMapAsSlice(filteredUserArgs), b.Env)
	for _, a := range flagArgs {
		arg, err := ProcessWord(a, userArgs)
		if err != nil {
			return err
		}
		switch {
		case strings.HasPrefix(arg, "--chown="):
			chown = strings.TrimPrefix(arg, "--chown=")
		case strings.HasPrefix(arg, "--chmod="):
			chmod = strings.TrimPrefix(arg, "--chmod=")
			err = checkChmodConversion(chmod)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("ADD only supports the --chmod=<permissions> and the --chown=<uid:gid> flag")
		}
	}
	b.PendingCopies = append(b.PendingCopies, Copy{Src: args[0:last], Dest: dest, Download: true, Chown: chown, Chmod: chmod})
	return nil
}

// COPY foo /path
//
// Same as 'ADD' but without the tar and remote url handling.
func dispatchCopy(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) < 2 {
		return errAtLeastTwoArgument("COPY")
	}
	last := len(args) - 1
	dest := makeAbsolute(args[last], b.RunConfig.WorkingDir)
	var chown string
	var chmod string
	var from string
	userArgs := mergeEnv(envMapAsSlice(b.Args), b.Env)
	for _, a := range flagArgs {
		arg, err := ProcessWord(a, userArgs)
		if err != nil {
			return err
		}
		switch {
		case strings.HasPrefix(arg, "--chown="):
			chown = strings.TrimPrefix(arg, "--chown=")
		case strings.HasPrefix(arg, "--chmod="):
			chmod = strings.TrimPrefix(arg, "--chmod=")
			err = checkChmodConversion(chmod)
			if err != nil {
				return err
			}
		case strings.HasPrefix(arg, "--from="):
			from = strings.TrimPrefix(arg, "--from=")
		default:
			return fmt.Errorf("COPY only supports the --chmod=<permissions> --chown=<uid:gid> and the --from=<image|stage> flags")
		}
	}
	b.PendingCopies = append(b.PendingCopies, Copy{From: from, Src: args[0:last], Dest: dest, Download: false, Chown: chown, Chmod: chmod})
	return nil
}

// FROM imagename
//
// This sets the image the dockerfile will build on top of.
func from(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	switch {
	case len(args) == 1:
	case len(args) == 3 && len(args[0]) > 0 && strings.EqualFold(args[1], "as") && len(args[2]) > 0:

	default:
		return fmt.Errorf("FROM requires either one argument, or three: FROM <source> [as <name>]")
	}

	name := args[0]

	// Support ARG before from
	argStrs := []string{}
	for n, v := range b.HeadingArgs {
		argStrs = append(argStrs, n+"="+v)
	}
	defaultArgs := envMapAsSlice(builtinBuildArgs)
	filteredUserArgs := make(map[string]string)
	for k, v := range b.UserArgs {
		for _, a := range b.GlobalAllowedArgs {
			if a == k {
				filteredUserArgs[k] = v
			}
		}
	}
	userArgs := mergeEnv(envMapAsSlice(filteredUserArgs), b.Env)
	userArgs = mergeEnv(defaultArgs, userArgs)
	nameArgs := mergeEnv(argStrs, userArgs)
	var err error
	if name, err = ProcessWord(name, nameArgs); err != nil {
		return err
	}

	// Windows cannot support a container with no base image.
	if name == NoBaseImageSpecifier {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("Windows does not support FROM scratch")
		}
	}
	for _, a := range flagArgs {
		arg, err := ProcessWord(a, userArgs)
		if err != nil {
			return err
		}
		switch {
		case strings.HasPrefix(arg, "--platform="):
			platformString := strings.TrimPrefix(arg, "--platform=")
			b.Platform = platformString
		default:
			return fmt.Errorf("FROM only supports the --platform flag")
		}
	}
	b.RunConfig.Image = name
	// TODO: handle onbuild
	return nil
}

// ONBUILD RUN echo yo
//
// ONBUILD triggers run when the image is used in a FROM statement.
//
// ONBUILD handling has a lot of special-case functionality, the heading in
// evaluator.go and comments around dispatch() in the same file explain the
// special cases. search for 'OnBuild' in internals.go for additional special
// cases.
func onbuild(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("ONBUILD")
	}

	triggerInstruction := strings.ToUpper(strings.TrimSpace(args[0]))
	switch triggerInstruction {
	case "ONBUILD":
		return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
	case "MAINTAINER", "FROM":
		return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", triggerInstruction)
	}

	original = obRgex.ReplaceAllString(original, "")

	b.RunConfig.OnBuild = append(b.RunConfig.OnBuild, original)
	return nil
}

// WORKDIR /tmp
//
// Set the working directory for future RUN/CMD/etc statements.
func workdir(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("WORKDIR")
	}

	// This is from the Dockerfile and will not necessarily be in platform
	// specific semantics, hence ensure it is converted.
	workdir := filepath.FromSlash(args[0])

	if !filepath.IsAbs(workdir) {
		current := filepath.FromSlash(b.RunConfig.WorkingDir)
		workdir = filepath.Join(string(os.PathSeparator), current, workdir)
	}

	b.RunConfig.WorkingDir = workdir
	return nil
}

// RUN some command yo
//
// run a command and commit the image. Args are automatically prepended with
// 'sh -c' under linux or 'cmd /S /C' under Windows, in the event there is
// only one argument. The difference in processing:
//
// RUN echo hi          # sh -c echo hi       (Linux)
// RUN echo hi          # cmd /S /C echo hi   (Windows)
// RUN [ "echo", "hi" ] # echo hi
func run(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if b.RunConfig.Image == "" {
		return fmt.Errorf("Please provide a source image with `from` prior to run")
	}

	args = handleJSONArgs(args, attributes)

	var mounts []string
	var network string
	filteredUserArgs := make(map[string]string)
	for k, v := range b.Args {
		if _, ok := b.AllowedArgs[k]; ok {
			filteredUserArgs[k] = v
		}
	}
	userArgs := mergeEnv(envMapAsSlice(filteredUserArgs), b.Env)
	for _, a := range flagArgs {
		arg, err := ProcessWord(a, userArgs)
		if err != nil {
			return err
		}
		switch {
		case strings.HasPrefix(arg, "--mount="):
			mount := strings.TrimPrefix(arg, "--mount=")
			mounts = append(mounts, mount)
		case strings.HasPrefix(arg, "--network="):
			network = strings.TrimPrefix(arg, "--network=")
		default:
			return fmt.Errorf("RUN only supports the --mount and --network flag")
		}
	}

	run := Run{
		Args:    args,
		Mounts:  mounts,
		Network: network,
	}

	if !attributes["json"] {
		run.Shell = true
	}
	b.PendingRuns = append(b.PendingRuns, run)
	return nil
}

// CMD foo
//
// Set the default command to run in the container (which may be empty).
// Argument handling is the same as RUN.
func cmd(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	cmdSlice := handleJSONArgs(args, attributes)

	if !attributes["json"] {
		if runtime.GOOS != "windows" {
			cmdSlice = append([]string{"/bin/sh", "-c"}, cmdSlice...)
		} else {
			cmdSlice = append([]string{"cmd", "/S", "/C"}, cmdSlice...)
		}
	}

	b.RunConfig.Cmd = strslice.StrSlice(cmdSlice)
	if len(args) != 0 {
		b.CmdSet = true
	}
	return nil
}

// ENTRYPOINT /usr/sbin/nginx
//
// Set the entrypoint (which defaults to sh -c on linux, or cmd /S /C on Windows) to
// /usr/sbin/nginx. Will accept the CMD as the arguments to /usr/sbin/nginx.
//
// Handles command processing similar to CMD and RUN, only b.RunConfig.Entrypoint
// is initialized at NewBuilder time instead of through argument parsing.
func entrypoint(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	parsed := handleJSONArgs(args, attributes)

	switch {
	case attributes["json"]:
		// ENTRYPOINT ["echo", "hi"]
		b.RunConfig.Entrypoint = strslice.StrSlice(parsed)
	case len(parsed) == 0:
		// ENTRYPOINT []
		b.RunConfig.Entrypoint = nil
	default:
		// ENTRYPOINT echo hi
		if runtime.GOOS != "windows" {
			b.RunConfig.Entrypoint = strslice.StrSlice{"/bin/sh", "-c", parsed[0]}
		} else {
			b.RunConfig.Entrypoint = strslice.StrSlice{"cmd", "/S", "/C", parsed[0]}
		}
	}

	// when setting the entrypoint if a CMD was not explicitly set then
	// set the command to nil
	if !b.CmdSet {
		b.RunConfig.Cmd = nil
	}
	return nil
}

// EXPOSE 6667/tcp 7000/tcp
//
// Expose ports for links and port mappings. This all ends up in
// b.RunConfig.ExposedPorts for runconfig.
func expose(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("EXPOSE")
	}

	if b.RunConfig.ExposedPorts == nil {
		b.RunConfig.ExposedPorts = make(map[docker.Port]struct{})
	}

	existing := map[string]struct{}{}
	for k := range b.RunConfig.ExposedPorts {
		existing[k.Port()] = struct{}{}
	}

	for _, port := range args {
		dp := docker.Port(port)
		if _, exists := existing[dp.Port()]; !exists {
			b.RunConfig.ExposedPorts[docker.Port(fmt.Sprintf("%s/%s", dp.Port(), dp.Proto()))] = struct{}{}
		}
	}
	return nil
}

// USER foo
//
// Set the user to 'foo' for future commands and when running the
// ENTRYPOINT/CMD at container run time.
func user(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("USER")
	}

	b.RunConfig.User = args[0]
	return nil
}

// VOLUME /foo
//
// Expose the volume /foo for use. Will also accept the JSON array form.
func volume(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("VOLUME")
	}

	if b.RunConfig.Volumes == nil {
		b.RunConfig.Volumes = map[string]struct{}{}
	}
	for _, v := range args {
		v = strings.TrimSpace(v)
		if v == "" {
			return fmt.Errorf("Volume specified can not be an empty string")
		}
		b.RunConfig.Volumes[v] = struct{}{}
		b.PendingVolumes.Add(v)
	}
	return nil
}

// STOPSIGNAL signal
//
// Set the signal that will be used to kill the container.
func stopSignal(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) != 1 {
		return errExactlyOneArgument("STOPSIGNAL")
	}

	sig := args[0]
	if err := signal.CheckSignal(sig); err != nil {
		return err
	}

	b.RunConfig.StopSignal = sig
	return nil
}

// HEALTHCHECK foo
//
// Set the default healthcheck command to run in the container (which may be empty).
// Argument handling is the same as RUN.
func healthcheck(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	if len(args) == 0 {
		return errAtLeastOneArgument("HEALTHCHECK")
	}
	typ := strings.ToUpper(args[0])
	args = args[1:]
	if typ == "NONE" {
		if len(args) != 0 {
			return fmt.Errorf("HEALTHCHECK NONE takes no arguments")
		}
		test := strslice.StrSlice{typ}
		b.RunConfig.Healthcheck = &docker.HealthConfig{
			Test: test,
		}
	} else {
		if b.RunConfig.Healthcheck != nil {
			oldCmd := b.RunConfig.Healthcheck.Test
			if len(oldCmd) > 0 && oldCmd[0] != "NONE" {
				b.Warnings = append(b.Warnings, fmt.Sprintf("Note: overriding previous HEALTHCHECK: %v\n", oldCmd))
			}
		}

		healthcheck := docker.HealthConfig{}

		flags := flag.NewFlagSet("", flag.ContinueOnError)
		flags.String("start-period", "", "")
		flags.String("interval", "", "")
		flags.String("timeout", "", "")
		flRetries := flags.String("retries", "", "")

		if err := flags.Parse(flagArgs); err != nil {
			return err
		}

		switch typ {
		case "CMD":
			cmdSlice := handleJSONArgs(args, attributes)
			if len(cmdSlice) == 0 {
				return fmt.Errorf("Missing command after HEALTHCHECK CMD")
			}

			if !attributes["json"] {
				typ = "CMD-SHELL"
			}

			healthcheck.Test = strslice.StrSlice(append([]string{typ}, cmdSlice...))
		default:
			return fmt.Errorf("Unknown type %#v in HEALTHCHECK (try CMD)", typ)
		}

		period, err := parseOptInterval(flags.Lookup("start-period"))
		if err != nil {
			return err
		}
		healthcheck.StartPeriod = period

		interval, err := parseOptInterval(flags.Lookup("interval"))
		if err != nil {
			return err
		}
		healthcheck.Interval = interval

		timeout, err := parseOptInterval(flags.Lookup("timeout"))
		if err != nil {
			return err
		}
		healthcheck.Timeout = timeout

		if *flRetries != "" {
			retries, err := strconv.ParseInt(*flRetries, 10, 32)
			if err != nil {
				return err
			}
			if retries < 1 {
				return fmt.Errorf("--retries must be at least 1 (not %d)", retries)
			}
			healthcheck.Retries = int(retries)
		} else {
			healthcheck.Retries = 0
		}
		b.RunConfig.Healthcheck = &healthcheck
	}

	return nil
}

var targetArgs = []string{"TARGETOS", "TARGETARCH", "TARGETVARIANT"}

// ARG name[=value]
//
// Adds the variable foo to the trusted list of variables that can be passed
// to builder using the --build-arg flag for expansion/subsitution or passing to 'run'.
// Dockerfile author may optionally set a default value of this variable.
func arg(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	var (
		name       string
		value      string
		hasDefault bool
	)

	for _, argument := range args {
		arg := argument
		// 'arg' can just be a name or name-value pair. Note that this is different
		// from 'env' that handles the split of name and value at the parser level.
		// The reason for doing it differently for 'arg' is that we support just
		// defining an arg and not assign it a value (while 'env' always expects a
		// name-value pair). If possible, it will be good to harmonize the two.
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			name = parts[0]
			value = parts[1]
			hasDefault = true
			if name == "TARGETPLATFORM" {
				p, err := platforms.Parse(value)
				if err != nil {
					return fmt.Errorf("error parsing TARGETPLATFORM argument")
				}
				for _, val := range targetArgs {
					b.AllowedArgs[val] = true
				}
				b.Args["TARGETPLATFORM"] = p.OS + "/" + p.Architecture
				b.Args["TARGETOS"] = p.OS
				b.Args["TARGETARCH"] = p.Architecture
				b.Args["TARGETVARIANT"] = p.Variant
				if p.Variant != "" {
					b.Args["TARGETPLATFORM"] = b.Args["TARGETPLATFORM"] + "/" + p.Variant
				}
			}
		} else if val, ok := builtinBuildArgs[arg]; ok {
			name = arg
			value = val
			hasDefault = true
		} else {
			name = arg
			hasDefault = false
		}
		// add the arg to allowed list of build-time args from this step on.
		b.AllowedArgs[name] = true

		// If there is still no default value, a value can be assigned from the heading args
		if val, ok := b.HeadingArgs[name]; ok && !hasDefault {
			b.Args[name] = val
		}

		// If there is a default value associated with this arg then add it to the
		// b.buildArgs, later default values for the same arg override earlier ones.
		// The args passed to builder (UserArgs) override the default value of 'arg'
		// Don't add them here as they were already set in NewBuilder.
		if _, ok := b.UserArgs[name]; !ok && hasDefault {
			b.Args[name] = value
		}
	}

	return nil
}

// SHELL powershell -command
//
// Set the non-default shell to use.
func shell(b *Builder, args []string, attributes map[string]bool, flagArgs []string, original string) error {
	shellSlice := handleJSONArgs(args, attributes)
	switch {
	case len(shellSlice) == 0:
		// SHELL []
		return errAtLeastOneArgument("SHELL")
	case attributes["json"]:
		// SHELL ["powershell", "-command"]
		b.RunConfig.Shell = strslice.StrSlice(shellSlice)
		// b.RunConfig.Shell = strslice.StrSlice(shellSlice)
	default:
		// SHELL powershell -command - not JSON
		return errNotJSON("SHELL")
	}
	return nil
}

func checkChmodConversion(chmod string) error {
	_, err := strconv.ParseUint(chmod, 8, 32)
	if err != nil {
		return fmt.Errorf("Error parsing chmod %s", chmod)
	}
	return nil
}

func errAtLeastOneArgument(command string) error {
	return fmt.Errorf("%s requires at least one argument", command)
}

func errAtLeastTwoArgument(command string) error {
	return fmt.Errorf("%s requires at least two arguments", command)
}

func errExactlyOneArgument(command string) error {
	return fmt.Errorf("%s requires exactly one argument", command)
}

func errTooManyArguments(command string) error {
	return fmt.Errorf("Bad input to %s, too many arguments", command)
}

func errNotJSON(command string) error {
	return fmt.Errorf("%s requires the arguments to be in JSON form", command)
}
