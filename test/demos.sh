#!/bin/bash

echo "This is a demo of the podman search command."
echo ""

read -p "--> cat /etc/containers/registries.conf"
cat /etc/containers/registries.conf
echo ""

read -p "--> podman search fedora"
podman search fedora
echo ""

read -p "--> podman search --filter stars=34 fedora"
podman search --filter stars=34 fedora
echo ""

read -p "--> podman search --filter is-automated=false --filter stars=34 --filter is-official fedora"
podman search --filter is-automated=false --filter stars=34 --filter is-official fedora
echo ""

read -p "--> podman search --no-trunc --limit 3 fedora"
podman search --no-trunc --limit 3 fedora
echo ""

read -p "--> podman search --registry registry.access.redhat.com rhel7"
podman search --registry registry.access.redhat.com rhel7
echo ""

read -p "--> podman search --format \"table {{.Name}} {{.Description}}\" fedora"
podman search --format "table {{.Name}} {{.Description}}" fedora
echo ""

read -p "Demo of a few podman run and create options"
echo ""

read -p "--> podman run --memory 80m fedora cat /sys/fs/cgroup/memory/memory.limit_in_bytes"
podman run --rm --memory 80m fedora cat /sys/fs/cgroup/memory/memory.limit_in_bytes
echo ""

read -p "--> podman run --memory 80m --memory-reservation 40m fedora cat /sys/fs/cgroup/memory/memory.soft_limit_in_bytes"
podman run --rm --memory 80m --memory-reservation 40m fedora cat /sys/fs/cgroup/memory/memory.soft_limit_in_bytes
echo ""

read -p "--> podman run --memory 40m --memory-reservation 80m fedora cat /sys/fs/cgroup/memory/memory.soft_limit_in_bytes"
podman run --rm --memory 40m --memory-reservation 80m fedora cat /sys/fs/cgroup/memory/memory.soft_limit_in_bytes
echo ""

read -p "--> podman run --memory-swappiness 15 fedora cat /sys/fs/cgroup/memory/memory.swappiness"
podman run --rm --memory-swappiness 15 fedora cat /sys/fs/cgroup/memory/memory.swappiness
echo ""

read -p "--> podman run --kernel-memory 40m fedora cat /sys/fs/cgroup/memory/memory.kmem.limit_in_bytes"
podman run --rm --kernel-memory 40m fedora cat /sys/fs/cgroup/memory/memory.kmem.limit_in_bytes
echo ""

read -p "--> podman run --cpu-period 5000 fedora cat /sys/fs/cgroup/cpu/cpu.cfs_period_us"
podman run --rm --cpu-period 5000 fedora cat /sys/fs/cgroup/cpu/cpu.cfs_period_us
echo ""

read -p "--> podman run --cpu-quota 15000 --cpu-period 5000 fedora cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us"
podman run --rm --cpu-quota 15000 --cpu-period 5000 fedora cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us
echo ""

read -p "--> podman run --cpus 0.5 fedora /bin/bash"
read -p "cat /sys/fs/cgroup/cpu/cpu.cfs_period_us"
podman run --rm --cpus 0.5 fedora cat /sys/fs/cgroup/cpu/cpu.cfs_period_us
read -p "cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us"
podman run --rm --cpus 0.5 fedora cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us
echo ""

read -p "--> podman run --cpu-shares 2 fedora cat /sys/fs/cgroup/cpu/cpu.shares"
podman run --rm --cpu-shares 2 fedora cat /sys/fs/cgroup/cpu/cpu.shares
echo ""

read -p "--> podman run --cpuset-cpus=0,2 fedora cat /sys/fs/cgroup/cpuset/cpuset.cpus"
podman run --rm --cpuset-cpus=0,2 fedora cat /sys/fs/cgroup/cpuset/cpuset.cpus
echo ""

read -p "--> podman run --sysctl net.core.somaxconn=65535 alpine sysctl net.core.somaxconn"
podman run --rm --sysctl net.core.somaxconn=65535 alpine sysctl net.core.somaxconn
echo ""

read -p "--> podman run --ulimit nofile=1024:1028 fedora ulimit -n"
podman run --rm --ulimit nofile=1024:1028 fedora ulimit -n
echo ""

read -p "--> podman run --blkio-weight 15 fedora cat /sys/fs/cgroup/blkio/blkio.weight"
podman run --rm --blkio-weight 15 fedora cat /sys/fs/cgroup/blkio/blkio.weight
echo ""

read -p "End of Demo."
echo "Thank you!"