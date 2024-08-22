####> This option file is used in:
####>   podman artifact pull, artifact push, build, container runlabel, farm build, kube play, manifest add, manifest push, pull, push, search
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--creds**=*[username[:password]]*

The [username[:password]] to use to authenticate with the registry, if required.
If one or both values are not supplied, a command line prompt appears and the
value can be entered. The password is entered without echo.

Note that the specified credentials are only used to authenticate against
target registries.  They are not used for mirrors or when the registry gets
rewritten (see `containers-registries.conf(5)`); to authenticate against those
consider using a `containers-auth.json(5)` file.
