####> This option file is used in:
####>   podman create, run
####> If file is edited, make sure the changes
####> are applicable to all of those.
#### **--uidmap**=*container_uid:from_uid:amount*

Run the container in a new user namespace using the supplied UID mapping. This
option conflicts with the **--userns** and **--subuidname** options. This
option provides a way to map host UIDs to container UIDs. It can be passed
several times to map different ranges.

The *from_uid* value is based upon the user running the command, either rootful or rootless users.
* rootful user:  *container_uid*:*host_uid*:*amount*
* rootless user: *container_uid*:*intermediate_uid*:*amount*

When **podman <<subcommand>>** is called by a privileged user, the option **--uidmap**
works as a direct mapping between host UIDs and container UIDs.

host UID -> container UID

The _amount_ specifies the number of consecutive UIDs that is mapped.
If for example _amount_ is **4** the mapping looks like:

|   host UID     |    container UID    |
| ----------     | ----------------    |
| *from_uid*     | *container_uid*     |
| *from_uid* + 1 | *container_uid* + 1 |
| *from_uid* + 2 | *container_uid* + 2 |
| *from_uid* + 3 | *container_uid* + 3 |

When **podman <<subcommand>>** is called by an unprivileged user (i.e. running rootless),
the value *from_uid* is interpreted as an "intermediate UID". In the rootless
case, host UIDs are not mapped directly to container UIDs. Instead the mapping
happens over two mapping steps:

host UID -> intermediate UID -> container UID

The **--uidmap** option only influences the second mapping step.

The first mapping step is derived by Podman from the contents of the file
_/etc/subuid_ and the UID of the user calling Podman.

First mapping step:

| host UID            | intermediate UID |
| --------            | ---------------- |
| UID for Podman user |                0 |
| 1st subordinate UID |                1 |
| 2nd subordinate UID |                2 |
| 3rd subordinate UID |                3 |
| nth subordinate UID |                n |

To be able to use intermediate UIDs greater than zero, the user needs to have
subordinate UIDs configured in _/etc/subuid_. See **subuid**(5).

The second mapping step is configured with **--uidmap**.

If for example _amount_ is **5** the second mapping step looks like:

|   intermediate UID   |    container UID    |
| ------------------   | ----------------    |
| *from_uid*           | *container_uid*     |
| *from_uid* + 1       | *container_uid* + 1 |
| *from_uid* + 2       | *container_uid* + 2 |
| *from_uid* + 3       | *container_uid* + 3 |
| *from_uid* + 4       | *container_uid* + 4 |

When running as rootless, Podman uses all the ranges configured in the _/etc/subuid_ file.

The current user ID is mapped to UID=0 in the rootless user namespace.
Every additional range is added sequentially afterward:

|   host                | rootless user namespace | length              |
| ------                | ----------------------- | ------              |
| $UID                  | 0                       | 1                   |
| 1                     | $FIRST_RANGE_ID         | $FIRST_RANGE_LENGTH |
| 1+$FIRST_RANGE_LENGTH | $SECOND_RANGE_ID        | $SECOND_RANGE_LENGTH|

Even if a user does not have any subordinate UIDs in  _/etc/subuid_,
**--uidmap** can be used to map the normal UID of the user to a
container UID by running `podman <<subcommand>> --uidmap $container_uid:0:1 --user $container_uid ...`.

Note: the **--uidmap** flag cannot be called in conjunction with the **--pod** flag as a uidmap cannot be set on the container level when in a pod.
