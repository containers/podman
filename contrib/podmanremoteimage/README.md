podman-remote-images
====================

Overview
--------

This directory contains the containerfile for creating a container image which consist podman-remote binary
for each platform (win/linux/mac).

Users can copy those binaries onto the specific platforms using following instructions

- For Windows binary
```bash
$ podman cp $(podman create --name remote-temp quay.io/containers/podman-remote-artifacts:latest):/windows/podman.exe . && podman rm remote-temp
```

- For Linux binary
```bash
$ podman cp $(podman create --name remote-temp quay.io/containers/podman-remote-artifacts:latest):/podman-remote-static-linux_amd64 . && podman rm remote-temp
```

- For Mac binary
```bash
$ podman cp $(podman create --name remote-temp quay.io/containers/podman-remote-artifacts:latest):/darwin/podman . && podman rm remote-temp
```
