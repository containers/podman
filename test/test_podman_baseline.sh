#!/bin/bash
# test_podman_baseline.sh
# A script to be run at the command line with Podman installed.
# This should be run against a new kit to provide base level testing
# on a freshly installed machine with no images or container in
# play.  This currently needs to be run as root.
#
# To run this command:
#
# /bin/bash -v test_podman_baseline.sh -d # Install and then deinstall Docker
# /bin/bash -v test_podman_baseline.sh -e # Stop on error
# /bin/bash -v test_podman_baseline.sh    # Continue on error

#######
# See if we want to stop on errors and/or install and then remove Docker.
#######
showerror=0
installdocker=0
while getopts "de" opt; do
    case "$opt" in
    d) installdocker=1
       ;;
    e) showerror=1
       ;;
    esac
done

if [ "$installdocker" -eq 1 ]
then
    echo "Script will install and then deinstall Docker."
fi

if [ "$showerror" -eq 1 ]
then
    echo "Script will stop on unexpected errors."
    set -eu
fi

pkg_manager=`command -v dnf`
if [ -z "$pkg_manager" ]; then
    pkg_manager=`command -v yum`
fi

echo "Package manager binary: $pkg_manager"

########
# Next two commands should return blanks
########
podman images
podman ps --all

########
# Run ls in redis container, this should work
########
ctrid=$(podman pull registry.access.redhat.com/rhscl/redis-32-rhel7)
podman run $ctrid ls /

########
# Remove images and containers
########
podman rm --all
podman rmi --all

########
# Create Fedora based image
########
image=$(podman pull fedora)
echo $image

########
# Run container and display contents in /etc
########
podman run $image ls -alF /etc

########
# Run Java in the container - should ERROR but never stop
########
podman run $image java 2>&1 || echo $?

########
# Clean out containers
########
podman rm --all

########
# Install java onto the container, commit it, then run it showing java usage
########
podman run --net=host $image dnf -y install java
javaimage=$(podman ps --all -q)
podman commit $javaimage javaimage
podman run javaimage java

########
# Cleanup containers and images
########
podman rm --all
podman rmi --all

########
# Check images and containers, should be blanks
########
podman ps --all
podman images

########
# Create Fedora based container
########
image=$(podman pull fedora)
echo $image
podman run $image ls /

########
# Create shell script to test on
########
FILE=./runecho.sh
/bin/cat <<EOM >$FILE
#!/bin/bash
for i in {1..9};
do
    echo "This is a new container pull ipbabble [" \$i "]"
done
EOM
chmod +x $FILE

########
# Copy and run file on container
########
ctrid=$(podman ps --all -q)
mnt=$(podman mount $ctrid)
cp ./runecho.sh ${mnt}/tmp/runecho.sh
podman umount $ctrid
podman commit $ctrid runecho
podman run runecho ./tmp/runecho.sh

########
# Inspect the container, verifying above was put into it
########
podman inspect $ctrid

########
# Check the images there should be a runecho image
########
podman images

########
# Remove the containers
########
podman rm -a

if [ "$installdocker" -eq 1 ]
then
    ########
    # Install Docker, but not for long!
    ########
    $package_manager -y install docker
fi
systemctl restart docker

########
# Push fedora-bashecho to the Docker daemon
########
podman push runecho docker-daemon:fedora-bashecho:latest

########
# Run fedora-bashecho pull Docker
########
docker run fedora-bashecho ./tmp/runecho.sh

if [ "$installdocker" -eq 1 ]
then
    ########
    # Time to remove Docker
    ########
    $package_manager -y remove docker
fi

########
# Clean up Podman
########
podman rm --all
podman rmi --all

########
# Set up xfs mount for overlay quota
########

# 1.004608 MB is 1,004,608 bytes. The container overhead is 4608 bytes (or 9 512 byte pages), so this allocates 1 MB of usable storage
PODMANBASE="-s overlay --storage-opt overlay.size=1.004608M --root /tmp/podman_test/crio"
TMPDIR=/tmp/podman_test
mkdir  $TMPDIR
dd if=/dev/zero of=$TMPDIR/virtfs bs=1024 count=30720
device=$(losetup -f | tr -d '[:space:]')
losetup $device $TMPDIR/virtfs
mkfs.xfs $device
mount -t xfs -o prjquota $device $TMPDIR

########
# Expected to succeed
########
podman $PODMANBASE run --security-opt label=disable alpine sh -c 'touch file.txt && dd if=/dev/zero of=file.txt count=1048576 bs=1'
rc=$?
if [ $rc == 0 ];
then
    echo "Overlay test within limits passed"
else
    echo "Overlay test within limits failed"
fi

