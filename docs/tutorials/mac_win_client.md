# Podman Remote clients for macOS and Windows

***
**_NOTE:_** For running Podman on Windows, refer to the [Podman for Windows](podman-for-windows.md) guide, which uses the recommended approach of a Podman-managed Linux backend. For Mac, see the [Podman installation instructions](https://podman.io/getting-started/installation). This guide covers the advanced usage of Podman with a custom Linux VM or a remote external Linux system.
***

## Introduction

The core Podman runtime environment can only run on Linux operating systems.  But other operating systems can use the “remote client” to manage their containers to a Linux backend.  This remote client is nearly identical to the standard Podman program.  Certain functions that do not make sense for remote clients have been removed.  For example, the “--latest” switch for container commands has been removed.

### Brief architecture

The remote client uses a client-server model. You need Podman installed on a Linux machine or VM that also has the SSH daemon running. On the local operating system, when you execute a Podman command, Podman connects to the server via SSH. It then connects to the Podman service by using systemd socket activation. The Podman commands are executed on the server. From the client's point of view, it seems like Podman runs locally.

## Obtaining and installing Podman

### Windows

Installing the Windows Podman client begins by downloading the Podman Windows installer. The Windows installer is built with each Podman release and is downloadable from its [release description page](https://github.com/containers/podman/releases/latest).  The Windows installer file is named `podman-#.#.#-setup.exe`, where the `#` symbols represent the version number of Podman.
As of 2022-11-09 the latest version is [v4.3.0](https://github.com/containers/podman/releases/download/v4.3.0/podman-v4.3.0-setup.exe).

Once you have downloaded the installer to your Windows host, simply double click the installer and Podman will be installed.  The path is also set to put `podman` in the default user path.

Podman must be run at a command prompt using the Windows Command Prompt (`cmd.exe`) or PowerShell (`pwsh.exe`) applications.

### macOS

The Mac Client is available through [Homebrew](https://brew.sh/). You can download homebrew via the instructions on their site. Install podman using:
```
$ brew install podman
```

## Creating the first connection

### Enable the Podman service on the server machine.

Before performing any  Podman client commands, you must enable the podman.sock SystemD service on the Linux server.  In these examples, we are running Podman as a normal, unprivileged user, also known as a rootless user.  By default, the rootless socket listens at  `/run/user/${UID}/podman/podman.sock`.  You can enable and start this socket permanently, using the following commands:
```
$ systemctl --user enable --now podman.socket
```
You will need to enable linger for this user in order for the socket to work when the user is not logged in.

```
sudo loginctl enable-linger $USER
```

You can verify that the socket is listening with a simple Podman command.

```
$ podman --remote info
host:
  arch: amd64
  buildahVersion: 1.16.0-dev
  cgroupVersion: v2
  conmon:
	package: conmon-2.0.19-1.fc32.x86_64
```

#### Enable sshd

In order for the client to communicate with the server you need to enable and start the SSH daemon on your Linux machine, if it is not currently enabled.
```
sudo systemctl enable --now sshd
```

#### Setting up SSH
Remote podman uses SSH to communicate between the client and server. The remote client works considerably smoother using SSH keys. To set up your ssh connection, you need to generate an ssh key pair from your client machine.
```
$ ssh-keygen
```
Your public key by default should be in your home directory under .ssh\id_rsa.pub. You then need to copy the contents of id_rsa.pub and append it into  ~/.ssh/authorized_keys on the Linux  server. On a Mac, you can automate this using ssh-copy-id.

If you do not wish to use SSH keys, you will be prompted with each Podman command for your login password.

## Using the client

The first step in using the Podman remote client is to configure a connection..

You can add a connection by using the `podman system connection add` command.

```
C:\Users\baude> podman system connection add baude --identity c:\Users\baude\.ssh\id_rsa ssh://192.168.122.1/run/user/1000/podman/podman.sock
```

This will add a remote connection to Podman and if it is the first connection added, it will mark the connection as the default.  You can observe your connections with `podman system connection list`

```
C:\Users\baude> podman system connection list
Name	Identity 	URI
baude*	id_rsa	       ssh://baude@192.168.122.1/run/user/1000/podman/podman.sock
```

Now we can test the connection with `podman info`.

```
C:\Users\baude> podman info
host:
  arch: amd64
  buildahVersion: 1.16.0-dev
  cgroupVersion: v2
  conmon:
	package: conmon-2.0.19-1.fc32.x86_64
```

Podman has also introduced a “--connection” flag where you can use other connections you have defined.  If no connection is provided, the default connection will be used.

```
C:\Users\baude> podman system connection --help
```

## Wrap up

You can use the podman remote clients to manage your containers running on a Linux server.  The communication between client and server relies heavily on SSH connections and the use of SSH keys are encouraged.  Once you have Podman installed on your remote client, you should set up a connection using   `podman system connection add` which will then be used by subsequent Podman commands.

## History
Originally published on [Red Hat Enable Sysadmin](https://www.redhat.com/sysadmin/podman-clients-macos-windows)
