# Adding a podman V2 commands

## Build podman V2

```shell script
$ cd $GOPATH/src/github.com/containers/libpod/cmd/podmanV2
```
If you wish to include the libpod library in your program,
```shell script
$ go build -tags 'ABISupport' .
```
The `--remote` flag may be used to connect to the Podman service using the API.
Otherwise, direct calls will be made to the Libpod library.
```shell script
$ go build -tags '!ABISupport' .
```
The Libpod library is not linked into the executable.
All calls are made via the API and `--remote=False` is an error condition.

## Adding a new command `podman manifests`
```shell script
$ mkdir -p $GOPATH/src/github.com/containers/libpod/cmd/podmanV2/manifests
```
Create the file ```$GOPATH/src/github.com/containers/libpod/cmd/podmanV2/manifests/manifest.go```
```go
package manifests

import (
    "github.com/containers/libpod/cmd/podmanV2/registry"
    "github.com/containers/libpod/pkg/domain/entities"
    "github.com/spf13/cobra"
)

var (
    // podman _manifests_
    manifestCmd = &cobra.Command{
        Use:               "manifest",
        Short:             "Manage manifests",
        Long:              "Manage manifests",
        Example:           "podman manifests IMAGE",
        TraverseChildren:  true,
        PersistentPreRunE: preRunE,
        RunE:              registry.SubCommandExists, // Report error if there is no sub command given
    }
)
func init() {
    // Subscribe command to podman
    registry.Commands = append(registry.Commands, registry.CliCommand{
        // _podman manifest_ will support both ABIMode and TunnelMode
        Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
        // The definition for this command
        Command: manifestCmd,
    })
    // Setup cobra templates, sub commands will inherit
    manifestCmd.SetHelpTemplate(registry.HelpTemplate())
    manifestCmd.SetUsageTemplate(registry.UsageTemplate())
}

// preRunE populates the image engine for sub commands
func preRunE(cmd *cobra.Command, args []string) error {
    _, err := registry.NewImageEngine(cmd, args)
    return err
}
```
To "wire" in the `manifest` command, edit the file ```$GOPATH/src/github.com/containers/libpod/cmd/podmanV2/main.go``` to add:
```go
package main

import	_ "github.com/containers/libpod/cmd/podmanV2/manifests"
```

## Adding a new sub command `podman manifests list`
Create the file ```$GOPATH/src/github.com/containers/libpod/cmd/podmanV2/manifests/inspect.go```
```go
package manifests

import (
    "github.com/containers/libpod/cmd/podmanV2/registry"
    "github.com/containers/libpod/pkg/domain/entities"
    "github.com/spf13/cobra"
)

var (
    // podman manifests _inspect_
    inspectCmd = &cobra.Command{
        Use:     "inspect IMAGE",
        Short:   "Display manifest from image",
        Long:    "Displays the low-level information on a manifest identified by image name or ID",
        RunE:    inspect,
        Example: "podman manifest DEADBEEF",
    }
)

func init() {
    // Subscribe inspect sub command to manifest command
    registry.Commands = append(registry.Commands, registry.CliCommand{
        // _podman manifest inspect_ will support both ABIMode and TunnelMode
        Mode:    []entities.EngineMode{entities.ABIMode, entities.TunnelMode},
        // The definition for this command
        Command: inspectCmd,
        Parent:  manifestCmd,
    })

    // This is where you would configure the cobra flags using inspectCmd.Flags()
}

// Business logic: cmd is inspectCmd, args is the positional arguments from os.Args
func inspect(cmd *cobra.Command, args []string) error {
    // Business logic using registry.ImageEngine
    // Do not pull from libpod directly use the domain objects and types
    return nil
}
```
