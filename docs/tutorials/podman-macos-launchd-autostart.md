# Running Podman Machine at Startup on macOS with launchd

This tutorial shows how to automatically start `podman machine` when you log in
on macOS using `launchd` — the macOS init and service management daemon.

## Why launchd?

On Linux, systemd manages services. On macOS, `launchd` fills the same role.
Podman's `podman machine` command manages a Linux VM on macOS; the VM needs to
be started before you can run containers.

> **Note:** Only the machine VM needs to run at startup. Individual containers
> should be started via Quadlet or `podman auto-update`, not from launchd
> directly.

## Prerequisites

- Podman installed (`brew install podman`)
- At least one machine already created (`podman machine init`)
- The machine name you want to autostart (default: `podman-machine-default`)

Check your machine name:
```bash
podman machine list
```

## Step 1 — Create the Launch Agent plist

Launch Agents live in `~/Library/LaunchAgents/` and run as your user.

Create the file `~/Library/LaunchAgents/com.github.containers.podman.machine.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <!-- A unique reverse-DNS label for this agent -->
    <key>Label</key>
    <string>com.github.containers.podman.machine</string>

    <!-- Run once at login; do NOT set KeepAlive to true.
         Podman machine forks its own daemon process, so launchd must
         NOT try to restart it — doing so would create an restart loop. -->
    <key>RunAtLoad</key>
    <true/>

    <!-- Only start once; do not restart after the command exits -->
    <key>KeepAlive</key>
    <false/>

    <!-- Full path to the podman binary (brew installs it here) -->
    <key>ProgramArguments</key>
    <array>
        <string>/opt/homebrew/bin/podman</string>
        <string>machine</string>
        <string>start</string>
    </array>

    <!-- Where to redirect stdout and stderr for debugging -->
    <key>StandardOutPath</key>
    <string>/tmp/podman-machine-start.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/podman-machine-start.log</string>
</dict>
</plist>
```

### Adjustments for Intel Macs

On Intel Macs, Homebrew installs to `/usr/local` instead of `/opt/homebrew`:

```xml
<string>/usr/local/bin/podman</string>
```

### Starting a non-default machine

To autostart a machine named `dev` instead:

```xml
<array>
    <string>/opt/homebrew/bin/podman</string>
    <string>machine</string>
    <string>start</string>
    <string>dev</string>   <!-- Add the machine name here -->
</array>
```

## Step 2 — Load the Launch Agent

```bash
launchctl load ~/Library/LaunchAgents/com.github.containers.podman.machine.plist
```

You can also use the newer `bootstrap`/`bootout` syntax:

```bash
launchctl bootstrap gui/$(id -u) \
    ~/Library/LaunchAgents/com.github.containers.podman.machine.plist
```

## Step 3 — Verify it works

### Check the agent is loaded

```bash
launchctl list | grep podman
# Should show: -  0  com.github.containers.podman.machine
```

### Start it manually (without rebooting)

```bash
launchctl start com.github.containers.podman.machine
```

Wait a few seconds, then:

```bash
podman machine list
# NAME                     VM TYPE     CREATED     LAST UP            CPUS
# podman-machine-default*  applehv     2 days ago  Currently running  4
```

### Check the log for errors

```bash
cat /tmp/podman-machine-start.log
```

## Stopping and unloading

To stop the machine:

```bash
podman machine stop
```

To unload the agent so it no longer starts at login:

```bash
launchctl unload ~/Library/LaunchAgents/com.github.containers.podman.machine.plist
# or with the newer API:
launchctl bootout gui/$(id -u) \
    ~/Library/LaunchAgents/com.github.containers.podman.machine.plist
```

## Troubleshooting

### The machine keeps restarting in a loop

**Cause:** `KeepAlive` is set to `true`. Because `podman machine start` forks a
background daemon and then exits immediately, launchd sees the exit and
relaunches it — which conflicts with the already-running VM.

**Fix:** Set `<key>KeepAlive</key><false/>` as shown above.

### `podman machine start` is not found

**Cause:** The path `/opt/homebrew/bin/podman` is wrong.

**Fix:** Find the correct path with:
```bash
which podman
```
Update `ProgramArguments` in the plist to use that path.

### The machine starts but containers can't connect

**Cause:** There is a small window between `podman machine start` returning and
the UNIX socket becoming available. If you have scripts that run immediately
after login and depend on Podman, add a short `sleep` or a retry loop:

```bash
# In your script or shell profile:
until podman info &>/dev/null; do sleep 1; done
```

### Permission denied writing to log file

**Cause:** `/tmp` might not be writable if the agent runs before the user
session is fully established.

**Fix:** Use a path under `~/Library/Logs/`:
```xml
<key>StandardOutPath</key>
<string>/Users/YOUR_USERNAME/Library/Logs/podman-machine-start.log</string>
```

## Auto-starting containers after the machine starts

Once the machine is running, use **Quadlet** to manage containers declaratively.
Quadlet files live in `~/.config/containers/systemd/` and are managed by the
`systemd` instance inside the Podman machine.

See [Quadlet documentation](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html)
for details.

## See also

- `man launchd.plist` — full reference for plist keys
- `man launchctl` — launchctl command reference
- [Podman Machine overview](podman_tutorial.md)
- [Podman Quadlet documentation](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html)