########
# Expected to fail
########
podman $PODMANBASE run --security-opt label=disable alpine sh -c 'touch file.txt && dd if=/dev/zero of=file.txt count=1048577 bs=1'
rc=$?
if [ $rc != 0 ];
then
    echo "Overlay test outside limits passed"
else
    echo "Overlay test outside limits failed"
fi

########
# Clean up Podman
########
podman rm --all
podman rmi --all
umount $TMPDIR -l
losetup -d $device
rm -rf /tmp/podman_test

########
# Prep for UserNamespace testing
# Thanks @marcov!
########
PODMAN_OPTS_VOLUMES="-v /tmp/voltest/vol-0:/mnt/vol-0 -v /tmp/voltest/vol-1000:/mnt/vol-1000 -v /tmp/voltest/vol-100000:/mnt/vol-100000 -v /tmp/voltest/vol-101000:/mnt/vol-101000"
PODMAN_OPTS="$PODMAN_OPTS_VOLUMES --rm"
PODMAN_ID_MAPS="--uidmap=0:100000:1000000 --gidmap=0:100000:1000000"

########
# Make directories for UserNamespace testing
########
mkdir -p /tmp/voltest/vol-0
mkdir -p /tmp/voltest/vol-1000
mkdir -p /tmp/voltest/vol-100000
mkdir -p /tmp/voltest/vol-101000
UIDGID=`/usr/bin/tr -cd "[:digit:]" <<< /tmp/voltest/vol-0`

chown $UIDGID:$UIDGID /tmp/voltest/vol-0
chown $UIDGID:$UIDGID /tmp/voltest/vol-1000
chown $UIDGID:$UIDGID /tmp/voltest/vol-100000
chown $UIDGID:$UIDGID /tmp/voltest/vol-101000

########
# Make run test script
########
FILE=./runtest.sh
/bin/cat <<EOM >$FILE
#!/usr/bin/env bash
ls -n /mnt
for i in $(find /mnt -mindepth 1 -type d); do
    touch "$i/foobar" 2>/dev/null;
    echo "create $i/foobar: $?";
    /bin/rm "$i/foobar" 2>/dev/null;
done;
exit 0
EOM
chmod +x $FILE

########
# Make Dockerfile
########
FILE=./Dockerfile
/bin/cat <<EOM >$FILE
FROM debian
ADD ./runtest.sh /runtest.sh
EOM
chmod +x $FILE

########
# Build container
########
podman build -t usernamespace -f ./Dockerfile .

########
# Run the tests for UserNamespaces
########
echo "Run as root with no user NS"
podman run $PODMAN_OPTS usernamespace /bin/bash runtest.sh
echo ""

echo "Run as user 1000 with no user NS"
podman run --user=1000 $PODMAN_OPTS usernamespace /bin/bash /runtest.sh
echo ""

echo "Run as root with user NS "
podman run $PODMAN_ID_MAPS $PODMAN_OPTS usernamespace /bin/bash /runtest.sh
echo ""

echo "Run as user 1000 with user NS "
podman run --user=1000 $PODMAN_ID_MAPS $PODMAN_OPTS usernamespace /bin/bash /runtest.sh
echo ""

########
# Clean up Podman
########
podman rm --all
podman rmi --all
rm -f ./runtest.sh
rm -rf /tmp/voltest
rm -f ./Dockerfile

########
# Build Dockerfiles for OnBuild Test
# (Thanks @clcollins!)
########
FILE=./Dockerfile
/bin/cat <<EOM >$FILE
FROM alpine
RUN touch /foo
ONBUILD RUN touch /bar
EOM
chmod +x $FILE

FILE=./Dockerfile-2
/bin/cat <<EOM >$FILE
FROM onbuild-image
RUN touch /baz
EOM
chmod +x $FILE

########
# Build with Dockerfiles
########
podman build -f ./Dockerfile --format=docker -t onbuild-image .
podman build -f ./Dockerfile-2 --format=docker -t result-image .

########
# Check for /bar /baz and /foo files
########
podman run --network=host result-image ls -alF /bar /baz /foo

########
# Clean up Podman
########
podman rm --all
podman rmi --all
rm ./Dockerfile*

########
# Build Dockerfile for WhaleSays test
########
FILE=./Dockerfile
/bin/cat <<EOM >$FILE
FROM docker/whalesay:latest
RUN apt-get -y update && apt-get install -y fortunes
CMD /usr/games/fortune -a | cowsay
EOM
chmod +x $FILE

########
# Build with the Dockerfile
########
podman build -f Dockerfile -t whale-says

########
# Run the container to see what the whale says
########
podman run whale-says

########
# Clean up Podman and /tmp
########
podman rm --all
podman rmi --all
rm ./Dockerfile*
