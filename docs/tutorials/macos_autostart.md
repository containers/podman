# Running Podman on macOS startup with launchd

On macOS, Podman requires a virtual machine to run containers. By default,
you must manually start this VM with `podman machine start` each time you
log in. This tutorial shows how to use macOS's `launchd` service manager
to automatically start your Podman machine at login.

## Prerequisites

- macOS with Podman installed (e.g., via `brew install podman`)
- A Podman machine already initialized (`podman machine init`)
- The Podman machine should start successfully when running `podman machine start` manually

## Overview

macOS uses `launchd` to manage services. User-level services are defined by
property list (plist) files placed in `~/Library/LaunchAgents/`. These
"LaunchAgents" run when you log in and stop when you log out.

Key considerations for running `podman machine start` under launchd:

- **Do not use `KeepAlive` or automatic restart.** `podman machine start` is a
  one-shot command that starts the VM and then exits. If launchd restarts it,
  it will find the machine already running and return an error, which can cause
  a restart loop.
- **Use `RunAtLoad` to start the machine once at login.**
- **Use the full path to the `podman` binary** to avoid PATH issues.

## Step 1: Find your Podman binary path

Determine the full path to your `podman` binary:

```
$ which podman
/opt/homebrew/bin/podman
```

If you installed Podman with Homebrew on an Apple Silicon Mac, the path is
typically `/opt/homebrew/bin/podman`. On Intel Macs, it is usually
`/usr/local/bin/podman`.

## Step 2: Create the LaunchAgent plist

Create the file `~/Library/LaunchAgents/com.podman.machine.default.plist`
with the following content. Replace `/opt/homebrew/bin/podman` with the
path from Step 1 if it differs on your system. Replace `podman-machine-default`
with the name of your machine if you are not using the default.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.podman.machine.default</string>

    <key>ProgramArguments</key>
    <array>
        <string>/opt/homebrew/bin/podman</string>
        <string>machine</string>
        <string>start</string>
        <string>podman-machine-default</string>
    </array>

    <key>RunAtLoad</key>
    <true/>

    <key>StandardOutPath</key>
    <string>/tmp/podman-machine-start.log</string>

    <key>StandardErrorPath</key>
    <string>/tmp/podman-machine-start.err.log</string>
</dict>
</plist>
```

### Explanation of the plist keys

| Key | Purpose |
|---|---|
| `Label` | A unique identifier for the service. |
| `ProgramArguments` | The command and arguments to run. Each argument is a separate `<string>`. |
| `RunAtLoad` | Start the command as soon as the agent is loaded (at login). |
| `StandardOutPath` | Where to write stdout for debugging. |
| `StandardErrorPath` | Where to write stderr for debugging. |

### Important: do not use KeepAlive

A common mistake is to add `<key>KeepAlive</key><true/>` to the plist.
This tells launchd to restart the process whenever it exits. Because
`podman machine start` exits after the VM starts, launchd would immediately
try to start it again. The second attempt finds the machine already running
and fails, creating a rapid restart loop. This can interfere with the Podman
CLI and degrade system performance. Always omit `KeepAlive` for this use case.

## Step 3: Load the LaunchAgent

Load the agent so it takes effect without logging out and back in:

```
$ launchctl load ~/Library/LaunchAgents/com.podman.machine.default.plist
```

The Podman machine should start. You can verify with:

```
$ podman machine list
NAME                    VM TYPE     CREATED      LAST UP             CPUS    MEMORY      DISK SIZE
podman-machine-default  applehv     2 days ago   Currently running   4       2GiB        100GiB
```

## Step 4: Verify the logs

If the machine did not start as expected, check the log files for errors:

```
$ cat /tmp/podman-machine-start.log
$ cat /tmp/podman-machine-start.err.log
```

## Managing the LaunchAgent

### Stop the machine from auto-starting

To prevent the Podman machine from starting at login, unload the agent:

```
$ launchctl unload ~/Library/LaunchAgents/com.podman.machine.default.plist
```

### Re-enable auto-starting

```
$ launchctl load ~/Library/LaunchAgents/com.podman.machine.default.plist
```

### Remove the LaunchAgent entirely

```
$ launchctl unload ~/Library/LaunchAgents/com.podman.machine.default.plist
$ rm ~/Library/LaunchAgents/com.podman.machine.default.plist
```

## Troubleshooting

### The machine enters a restart loop

This typically occurs when `KeepAlive` is set to `true` in the plist. Edit the
plist file, remove the `KeepAlive` key and its value, then unload and reload:

```
$ launchctl unload ~/Library/LaunchAgents/com.podman.machine.default.plist
$ launchctl load ~/Library/LaunchAgents/com.podman.machine.default.plist
```

### The machine does not start

1. Check that the `podman` path in the plist is correct (`which podman`).
2. Check the log files at `/tmp/podman-machine-start.log` and
   `/tmp/podman-machine-start.err.log`.
3. Verify the machine starts correctly when running `podman machine start`
   manually in a terminal.
4. Ensure the machine has been initialized with `podman machine init`.

### Permission issues

LaunchAgents run in your user context, so they have the same permissions
as your terminal session. If `podman machine start` works in a terminal but
not from launchd, check that the plist uses the full absolute path to the
`podman` binary.

## SEE ALSO

**[podman-machine-start(1)](../source/markdown/podman-machine-start.1.md.in)**, **[podman-machine(1)](../source/markdown/podman-machine.1.md)**

Apple's [launchd documentation](https://developer.apple.com/library/archive/documentation/MacOSX/Conceptual/BPSystemStartup/Chapters/CreatingLaunchdJobs.html)
