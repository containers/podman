#!/usr/bin/env bash
# test_podman_baseline.sh
# A script to be run at the command line with Podman installed.
# This should be run against a new kit to provide base level testing
# on a freshly installed machine with no images or container in
# play.  This currently needs to be run as root.
#
# Please leave the whale-says test as the last test in this script.
# It makes it easier to identify if the script has finished or not.
#
# To run this command:
#
# /bin/bash -v test_podman_baseline.sh -d # Install and then deinstall Docker
# /bin/bash -v test_podman_baseline.sh -n # Do not perform docker test
# /bin/bash -v test_podman_baseline.sh -e # Stop on error
# /bin/bash -v test_podman_baseline.sh    # Continue on error
#

#######
# See if we want to stop on errors and/or install and then remove Docker.
#######
HOST_PORT="${HOST_PORT:-8080}"
showerror=0
installdocker=0
usedocker=1
while getopts "den" opt; do
    case "$opt" in
    d) installdocker=1
       ;;
    e) showerror=1
       ;;
    n) usedocker=0
       ;;
    esac
done

if [ "$installdocker" -eq 1 ] && [ "usedocker" -ne 0 ]
then
    echo "Script will install and then deinstall Docker."
fi

if [ "$showerror" -eq 1 ]
then
    echo "Script will stop on unexpected errors."
    set -e
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
ctrid=$(podman pull docker.io/library/redis:4-alpine3.8)
podman run $ctrid ls /

########
# Remove images and containers
########
podman rm --all
podman rmi --all

########
# Create Fedora based image
########
image=$(podman pull registry.fedoraproject.org/fedora:latest)
echo $image

########
# Run container and display contents in /etc
########
podman run --rm $image ls -alF /etc

