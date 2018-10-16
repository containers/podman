package main

import (
	"context"
	"fmt"
	"os"
	gosignal "os/signal"
	"sync"

	"github.com/containers/libpod/libpod"
	"github.com/docker/docker/pkg/signal"
	"github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"k8s.io/client-go/tools/remotecommand"
)

type RawTtyFormatter struct {
}

// Start (if required) and attach to a container
func startAttachCtr(ctr *libpod.Container, stdout, stderr, stdin *os.File, detachKeys string, sigProxy bool, startContainer bool) error {
	ctx := context.Background()
	resize := make(chan remotecommand.TerminalSize)

	haveTerminal := terminal.IsTerminal(int(os.Stdin.Fd()))

	// Check if we are attached to a terminal. If we are, generate resize
	// events, and set the terminal to raw mode
	if haveTerminal && ctr.Spec().Process.Terminal {
		logrus.Debugf("Handling terminal attach")

		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		resizeTty(subCtx, resize)

		oldTermState, err := term.SaveState(os.Stdin.Fd())
		if err != nil {
			return errors.Wrapf(err, "unable to save terminal state")
		}

		logrus.SetFormatter(&RawTtyFormatter{})
		term.SetRawTerminal(os.Stdin.Fd())

		defer restoreTerminal(oldTermState)
	}

	streams := new(libpod.AttachStreams)
	streams.OutputStream = stdout
	streams.ErrorStream = stderr
	streams.InputStream = stdin
	streams.AttachOutput = true
	streams.AttachError = true
	streams.AttachInput = true

	if stdout == nil {
		logrus.Debugf("Not attaching to stdout")
		streams.AttachOutput = false
	}
	if stderr == nil {
		logrus.Debugf("Not attaching to stderr")
		streams.AttachError = false
	}
	if stdin == nil {
		logrus.Debugf("Not attaching to stdin")
		streams.AttachInput = false
	}

	if !startContainer {
		if sigProxy {
			ProxySignals(ctr)
		}

		return ctr.Attach(streams, detachKeys, resize)
	}

	attachChan, err := ctr.StartAndAttach(getContext(), streams, detachKeys, resize)
	if err != nil {
		return err
	}

	if sigProxy {
		ProxySignals(ctr)
	}

	if stdout == nil && stderr == nil {
		fmt.Printf("%s\n", ctr.ID())
	}

	err = <-attachChan
	if err != nil {
		return errors.Wrapf(err, "error attaching to container %s", ctr.ID())
	}

	return nil
}

// getResize returns a TerminalSize command matching stdin's current
// size on success, and nil on errors.
func getResize() *remotecommand.TerminalSize {
	winsize, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		logrus.Warnf("Could not get terminal size %v", err)
		return nil
	}
	return &remotecommand.TerminalSize{
		Width:  winsize.Width,
		Height: winsize.Height,
	}
}

// Helper for prepareAttach - set up a goroutine to generate terminal resize events
func resizeTty(ctx context.Context, resize chan remotecommand.TerminalSize) {
	sigchan := make(chan os.Signal, 1)
	gosignal.Notify(sigchan, signal.SIGWINCH)
	go func() {
		defer close(resize)
		// Update the terminal size immediately without waiting
		// for a SIGWINCH to get the correct initial size.
		resizeEvent := getResize()
		for {
			if resizeEvent == nil {
				select {
				case <-ctx.Done():
					return
				case <-sigchan:
					resizeEvent = getResize()
				}
			} else {
				select {
				case <-ctx.Done():
					return
				case <-sigchan:
					resizeEvent = getResize()
				case resize <- *resizeEvent:
					resizeEvent = nil
				}
			}
		}
	}()
}

func restoreTerminal(state *term.State) error {
	logrus.SetFormatter(&logrus.TextFormatter{})
	return term.RestoreTerminal(os.Stdin.Fd(), state)
}

func (f *RawTtyFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	textFormatter := logrus.TextFormatter{}
	bytes, err := textFormatter.Format(entry)

	if err == nil {
		bytes = append(bytes, '\r')
	}

	return bytes, err
}

func checkMutuallyExclusiveFlags(c *cli.Context) error {
	if err := checkAllAndLatest(c); err != nil {
		return err
	}
	if err := validateFlags(c, startFlags); err != nil {
		return err
	}
	return nil
}

// For pod commands that have a latest and all flag, getPodsFromContext gets
// pods the user specifies. If there's an error before getting pods, the pods slice
// will be empty and error will be not nil. If an error occured after, the pod slice
// will hold all of the successful pods, and error will hold the last error.
// The remaining errors will be logged. On success, pods will hold all pods and
// error will be nil.
func getPodsFromContext(c *cli.Context, r *libpod.Runtime) ([]*libpod.Pod, error) {
	args := c.Args()
	var pods []*libpod.Pod
	var lastError error
	var err error

	if c.Bool("all") {
		pods, err = r.Pods()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get running pods")
		}
	}

	if c.Bool("latest") {
		pod, err := r.GetLatestPod()
		if err != nil {
			return nil, errors.Wrapf(err, "unable to get latest pod")
		}
		pods = append(pods, pod)
	}

	for _, i := range args {
		pod, err := r.LookupPod(i)
		if err != nil {
			if lastError != nil {
				logrus.Errorf("%q", lastError)
			}
			lastError = errors.Wrapf(err, "unable to find pod %s", i)
			continue
		}
		pods = append(pods, pod)
	}
	return pods, lastError
}

type pFunc func() error

type workerInput struct {
	containerID  string
	parallelFunc pFunc
}

type containerError struct {
	containerID string
	err         error
}

// worker is a "threaded" worker that takes jobs from the channel "queue"
func worker(wg *sync.WaitGroup, jobs <-chan workerInput, results chan<- containerError) {
	for j := range jobs {
		err := j.parallelFunc()
		results <- containerError{containerID: j.containerID, err: err}
		wg.Done()
	}
}

// parallelExecuteWorkerPool takes container jobs and performs them in parallel.  The worker
// int is determines how many workers/threads should be premade.
func parallelExecuteWorkerPool(workers int, functions []workerInput) map[string]error {
	var (
		wg sync.WaitGroup
	)

	resultChan := make(chan containerError, len(functions))
	results := make(map[string]error)
	paraJobs := make(chan workerInput, len(functions))

	// If we have more workers than functions, match up the number of workers and functions
	if workers > len(functions) {
		workers = len(functions)
	}

	// Create the workers
	for w := 1; w <= workers; w++ {
		go worker(&wg, paraJobs, resultChan)
	}

	// Add jobs to the workers
	for _, j := range functions {
		j := j
		wg.Add(1)
		paraJobs <- j
	}

	close(paraJobs)
	wg.Wait()

	close(resultChan)
	for ctrError := range resultChan {
		results[ctrError.containerID] = ctrError.err
	}

	return results
}
