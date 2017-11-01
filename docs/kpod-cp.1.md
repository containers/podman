% kpod(1) kpod-cp - Copy content between container's file system and the host
% Dan Walsh
# kpod-cp "1" "August 2017" "kpod"

## NAME
kpod-cp - Copy files/folders between a container and the local filesystem.

## Description
We chose not to implement the `cp` feature in `kpod` even though the upstream Docker
project has it. We have a much stronger capability.  Using standard kpod-mount
and kpod-umount, we can take advantage of the entire linux tool chain, rather
then just cp.

If a user wants to copy contents out of a container or into a container, they
can execute a few simple commands.

You can copy from the container's file system to the local machine or the
reverse, from the local filesystem to the container. 

If you want to copy the /etc/foobar directory out of a container and onto /tmp
on the host, you could execute the following commands:

	mnt=$(kpod mount CONTAINERID)
	cp -R ${mnt}/etc/foobar /tmp
	kpod umount CONTAINERID

If you want to untar a tar ball into a container, you can execute these commands:

	mnt=$(kpod mount CONTAINERID)
	tar xf content.tgz -C ${mnt}
	kpod umount CONTAINERID

One last example, if you want to install a package into a container that
does not have dnf installed, you could execute something like:

	mnt=$(kpod mount CONTAINERID)
	dnf install --installroot=${mnt} httpd
	chroot ${mnt} rm -rf /var/log/dnf /var/cache/dnf
	kpod umount CONTAINERID

This shows that using `kpod mount` and `kpod umount` you can use all of the
standard linux tools for moving files into and out of containers, not just
the cp command.

## SEE ALSO
kpod(1), kpod-mount(1), kpod-umount(1)
