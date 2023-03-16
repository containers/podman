package imagebuilder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/openshift/imagebuilder/dockerfile/command"
	"github.com/openshift/imagebuilder/dockerfile/parser"
)

// Copy defines a copy operation required on the container.
type Copy struct {
	// If true, this is a copy from the file system to the container. If false,
	// the copy is from the context.
	FromFS bool
	// If set, this is a copy from the named stage or image to the container.
	From     string
	Src      []string
	Dest     string
	Download bool
	// If set, the owner:group for the destination.  This value is passed
	// to the executor for handling.
	Chown string
	Chmod string
}

// Run defines a run operation required in the container.
type Run struct {
	Shell bool
	Args  []string
	// Mounts are mounts specified through the --mount flag inside the Containerfile
	Mounts []string
	// Network specifies the network mode to run the container with
	Network string
}

type Executor interface {
	Preserve(path string) error
	// EnsureContainerPath should ensure that the directory exists, creating any components required
	EnsureContainerPath(path string) error
	// EnsureContainerPathAs should ensure that the directory exists, creating any components required
	// with the specified owner and mode, if either is specified
	EnsureContainerPathAs(path, user string, mode *os.FileMode) error
	Copy(excludes []string, copies ...Copy) error
	Run(run Run, config docker.Config) error
	UnrecognizedInstruction(step *Step) error
}

type logExecutor struct{}

func (logExecutor) Preserve(path string) error {
	log.Printf("PRESERVE %s", path)
	return nil
}

func (logExecutor) EnsureContainerPath(path string) error {
	log.Printf("ENSURE %s", path)
	return nil
}

func (logExecutor) EnsureContainerPathAs(path, user string, mode *os.FileMode) error {
	if mode != nil {
		log.Printf("ENSURE %s AS %q with MODE=%q", path, user, *mode)
	} else {
		log.Printf("ENSURE %s AS %q", path, user)
	}
	return nil
}

func (logExecutor) Copy(excludes []string, copies ...Copy) error {
	for _, c := range copies {
		log.Printf("COPY %v -> %s (from:%s download:%t), chown: %s, chmod %s", c.Src, c.Dest, c.From, c.Download, c.Chown, c.Chmod)
	}
	return nil
}

func (logExecutor) Run(run Run, config docker.Config) error {
	log.Printf("RUN %v %v %t (%v)", run.Args, run.Mounts, run.Shell, config.Env)
	return nil
}

func (logExecutor) UnrecognizedInstruction(step *Step) error {
	log.Printf("Unknown instruction: %s", strings.ToUpper(step.Command))
	return nil
}

type noopExecutor struct{}

func (noopExecutor) Preserve(path string) error {
	return nil
}

func (noopExecutor) EnsureContainerPath(path string) error {
	return nil
}

func (noopExecutor) EnsureContainerPathAs(path, user string, mode *os.FileMode) error {
	return nil
}

func (noopExecutor) Copy(excludes []string, copies ...Copy) error {
	return nil
}

func (noopExecutor) Run(run Run, config docker.Config) error {
	return nil
}

func (noopExecutor) UnrecognizedInstruction(step *Step) error {
	return nil
}

type VolumeSet []string

func (s *VolumeSet) Add(path string) bool {
	if path == "/" {
		set := len(*s) != 1 || (*s)[0] != ""
		*s = []string{""}
		return set
	}
	path = strings.TrimSuffix(path, "/")
	var adjusted []string
	for _, p := range *s {
		if p == path || strings.HasPrefix(path, p+"/") {
			return false
		}
		if strings.HasPrefix(p, path+"/") {
			continue
		}
		adjusted = append(adjusted, p)
	}
	adjusted = append(adjusted, path)
	*s = adjusted
	return true
}

func (s VolumeSet) Has(path string) bool {
	if path == "/" {
		return len(s) == 1 && s[0] == ""
	}
	path = strings.TrimSuffix(path, "/")
	for _, p := range s {
		if p == path {
			return true
		}
	}
	return false
}

