package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "bytes"

    "github.com/containers/libpod/cmd/podman/libpodruntime"
    "github.com/containers/libpod/libpod"
    "github.com/containers/libpod/pkg/rootless"
    "github.com/pkg/errors"
    "github.com/containers/libpod/cmd/podman/cliconfig"
    "github.com/spf13/cobra"
)

var (
    patchCommand     cliconfig.PatchValues
    patchDescription = `Patch containers files with the given patch.`

    _patchCommand = &cobra.Command{
        Use:   "patch",
        Short: "Apply a patch to the specified containers",
        Long:  patchDescription,
        RunE: func(cmd *cobra.Command, args []string) error {
            patchCommand.InputArgs = args
            patchCommand.GlobalFlags = MainGlobalOpts
            return patchCmd(&patchCommand)
        },
        Example: "DEST_FILE SRC_PATCH_FILE [CONTAINERS]",
    }
)

func init() {
        patchCommand.Command = _patchCommand
        flags := patchCommand.Flags()
        flags.BoolVarP(
            &patchCommand.All,
            "all",
            "a",
            false,
            "Apply the patch to all the running containers")
        flags.BoolVarP(
            &patchCommand.Ignore,
            "ignore-fail",
            "i",
            false,
            "Ignore if the file to patch doesn't exist in a container")

        rootCmd.AddCommand(patchCommand.Command)
}

// Define the entrypoint command
func patchCmd(c *cliconfig.PatchValues) error {
//func patchCmd(c *cli.Context) error {
    runtime, err := libpodruntime.GetRuntime(&c.PodmanCommand)
    //runtime, err := libpodruntime.GetRuntime(c)
    if os.Geteuid() != 0 {
        rootless.SetSkipStorageSetup(true)
    }

    if os.Geteuid() != 0 {
        if driver := runtime.GetConfig().StorageConfig.GraphDriverName; driver != "vfs" {
            // Do not allow to mount a graphdriver that is not vfs if we are
            // creating the userns as part
            // of the mount command.
            fmt.Errorf("cannot mount using driver %s in rootless mode", driver)
        }

        became, ret, err := rootless.BecomeRootInUserNS()
        if err != nil {
            return err
        }
        if became {
            os.Exit(ret)
        }
    }
    args := c.InputArgs
    if len(args) < 3 {
        if c.All && len(args) < 2 {
            return errors.Errorf("too few given arguments")
        }
    }

    if err != nil {
        return errors.Wrapf(err, "could not get runtime")
    }
    defer runtime.Shutdown(false)

    dst := args[0]
    patch := args[1]

    var containers []string
    if ! c.All {
        containers = args[2:]
    } else {
        containers = retrieveExisting(runtime)
    }

    if ! canPatch() {
        fmt.Println("Patch command not on your system")
        os.Exit(1)
    }

    if ! exists(patch) {
        fmt.Println("Patch file not found:", patch)
        os.Exit(1)
    }

    return patchContainers(runtime, patch, dst, containers, c.Ignore)
}

// Retrieve all running containers and return array of name
func retrieveExisting(runtime *libpod.Runtime) ([]string) {
    runningPods, err := runtime.GetRunningContainers()
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    var containers []string

    for _, container := range runningPods {
        containers = append(containers, container.Name())
    }
    return containers
}

// Check if the patch command is available on the current system (host)
func canPatch() (bool) {
    _, err := exec.Command("which", "patch").Output()
    if err != nil {
        return false
    }
    return true
}

// Check if a file exist
func exists(path string) (bool) {
    _, err := os.Stat(path)
    if os.IsNotExist(err) {
        return false
    }
    if err == nil { return true }
    return true
}

// Retrieve all the containers by name and
// loop over to apply the given patch
func patchContainers(runtime *libpod.Runtime,
                     patch string, dst string, containers []string, ignore bool) error {
    var ctrs []*libpod.Container
    for _, container := range containers {
        ctr, err := runtime.LookupContainer(container)
        if err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
        ctrs = append(ctrs, ctr)
    }
    for _, ctr := range ctrs {
        applyPatch(patch, dst, ctr, ignore)
    }
    return nil
}

// Apply the given patch file to the given container.
// The patch is applied to the dst file.
// If the destintion doesn't exist inside the given container
// and if the flag ignore-fail is given the command
// simply continue and doesn't exit with an error code.
func applyPatch(patch string, dst string, container *libpod.Container, ignore bool) {
    fmt.Println("Patching: ", container.Name())
    mountPoint, err := container.Mount()
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }

    fulldest := filepath.Join(mountPoint, dst)

    if ! exists(fulldest) {
        fmt.Println("File to patch not found:", dst)
        if ignore {
            fmt.Println("Skipping container:", container.Name())
            return
        } else {
            os.Exit(1)
        }
    }
    fmt.Println("Execute: patch ", fulldest, patch)
    cmd := exec.Command("patch", fulldest, patch)
    var out bytes.Buffer
    var stderr bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &stderr
    errexec := cmd.Run()
    if errexec != nil {
        fmt.Println(fmt.Sprint(errexec) + ": " + stderr.String())
    }
    container.Unmount(true)
    fmt.Println(container.Name(), "patched successfully!")
}
