# CRI-O Tutorial

This tutorial will walk you through the installation of [CRI-O](https://github.com/kubernetes-incubator/cri-o), an Open Container Initiative-based implementation of [Kubernetes Container Runtime Interface](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/node/container-runtime-interface-v1.md), and the creation of [Redis](https://redis.io/) server running in a [Pod](http://kubernetes.io/docs/user-guide/pods/).

## Prerequisites

A Linux machine is required to download and build the `CRI-O` components and run the commands in this tutorial.

Create a machine running Ubuntu 16.10:

```
gcloud compute instances create cri-o \
  --machine-type n1-standard-2 \
  --image-family ubuntu-1610 \
  --image-project ubuntu-os-cloud
```

SSH into the machine:

```
gcloud compute ssh cri-o
```

## Installation

This section will walk you through installing the following components:

* crio - The implementation of the Kubernetes CRI, which manages Pods.
* crioctl - The crio client for testing.
* cni - The Container Network Interface
* runc - The OCI runtime to launch the container


### runc

Download the `runc` release binary:

```
wget https://github.com/opencontainers/runc/releases/download/v1.0.0-rc4/runc.amd64
```

Set the executable bit and copy the `runc` binary into your PATH:

```
chmod +x runc.amd64
```

```
sudo mv runc.amd64 /usr/bin/runc
```

Print the `runc` version:

```
runc -version
```
```
runc version 1.0.0-rc4
commit: 2e7cfe036e2c6dc51ccca6eb7fa3ee6b63976dcd
spec: 1.0.0
```

### crio

The `crio` project does not ship binary releases so you'll need to build it from source.

#### Install the Go runtime and tool chain

Download the Go 1.7.4 binary release:

```
wget https://storage.googleapis.com/golang/go1.7.4.linux-amd64.tar.gz
```

Install Go 1.7.4:

```
sudo tar -xvf go1.7.4.linux-amd64.tar.gz -C /usr/local/
```

```
mkdir -p $HOME/go/src
```

```
export GOPATH=$HOME/go
```

```
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
```

At this point the Go 1.7.4 tool chain should be installed:

```
go version
```

```
go version go1.7.4 linux/amd64
```

#### Build crio from source

```
sudo apt-get install -y libglib2.0-dev libseccomp-dev libapparmor-dev
```

```
go get -d github.com/kubernetes-incubator/cri-o
```

```
cd $GOPATH/src/github.com/kubernetes-incubator/cri-o
```

```
make install.tools
```

```
make
```

```
sudo make install
```

Output:

```
install -D -m 755 kpod /usr/local/bin/kpod
install -D -m 755 crio /usr/local/bin/crio
install -D -m 755 crioctl /usr/local/bin/crioctl
install -D -m 755 conmon/conmon /usr/local/libexec/crio/conmon
install -D -m 755 pause/pause /usr/local/libexec/crio/pause
install -d -m 755 /usr/local/share/man/man{1,5,8}
install -m 644 docs/kpod.1 docs/kpod-launch.1 -t /usr/local/share/man/man1
install -m 644 docs/crio.conf.5 -t /usr/local/share/man/man5
install -m 644 docs/crio.8 -t /usr/local/share/man/man8
install -D -m 644 crio.conf /etc/crio/crio.conf
install -D -m 644 seccomp.json /etc/crio/seccomp.json
```

If you are installing for the first time, generate config as follows:

```
sudo make install.config
```

Output:

```
install -D -m 644 crio.conf /etc/crio/crio.conf
install -D -m 644 seccomp.json /etc/crio/seccomp.json
```

#### Start the crio system daemon

```
sudo sh -c 'echo "[Unit]
Description=OCI-based implementation of Kubernetes Container Runtime Interface
Documentation=https://github.com/kubernetes-incubator/cri-o

[Service]
ExecStart=/usr/local/bin/crio --log-level debug
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target" > /etc/systemd/system/crio.service'
```

```
sudo systemctl daemon-reload
```
```
sudo systemctl enable crio
```
```
sudo systemctl start crio
```

#### Ensure the crio service is running

```
sudo crioctl runtimeversion
```
```
VersionResponse: Version: 0.1.0, RuntimeName: runc, RuntimeVersion: 1.0.0-rc4, RuntimeApiVersion: v1alpha1
```

### CNI plugins

This tutorial will use the latest version of `CNI` plugins from the master branch and build it from source.

Download the `CNI` plugins source tree:

```
go get -d github.com/containernetworking/plugins
```

```
cd $GOPATH/src/github.com/containernetworking/plugins
```

Build the `CNI` plugins:

```
./build.sh
```

Output:

```
Building API
Building reference CLI
Building plugins
   flannel
   tuning
   bridge
   ipvlan
   loopback
   macvlan
   ptp
   dhcp
   host-local
   noop
```

Install the `CNI` plugins:

```
sudo mkdir -p /opt/cni/bin
```

```
sudo cp bin/* /opt/cni/bin/
```

#### Configure CNI

```
sudo mkdir -p /etc/cni/net.d
```

```
sudo sh -c 'cat >/etc/cni/net.d/10-mynet.conf <<-EOF
{
    "cniVersion": "0.2.0",
    "name": "mynet",
    "type": "bridge",
    "bridge": "cni0",
    "isGateway": true,
    "ipMasq": true,
    "ipam": {
        "type": "host-local",
        "subnet": "10.88.0.0/16",
        "routes": [
            { "dst": "0.0.0.0/0"  }
        ]
    }
}
EOF'
```

```
sudo sh -c 'cat >/etc/cni/net.d/99-loopback.conf <<-EOF
{
    "cniVersion": "0.2.0",
    "type": "loopback"
}
EOF'
```

At this point `CNI` is installed and configured to allocation IP address to containers from the `10.88.0.0/16` subnet.

## Pod Tutorial

Now that the `CRI-O` components have been installed and configured we are ready to create a Pod. This section will walk you through launching a Redis server in a Pod. Once the Redis server is running we'll use telnet to verify it's working, then we'll stop the Redis server and clean up the Pod.

### Creating a Pod

First we need to setup a Pod sandbox using a Pod configuration, which can be found in the `cri-o` source tree:

```
cd $GOPATH/src/github.com/kubernetes-incubator/cri-o
```

Next create the Pod and capture the Pod ID for later use:

```
POD_ID=$(sudo crioctl pod run --config test/testdata/sandbox_config.json)
```

> sudo crioctl pod run --config test/testdata/sandbox_config.json

Use the `crioctl` command to get the status of the Pod:

```
sudo crioctl pod status --id $POD_ID
```

Output:

```
ID: cd6c0883663c6f4f99697aaa15af8219e351e03696bd866bc3ac055ef289702a
Name: podsandbox1
UID: redhat-test-crio
Namespace: redhat.test.crio
Attempt: 1
Status: SANDBOX_READY
Created: 2016-12-14 15:59:04.373680832 +0000 UTC
Network namespace: /var/run/netns/cni-bc37b858-fb4d-41e6-58b0-9905d0ba23f8
IP Address: 10.88.0.2
Labels:
	group -> test
Annotations:
	owner -> hmeng
	security.alpha.kubernetes.io/seccomp/pod -> unconfined
	security.alpha.kubernetes.io/sysctls -> kernel.shm_rmid_forced=1,net.ipv4.ip_local_port_range=1024 65000
	security.alpha.kubernetes.io/unsafe-sysctls -> kernel.msgmax=8192
```

### Create a Redis container inside the Pod

Use the `crioctl` command to pull the redis image, create a redis container from a container configuration and attach it to the Pod created earlier:

```
sudo crioctl image pull redis:alpine
CONTAINER_ID=$(sudo crioctl ctr create --pod $POD_ID --config test/testdata/container_redis.json)
```

> sudo crioctl ctr create --pod $POD_ID --config test/testdata/container_redis.json

The `crioctl ctr create` command  will take a few seconds to return because the redis container needs to be pulled.

Start the Redis container:

```
sudo crioctl ctr start --id $CONTAINER_ID
```

Get the status for the Redis container:

```
sudo crioctl ctr status --id $CONTAINER_ID
```

Output:

```
ID: d0147eb67968d81aaddbccc46cf1030211774b5280fad35bce2fdb0a507a2e7a
Name: podsandbox1-redis
Status: CONTAINER_RUNNING
Created: 2016-12-14 16:00:42.889089352 +0000 UTC
Started: 2016-12-14 16:01:56.733704267 +0000 UTC
```

### Test the Redis container

Connect to the Pod IP on port 6379:

```
telnet 10.88.0.2 6379
```

```
Trying 10.88.0.2...
Connected to 10.88.0.2.
Escape character is '^]'.
```

At the prompt type `MONITOR`:

```
Trying 10.88.0.2...
Connected to 10.88.0.2.
Escape character is '^]'.
MONITOR
+OK
```

Exit the telnet session by typing `ctrl-]` and `quit` at the prompt:

```
^]

telnet> quit
Connection closed.
```

#### Viewing the Redis logs

The Redis logs are logged to the stderr of the crio service, which can be viewed using `journalctl`:

```
sudo journalctl -u crio --no-pager
```

### Stop the redis container and delete the Pod

```
sudo crioctl ctr stop --id $CONTAINER_ID
```

```
sudo crioctl ctr remove --id $CONTAINER_ID
```

```
sudo crioctl pod stop --id $POD_ID
```

```
sudo crioctl pod remove --id $POD_ID
```

```
sudo crioctl pod list
```

```
sudo crioctl ctr list
```
