# Podman Mac Client tutorial

## What is the Podman Mac Client

First and foremost, the Mac Client is under heavy development. We are working on getting the
Mac client to be packaged and run for a native-like experience. This is the setup tutorial
for the Mac client at its current stage of development and packaging.

The purpose of the Mac client for Podman is to allow users to run Podman on a Mac. Since Podman is a Linux
container engine, The Mac client is actually a version of the [Podman-remote client](remote_client.md),
edited to that the client side works on a Mac machine, and connects to a Podman "backend" on a Linux
machine, virtual or physical. The goal is to have a native-like experience when working with the Mac
client, so the command line interface of the remote client is exactly the same as the regular Podman
commands with the exception of some flags and commands that do not apply to the Mac client.

## What you need

To use the Mac client, you will need a binary built for MacOS and a Podman "backend" on a Linux machine;
hereafter referred to as the Podman node. In this context, a Podman node is a Linux system with Podman
installed on it and the varlink service activated.  You will also need to be able to ssh into this
system as a user with privileges to the varlink socket (more on this later).

For best results, use the most recent version of MacOS

## Getting the Mac client
The Mac client is available through [Homebrew](https://brew.sh/).
```
$ brew cask install podman
```

## Setting up the client and Podman node connection

To use the Mac client, you must perform some setup on both the Mac and Podman nodes. In this case,
the Mac node refers to the Mac on which Podman is being run; and the Podman node refers to where
Podman and its storage reside.

### Connection settings
Your Linux box must have ssh enabled, and you must copy your Mac's public key from `~/.sconf sh/id.pub` to
`/root/.ssh/authorized_keys` on your Linux box using `ssh-copy-id` This allows for the use of SSH keys
for remote access.

You may need to edit your `/etc/ssh/sshd_config` in your Linux machine as follows:
```
PermitRootLogin yes
```

Use of SSH keys are strongly encouraged to ensure a secure login. However, if you wish to avoid ‘logging in’ every
time you run a Podman command, you may edit your `/etc/ssh/sshd_config` on your Linux machine as follows:
```
PasswordAuthentication no
PermitRootLogin without-password
```

### Podman node setup
The Podman node must be running a Linux distribution that supports Podman and must have Podman (not the Mac
client) installed. You must also have root access to the node. Check if your system uses systemd:
```
$cat /proc/1/comm
systemd
```
If it does, then simply start the Podman varlink socket:
```
$ sudo systemctl start io.podman.socket
$ sudo systemctl enable io.podman.socket
```

If your system cannot use systemd, then you can manually establish the varlink socket with the Podman
command:
```
$ sudo podman --log-level debug varlink --timeout 0  unix://run/podman/io.podman
```

### Required permissions
For now, the Mac client requires that you be able to run a privileged Podman and have privileged ssh
access to the remote system.  This limitation is being worked on.

#### Running the remote client
There are three different ways to pass connection information into the client: flags, conf file, and
environment variables. All three require information on username and a remote host ip address. Most often,
your username should be root and you can obtain your remote-host-ip using `ip addr`

To connect using flags, you can use
```
$ podman --remote-host remote-host-ip --username root images
REPOSITORY                 TAG               IMAGE ID       CREATED         SIZE
quay.io/podman/stable      latest            9c1e323be87f   10 days ago     414 MB
localhost/test             latest            4b8c27c343e1   4 weeks ago     253 MB
k8s.gcr.io/pause           3.1               da86e6ba6ca1   20 months ago   747 kB
```
If the conf file is set up, you may simply use Podman as you would on the linux machine. Take a look at
[podman-remote.conf.5.md](https://github.com/containers/libpod/blob/master/docs/podman-remote.conf.5.md) on how to use the conf file:

```
$ podman images
REPOSITORY                 TAG               IMAGE ID       CREATED         SIZE
quay.io/podman/stable      latest            9c1e323be87f   10 days ago     414 MB
localhost/test             latest            4b8c27c343e1   4 weeks ago     253 MB
k8s.gcr.io/pause           3.1               da86e6ba6ca1   20 months ago   747 kB
```
