####> This option file is used in:
####>   podman create, run
####> If you edit this file, make sure your changes
####> are applicable to all of those.
#### **--health-on-failure**=*action*

Action to take once the container transitions to an unhealthy state.  The default is **none**.

- **none**: Take no action.
- **kill**: Kill the container.
- **restart**: Restart the container.  Do not combine the `restart` action with the `--restart` flag.  When running inside of a systemd unit, consider using the `kill` or `stop` action instead to make use of systemd's restart policy.
- **stop**: Stop the container.