func (s VolumeSet) Covers(path string) bool {
	if path == "/" {
		return len(s) == 1 && s[0] == ""
	}
	path = strings.TrimSuffix(path, "/")
	for _, p := range s {
		if p == path || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

var (
	LogExecutor  = logExecutor{}
	NoopExecutor = noopExecutor{}
)

type Stages []Stage

func (stages Stages) ByName(name string) (Stage, bool) {
	for _, stage := range stages {
		if stage.Name == name {
			return stage, true
		}
	}
	return Stage{}, false
}

// Get just the target stage.
func (stages Stages) ByTarget(target string) (Stages, bool) {
	if len(target) == 0 {
		return stages, true
	}
	for i, stage := range stages {
		if stage.Name == target {
			return stages[i : i+1], true
		}
	}
	return nil, false
}

// Get all the stages up to and including the target.
func (stages Stages) ThroughTarget(target string) (Stages, bool) {
	if len(target) == 0 {
		return stages, true
	}
	for i, stage := range stages {
		if stage.Name == target {
			return stages[0 : i+1], true
		}
	}
	return nil, false
}

type Stage struct {
	Position int
	Name     string
	Builder  *Builder
	Node     *parser.Node
}

func NewStages(node *parser.Node, b *Builder) (Stages, error) {
	var stages Stages
	var allDeclaredArgs []string
	for _, root := range SplitBy(node, command.Arg) {
		argNode := root.Children[0]
		if argNode.Value == command.Arg {
			// extract declared variable
			s := strings.SplitN(argNode.Original, " ", 2)
			if len(s) == 2 && (strings.ToLower(s[0]) == command.Arg) {
				allDeclaredArgs = append(allDeclaredArgs, s[1])
			}
		}
	}
	if err := b.extractHeadingArgsFromNode(node); err != nil {
		return stages, err
	}
	for i, root := range SplitBy(node, command.From) {
		name, _ := extractNameFromNode(root.Children[0])
		if len(name) == 0 {
			name = strconv.Itoa(i)
		}
		stages = append(stages, Stage{
			Position: i,
			Name:     name,
			Builder:  b.builderForStage(allDeclaredArgs),
			Node:     root,
		})
	}
	return stages, nil
}

func (b *Builder) extractHeadingArgsFromNode(node *parser.Node) error {
	var args []*parser.Node
	var children []*parser.Node
	extract := true
	for _, child := range node.Children {
		if extract && child.Value == command.Arg {
			args = append(args, child)
		} else {
			if child.Value == command.From {
				extract = false
			}
			children = append(children, child)
		}
	}

	// Set children equal to everything except the leading ARG nodes
	node.Children = children

	// Use a separate builder to evaluate the heading args
	tempBuilder := NewBuilder(b.UserArgs)

	// Evaluate all the heading arg commands
	for _, c := range args {
		step := tempBuilder.Step()
		if err := step.Resolve(c); err != nil {
			return err
		}
		if err := tempBuilder.Run(step, NoopExecutor, false); err != nil {
			return err
		}
	}

	// Add all of the defined heading args to the original builder's HeadingArgs map
	for k, v := range tempBuilder.Args {
		if _, ok := tempBuilder.AllowedArgs[k]; ok {
			b.HeadingArgs[k] = v
		}
	}

	return nil
}

func extractNameFromNode(node *parser.Node) (string, bool) {
	if node.Value != command.From {
		return "", false
	}
	n := node.Next
	if n == nil || n.Next == nil {
		return "", false
	}
	n = n.Next
	if !strings.EqualFold(n.Value, "as") || n.Next == nil || len(n.Next.Value) == 0 {
		return "", false
	}
	return n.Next.Value, true
}

func (b *Builder) builderForStage(globalArgsList []string) *Builder {
	stageBuilder := newBuilderWithGlobalAllowedArgs(b.UserArgs, globalArgsList)
	for k, v := range b.HeadingArgs {
		stageBuilder.HeadingArgs[k] = v
	}
	return stageBuilder
}

type Builder struct {
	RunConfig docker.Config

	Env         []string
	Args        map[string]string
	HeadingArgs map[string]string
	UserArgs    map[string]string
	CmdSet      bool
	Author      string
	// Certain instructions like `FROM` will need to use
	// `ARG` decalred before or not in this stage hence
	// while processing instruction like `FROM ${SOME_ARG}`
	// we will make sure to verify if they are declared any
	// where in containerfile or not.
	GlobalAllowedArgs []string

	AllowedArgs map[string]bool
	Volumes     VolumeSet
	Excludes    []string

	PendingVolumes VolumeSet
	PendingRuns    []Run
	PendingCopies  []Copy

	Warnings []string
	// Raw platform string specified with `FROM --platform` of the stage
	// It's up to the implementation or client to parse and use this field
	Platform string
}

func NewBuilder(args map[string]string) *Builder {
	return newBuilderWithGlobalAllowedArgs(args, []string{})
}

func newBuilderWithGlobalAllowedArgs(args map[string]string, globalallowedargs []string) *Builder {
	allowed := make(map[string]bool)
	for k, v := range builtinAllowedBuildArgs {
		allowed[k] = v
	}
	userArgs := make(map[string]string)
	initialArgs := make(map[string]string)
	for k, v := range args {
		userArgs[k] = v
		initialArgs[k] = v
	}
	return &Builder{
		Args:              initialArgs,
		UserArgs:          userArgs,
		HeadingArgs:       make(map[string]string),
		AllowedArgs:       allowed,
		GlobalAllowedArgs: globalallowedargs,
	}
}

func ParseFile(path string) (*parser.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseDockerfile(f)
}

// Step creates a new step from the current state.
func (b *Builder) Step() *Step {
	// Include build arguments in the table of variables that we'll use in
	// Resolve(), but override them with values from the actual
	// environment in case there's any conflict.
	return &Step{Env: mergeEnv(b.Arguments(), mergeEnv(b.Env, b.RunConfig.Env))}
}

// Run executes a step, transforming the current builder and
// invoking any Copy or Run operations. noRunsRemaining is an
// optimization hint that allows the builder to avoid performing
// unnecessary work.
func (b *Builder) Run(step *Step, exec Executor, noRunsRemaining bool) error {
	fn, ok := evaluateTable[step.Command]
	if !ok {
		return exec.UnrecognizedInstruction(step)
	}
	if err := fn(b, step.Args, step.Attrs, step.Flags, step.Original); err != nil {
		return err
	}

	copies := b.PendingCopies
	b.PendingCopies = nil
	runs := b.PendingRuns
	b.PendingRuns = nil

	// Once a VOLUME is defined, future ADD/COPY instructions are
	// all that may mutate that path. Instruct the executor to preserve
	// the path. The executor must handle invalidating preserved info.
	for _, path := range b.PendingVolumes {
		if b.Volumes.Add(path) && !noRunsRemaining {
			if err := exec.Preserve(path); err != nil {
				return err
			}
		}
	}

	if err := exec.Copy(b.Excludes, copies...); err != nil {
		return err
	}

	if len(b.RunConfig.WorkingDir) > 0 {
		if err := exec.EnsureContainerPathAs(b.RunConfig.WorkingDir, b.RunConfig.User, nil); err != nil {
			return err
		}
	}

	for _, run := range runs {
		config := b.Config()
		config.Env = step.Env
		if err := exec.Run(run, *config); err != nil {
			return err
		}
	}

	return nil
}

// RequiresStart returns true if a running container environment is necessary
// to invoke the provided commands
func (b *Builder) RequiresStart(node *parser.Node) bool {
	for _, child := range node.Children {
		if child.Value == command.Run {
			return true
		}
	}
	return false
}

// Config returns a snapshot of the current RunConfig intended for
// use with a container commit.
func (b *Builder) Config() *docker.Config {
	config := b.RunConfig
	if config.OnBuild == nil {
		config.OnBuild = []string{}
	}
	if config.Entrypoint == nil {
		config.Entrypoint = []string{}
	}
	config.Image = ""
	return &config
}

// Arguments returns the currently active arguments.
func (b *Builder) Arguments() []string {
	var envs []string
	for key, val := range b.Args {
		if _, ok := b.AllowedArgs[key]; ok {
			envs = append(envs, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return envs
}

// ErrNoFROM is returned if the Dockerfile did not contain a FROM
// statement.
var ErrNoFROM = fmt.Errorf("no FROM statement found")

// From returns the image this dockerfile depends on, or an error
// if no FROM is found or if multiple FROM are specified. If a
// single from is found the passed node is updated with only
// the remaining statements.  The builder's RunConfig.Image field
// is set to the first From found, or left unchanged if already
// set.
func (b *Builder) From(node *parser.Node) (string, error) {
	if err := b.extractHeadingArgsFromNode(node); err != nil {
		return "", err
	}
	children := SplitChildren(node, command.From)
	switch {
	case len(children) == 0:
		return "", ErrNoFROM
	case len(children) > 1:
		return "", fmt.Errorf("multiple FROM statements are not supported")
	default:
		step := b.Step()
		if err := step.Resolve(children[0]); err != nil {
			return "", err
		}
		if err := b.Run(step, NoopExecutor, false); err != nil {
			return "", err
		}
		return b.RunConfig.Image, nil
	}
}

// FromImage updates the builder to use the provided image (resetting RunConfig
// and recording the image environment), and updates the node with any ONBUILD
// statements extracted from the parent image.
func (b *Builder) FromImage(image *docker.Image, node *parser.Node) error {
	SplitChildren(node, command.From)

	b.RunConfig = *image.Config
	b.Env = mergeEnv(b.Env, b.RunConfig.Env)
	b.RunConfig.Env = nil

	// Check to see if we have a default PATH, note that windows won't
	// have one as it's set by HCS
	if runtime.GOOS != "windows" && !hasEnvName(b.Env, "PATH") {
		b.RunConfig.Env = append(b.RunConfig.Env, "PATH="+defaultPathEnv)
	}

	// Join the image onbuild statements into node
	if image.Config == nil || len(image.Config.OnBuild) == 0 {
		return nil
	}
	extra, err := ParseDockerfile(bytes.NewBufferString(strings.Join(image.Config.OnBuild, "\n")))
	if err != nil {
		return err
	}
	for _, child := range extra.Children {
		switch strings.ToUpper(child.Value) {
		case "ONBUILD":
			return fmt.Errorf("Chaining ONBUILD via `ONBUILD ONBUILD` isn't allowed")
		case "MAINTAINER", "FROM":
			return fmt.Errorf("%s isn't allowed as an ONBUILD trigger", child.Value)
		}
	}
	node.Children = append(extra.Children, node.Children...)
	// Since we've processed the OnBuild statements, clear them from the runconfig state.
	b.RunConfig.OnBuild = nil
	return nil
}

// SplitChildren removes any children with the provided value from node
// and returns them as an array. node.Children is updated.
func SplitChildren(node *parser.Node, value string) []*parser.Node {
	var split []*parser.Node
	var children []*parser.Node
	for _, child := range node.Children {
		if child.Value == value {
			split = append(split, child)
		} else {
			children = append(children, child)
		}
	}
	node.Children = children
	return split
}

func SplitBy(node *parser.Node, value string) []*parser.Node {
	var split []*parser.Node
	var current *parser.Node
	for _, child := range node.Children {
		if current == nil || child.Value == value {
			copied := *node
			current = &copied
			current.Children = nil
			current.Next = nil
			split = append(split, current)
		}
		current.Children = append(current.Children, child)
	}
	return split
}

// StepFunc is invoked with the result of a resolved step.
type StepFunc func(*Builder, []string, map[string]bool, []string, string) error

var evaluateTable = map[string]StepFunc{
	command.Env:         env,
	command.Label:       label,
	command.Maintainer:  maintainer,
	command.Add:         add,
	command.Copy:        dispatchCopy, // copy() is a go builtin
	command.From:        from,
	command.Onbuild:     onbuild,
	command.Workdir:     workdir,
	command.Run:         run,
	command.Cmd:         cmd,
	command.Entrypoint:  entrypoint,
	command.Expose:      expose,
	command.Volume:      volume,
	command.User:        user,
	command.StopSignal:  stopSignal,
	command.Arg:         arg,
	command.Healthcheck: healthcheck,
	command.Shell:       shell,
}

// builtinAllowedBuildArgs is list of built-in allowed build args
var builtinAllowedBuildArgs = map[string]bool{
	"HTTP_PROXY":  true,
	"http_proxy":  true,
	"HTTPS_PROXY": true,
	"https_proxy": true,
	"FTP_PROXY":   true,
	"ftp_proxy":   true,
	"NO_PROXY":    true,
	"no_proxy":    true,
}

// ParseIgnore returns a list of the excludes in the specified path
// path should be a file with the .dockerignore format
// extracted from fsouza/go-dockerclient and modified to drop comments and
// empty lines.
func ParseIgnore(path string) ([]string, error) {
	var excludes []string

	ignores, err := ioutil.ReadFile(path)
	if err != nil {
		return excludes, err
	}
	for _, ignore := range strings.Split(string(ignores), "\n") {
		if len(ignore) == 0 || ignore[0] == '#' {
			continue
		}
		ignore = strings.Trim(ignore, "/")
		if len(ignore) > 0 {
			excludes = append(excludes, ignore)
		}
	}
	return excludes, nil
}

// ParseDockerIgnore returns a list of the excludes in the .containerignore or .dockerignore file.
func ParseDockerignore(root string) ([]string, error) {
	excludes, err := ParseIgnore(filepath.Join(root, ".containerignore"))
	if err != nil && os.IsNotExist(err) {
		excludes, err = ParseIgnore(filepath.Join(root, ".dockerignore"))
	}
	if err != nil && os.IsNotExist(err) {
		return excludes, nil
	}
	return excludes, err
}

// ExportEnv creates an export statement for a shell that contains all of the
// provided environment.
func ExportEnv(env []string) string {
	if len(env) == 0 {
		return ""
	}
	out := "export"
	for _, e := range env {
		if len(e) == 0 {
			continue
		}
		out += " " + BashQuote(e)
	}
	return out + "; "
}

// BashQuote escapes the provided string and surrounds it with double quotes.
// TODO: verify that these are all we have to escape.
func BashQuote(env string) string {
	out := []rune{'"'}
	for _, r := range env {
		switch r {
		case '$', '\\', '"':
			out = append(out, '\\', r)
		default:
			out = append(out, r)
		}
	}
	out = append(out, '"')
	return string(out)
}
