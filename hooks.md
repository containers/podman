# OCI Hooks Configuration

[The OCI Runtime Specification defines POSIX-platform Hooks:](
https://github.com/opencontainers/runtime-spec/blob/master/config.md#posix-platform-hooks)

## POSIX-platform Hooks

For POSIX platforms, the configuration structure supports hooks for configuring custom actions related to the life cycle of the container.

hooks (object, OPTIONAL) MAY contain any of the following properties:

 *  prestart (array of objects, OPTIONAL) is an array of pre-start hooks. Entries in the array contain the following properties:
    * path (string, REQUIRED) with similar semantics to [IEEE Std 1003.1-2008 execv's path][ieee-1003.1-2008-functions-exec]. This specification extends the IEEE standard in that path MUST be absolute.
    * args (array of strings, OPTIONAL) with the same semantics as [IEEE Std 1003.1-2008 execv's argv][ieee-1003.1-2008-functions-exec].
    * env (array of strings, OPTIONAL) with the same semantics as IEEE Std 1003.1-2008's environ.
    * timeout (int, OPTIONAL) is the number of seconds before aborting the hook. If set, timeout MUST be greater than zero.
 * poststart (array of objects, OPTIONAL) is an array of post-start hooks. Entries in the array have the same schema as pre-start entries.
 * poststop (array of objects, OPTIONAL) is an array of post-stop hooks. Entries in the array have the same schema as pre-start entries.

Hooks allow users to specify programs to run before or after various lifecycle events. Hooks MUST be called in the listed order. The state of the container MUST be passed to hooks over stdin so that they may do work appropriate to the current state of the container.

### Prestart

The Prestart hooks MUST be called after the start operation is called but before the user-specified program command is executed. On Linux, for example, they are called after the container namespaces are created, so they provide an opportunity to customize the container (e.g. the network namespace could be specified in this hook).

### Poststart

The post-start hooks MUST be called after the user-specified process is executed but before the start operation returns. For example, this hook can notify the user that the container process is spawned.

### Poststop

The post-stop hooks MUST be called after the container is deleted but before the delete operation returns. Cleanup or debugging functions are examples of such a hook.

## CRI-O configuration files for automatically enabling Hooks

The way you enable the hooks above is by editing the OCI Specification to add your hook before running the oci runtime, like runc.  But this is what `CRI-O` and `Kpod create` do for you, so we wanted a way for developers to drop configuration files onto the system, so that their hooks would be able to be plugged in.

One problem with hooks is that the runtime actually stalls execution of the container before running the hooks and stalls completion of the container, until all hooks complete.  This can cause some performance issues.  Also a lot of hooks just check if certain configuration is set and then exit early, without doing anything.  For example the [oci-systemd-hook](https://github.com/projectatomic/oci-systemd-hook) only executes if the command is `init` or `systemd`, otherwise it just exits.  This means if we automatically enable all hooks, every container will have to execute oci-systemd-hook, even if they don't run systemd inside of the container.   Also since there are three stages, prestart, poststart, poststop each hook gets executed three times.



### Json Definition

We decided to add a json file for hook builders which allows them to tell CRI-O when to run the hook and in which stage.
CRI-O reads all json files in /usr/share/containers/oci/hooks.d/*.json and /etc/containers/oci/hooks.d and sets up the specified hooks to run.  If the same name is in both directories, the one in /etc/containers/oci/hooks.d takes precedence.

The json configuration looks like this in GO
```
// HookParams is the structure returned from read the hooks configuration
type HookParams struct {
	Hook          string   `json:"hook"`
	Stage         []string `json:"stages"`
	Cmds          []string `json:"cmds"`
	Annotations   []string `json:"annotations"`
	HasBindMounts bool     `json:"hasbindmounts"`
}
```

| Key    | Description                                                                                                                        | Required/Optional |
| ------ |----------------------------------------------------------------------------------------------------------------------------------- | -------- |
| hook   | Path to the hook                                                                                                                   | Required |
| stages | List of stages to run the hook in: Valid options are `prestart`, `poststart`, `poststop`                                           | Required |
| cmds   | List of regular expressions to match the command for running the container.  If the command matches a regex, the hook will be run  | Optional |
| annotations | List of regular expressions to match against the Annotations in the container runtime spec, if an Annotation matches the hook will be run|optional |
| hasbindmounts | Tells CRI-O to run the hook if the container has bind mounts from the host into the container | Optional |

### Example


```
cat /etc/containers/oci/hooks.d/oci-systemd-hook.json
{
    "cmds": [".*/init$" , ".*/systemd$" ],
    "hook": "/usr/libexec/oci/hooks.d/oci-systemd-hook",
    "stages": [ "prestart", "poststop" ]
}
```

In the above example CRI-O will only run the oci-systemd-hook in the prestart and poststop stage, if the command ends with /init or /systemd


```
cat /etc/containers/oci/hooks.d/oci-systemd-hook.json
{
    "hasbindmounts": true,
    "hook": "/usr/libexec/oci/hooks.d/oci-umount",
    "stages": [ "prestart" ]
}
```
In this example the oci-umount will only be run during the prestart phase if the container has volume/bind mounts from the host into the container.
