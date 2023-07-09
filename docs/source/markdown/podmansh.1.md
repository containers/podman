% podmansh 1

## NAME
podmansh - Execute login shell within the Podman `podmansh` container

## SYNOPSIS
**podmansh**

## DESCRIPTION

Execute a user shell within a container when the user logs into the system. The container that the users get added to can be defined via a Podman Quadlet file. This user only has access to volumes and capabilities configured into the Quadlet file.

Administrators can create a Quadlet in /etc/containers/systemd/users, which systemd will start for all users when they log in. The administrator can create a specific Quadlet with the container name `podmansh`, then enable users to use the login shell /usr/bin/podmansh.  These user login shells are automatically executed inside  the `podmansh` container via Podman.	.

Optionally, the administrator can place Quadlet files in the /etc/containers/systemd/users/${UID} directory for a user. Only this UID will execute these Quadlet services when that user logs in.

The user is confined to the container environment via all of the security mechanisms, including SELinux. The only information that will be available from the system comes from volumes leaked into the container.

Systemd will automatically create the container when the user session is started. Systemd will take down the container when all connections to the user session are removed. This means users can log in to the system multiple times, with each session connected to the same container.

NOTE: This feature is currently a Tech Preview. Changes can be expected in upcoming versions.

## Setup
Modify user login session using usermod

```
# usermod -s /usr/bin/podmansh testu
# grep testu /etc/passwd
testu:x:4004:4004::/home/testu:/usr/bin/podmansh
```

Now create a podman Quadlet file that looks something like one of the following.

Fully locked down container, no access to host OS.

```
sudo cat > /etc/containers/systemd/users/podmansh.container << _EOF
[Unit]
Description=The podmansh container
After=local-fs.target

[Container]
Image=registry.fedoraproject.org/fedora
ContainerName=podmansh
RemapUsers=keep-id
RunInit=yes
DropCapabilities=all
NoNewPrivileges=true

Exec=sleep infinity

[Install]
RequiredBy=default.target
_EOF
```

Users inside of this Quadlet are allowed to become root within the user namespace, and able to read/write content in their homedirectory which is mounted from a subdir `data` of the hosts users account.

```
sudo cat > /etc/containers/systemd/users/podmansh.container << _EOF
[Unit]
Description=The podmansh container
After=local-fs.target

[Container]
Image=registry.fedoraproject.org/fedora
ContainerName=podmansh
RemapUsers=keep-id
RunInit=yes

Volume=%h/data:%h:Z
Exec=sleep infinity

[Service]
ExecStartPre=/usr/bin/mkdir -p %h/data

[Install]
RequiredBy=default.target
_EOF
```

Users inside this container will be allowed to execute containers with SELinux
separate and able to read and write content in the $HOME/data directory.

```
sudo cat > /etc/containers/systemd/users/podmansh.container << _EOF
[Unit]
Description=The podmansh container
After=local-fs.target

[Container]
Image=registry.fedoraproject.org/fedora
ContainerName=podmansh
RemapUsers=keep-id
RunInit=yes
PodmanArgs=--security-opt=unmask=/sys/fs/selinux \
	--security-opt=label=nested \
	--security-opt=label=user:container_user_u \
	--security-opt=label=type:container_user_t \
	--security-opt=label=role:container_user_r \
	--security-opt=label=level:s0-s0:c0.c1023

Volume=%h/data:%h:Z
WorkingDir=%h
Volume=/sys/fs/selinux:/sys/fs/selinux
Exec=sleep infinity

[Service]
ExecStartPre=/usr/bin/mkdir -p %h/data

[Install]
RequiredBy=default.target
_EOF
```

## SEE ALSO
**[podman(1)](podman.1.md)**, **[podman-exec(1)](podman-exec.1.md)**, **quadlet(5)**

## HISTORY
May 2023, Originally compiled by Dan Walsh <dwalsh@redhat.com>