########
# Test networking, bind mounting a file, stdin/stdout redirect
########
echo "Testing networking: ..."
port_test_failed=0
txt1="Hello, Podman"
echo "$txt1" > /tmp/hello.txt
podman run -d --name myweb -p "$HOST_PORT:80" -w /var/www -v /tmp/hello.txt:/var/www/index.txt busybox httpd -f -p 80
echo "$txt1" | podman exec -i myweb sh -c "cat > /var/www/index2.txt"
txt2=$( podman exec myweb cat /var/www/index2.txt )
[ "x$txt1" == "x$txt2" ] && echo "PASS1" || { echo "FAIL1"; port_test_failed=1; }
txt2=$( podman run --rm --net host busybox wget -qO - http://localhost:$HOST_PORT/index.txt )
[ "x$txt1" == "x$txt2" ] && echo "PASS2" || { echo "FAIL2"; port_test_failed=1; }
txt2=$( podman run --rm --net host busybox wget -qO - http://localhost:$HOST_PORT/index2.txt )
[ "x$txt1" == "x$txt2" ] && echo "PASS3" || { echo "FAIL3"; port_test_failed=1; }
# podman run --rm --net container:myweb --add-host myweb:127.0.0.1 busybox wget -qO - http://myweb/index.txt
rm /tmp/hello.txt
podman stop myweb
podman rm myweb
[ "0$port_test_failed" -eq 1 ] && [ "0$showerror" -eq 1 ] && {
  echo "networking test failed";
  exit -1;
}


########
# pull and run many containers in parallel, test locks ..etc.
########
prun_test_failed=0
podman rmi docker.io/library/busybox:latest > /dev/null || :
for i in `seq 10`
do ( podman run -d --name b$i docker.io/library/busybox:latest busybox httpd -f -p 80 )&
done
echo -e "\nwaiting for creation...\n"
wait
echo -e "\ndone\n"
# assert we have 10 running containers
count=$( podman ps -q  | wc -l )
[ "x$count" == "x10" ] && echo "PASS" || { echo "FAIL, expecting 10 found $count"; prun_test_failed=1; }
[ "0$prun_test_failed" -eq 1 ] && [ "0$showerror" -eq 1 ] && {
  echo "was expecting 10 running containers";
  exit -1;
}

prun_test_failed=0
for i in `seq 10`; do ( podman stop -t=1 b$i; podman rm b$i )& done
echo -e "\nwaiting for deletion...\n"
wait
echo -e "\ndone\n"
# assert we have 0 running containers
count=$( podman ps -q  | wc -l )
[ "x$count" == "x0" ] && echo "PASS" || { echo "FAIL, expecting 0 found $count"; prun_test_failed=1; }
[ "0$prun_test_failed" -eq 1 ] && [ "0$showerror" -eq 1 ] && {
  echo "was expecting 0 running containers";
  exit -1;
}



########
# run many containers in parallel for an existing image, test locks ..etc.
########
prun_test_failed=0
podman pull docker.io/library/busybox:latest > /dev/null || :
for i in `seq 10`
do ( podman run -d --name c$i docker.io/library/busybox:latest busybox httpd -f -p 80 )&
done
echo -e "\nwaiting for creation...\n"
wait
echo -e "\ndone\n"
# assert we have 10 running containers
count=$( podman ps -q  | wc -l )
[ "x$count" == "x10" ] && echo "PASS" || { echo "FAIL, expecting 10 found $count"; prun_test_failed=1; }
[ "0$prun_test_failed" -eq 1 ] && [ "0$showerror" -eq 1 ] && {
  echo "was expecting 10 running containers";
  exit -1;
}


for i in `seq 10`; do ( podman stop -t=1 c$i; podman rm c$i )& done
echo -e "\nwaiting for deletion...\n"
wait
echo -e "\ndone\n"
# assert we have 0 running containers
count=$( podman ps -q  | wc -l )
[ "x$count" == "x0" ] && echo "PASS" || { echo "FAIL, expecting 0 found $count"; prun_test_failed=1; }
[ "0$prun_test_failed" -eq 1 ] && [ "0$showerror" -eq 1 ] && {
  echo "was expecting 0 running containers";
  exit -1;
}


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
podman run javaimage java -version

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
image=$(podman pull registry.fedoraproject.org/fedora:latest)
echo $image
podman run $image ls /

########
# Create shell script to test on
########
FILE=./runecho.sh
/bin/cat <<EOM >$FILE
#!/usr/bin/env bash
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

if [ "$usedocker" -ne 0 ]; then
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
PODMANBASE="--storage-driver overlay --storage-opt overlay.size=1.004608M --root /tmp/podman_test/crio"
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
podman $PODMANBASE run --security-opt label=disable docker.io/library/alpine:latest sh -c 'touch file.txt && dd if=/dev/zero of=file.txt count=1048576 bs=1'
rc=$?
if [ $rc == 0 ];
then
    echo "Overlay test within limits passed"
else
    echo "Overlay test within limits failed"
fi

before=`xfs_quota -x -c 'report -N -p' $TMPDIR | grep -c ^#`
podman $PODMANBASE volume create -o o=noquota test-no-quota
after=`xfs_quota -x -c 'report -N -p' $TMPDIR | grep -c ^#`

if [ $before != $after ];
then
    echo "Test -o=noquota doesn't create a projid failed"
else
    echo "Test -o=noquota doesn't create a projid passed"
fi

before=`xfs_quota -x -c 'report -N -p' $TMPDIR | grep -c ^#`
podman $PODMANBASE volume create -o test-no-quota
after=`xfs_quota -x -c 'report -N -p' $TMPDIR | grep -c ^#`

if [ $before == $after ];
then
    echo "Test without -o=noquota creates a projid failed"
else
    echo "Test without -o=noquota creates a projid passed"
fi

########
# Expected to fail
########

if [ "$showerror" -ne 1 ]; then
    podman $PODMANBASE run --security-opt label=disable docker.io/library/alpine:latest sh -c 'touch file.txt && dd if=/dev/zero of=file.txt count=1048577 bs=1'
    rc=$?
    if [ $rc != 0 ];
    then
        echo "Overlay test outside limits passed"
    else
        echo "Overlay test outside limits failed"
    fi
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
FROM docker.io/library/debian:latest
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
FROM docker.io/library/alpine:latest
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
# Run AppArmor rootless tests
########
if aa-enabled >/dev/null && getent passwd 1000 >/dev/null; then
    # Expected to succeed
    sudo -u "#1000" podman run docker.io/library/alpine:latest echo hello
    rc=$?
    echo -n "rootless with no AppArmor profile "
    if [ $rc == 0 ]; then
        echo "passed"
    else
        echo "failed"
    fi

    # Expected to succeed
    sudo -u "#1000" podman run --security-opt apparmor=unconfined docker.io/library/alpine:latest echo hello
    rc=$?
    echo -n "rootless with unconfined AppArmor profile "
    if [ $rc == 0 ]; then
        echo "passed"
    else
        echo "failed"
    fi

    aaFile="/tmp/aaProfile"
    aaProfile="aa-demo-profile"
    cat > $aaFile << EOF
#include <tunables/global>
profile aa-demo-profile flags=(attach_disconnected,mediate_deleted) {
  #include <abstractions/base>
  deny mount,
  deny /sys/[^f]*/** wklx,
  deny /sys/f[^s]*/** wklx,
  deny /sys/fs/[^c]*/** wklx,
  deny /sys/fs/c[^g]*/** wklx,
  deny /sys/fs/cg[^r]*/** wklx,
  deny /sys/firmware/efi/efivars/** rwklx,
  deny /sys/kernel/security/** rwklx,
}
EOF

    apparmor_parser -Kr $aaFile

    #Expected to pass (as root)
    podman run --security-opt apparmor=$aaProfile docker.io/library/alpine:latest echo hello
    rc=$?
    echo -n "root with specified AppArmor profile: "
    if [ $rc == 0 ]; then
        echo "passed"
    else
        echo "failed"
    fi

    #Expected to pass (as root with --privileged).
    #Note that the profile should not be loaded letting the mount succeed.
    podman run --privileged docker.io/library/alpine:latest sh -c "mkdir tmp2; mount --bind tmp tmp2"
    rc=$?
    echo -n "root with specified AppArmor profile but --privileged: "
    if [ $rc == 0 ]; then
        echo "passed"
    else
        echo "failed"
    fi
    #Expected to fail (as rootless)
    sudo -u "#1000" podman run --security-opt apparmor=$aaProfile docker.io/library/alpine:latest echo hello
    rc=$?
    echo -n "rootless with specified AppArmor profile: "
    if [ $rc != 0 ]; then
        echo "passed"
    else
        echo "failed"
    fi

    ########
    # Clean up Podman and $aaFile
    ########
    apparmor_parser -R $aaFile
    podman rm --all
    podman rmi --all
    sudo -u "#1000" podman rm --all
    sudo -u "#1000" podman rmi --all
    rm -f $aaFile
fi

########
# Build Dockerfile for RUN with priv'd command test
########
FILE=./Dockerfile
/bin/cat <<EOM >$FILE
FROM alpine
RUN apk add nginx
EOM
chmod +x $FILE

########
# Build with the Dockerfile
########
podman build -f Dockerfile -t build-priv

########
# Cleanup
########
podman rm -a -f -t 0
podman rmi -a -f
rm ./Dockerfile

########
# Build Dockerfile for WhaleSays test
########
FILE=./Dockerfile
/bin/cat <<EOM >$FILE
FROM pharshal/whalesay:latest
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
# NOTE: Please leave the whale-says as the last test
# in this script.
########

########
# Clean up Podman and /tmp
########
podman rm --all
podman rmi --all
rm ./Dockerfile*
