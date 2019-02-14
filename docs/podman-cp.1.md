% podman-cp(1)

## NAME
podman\-cp - Copy files/folders between a container and the local filesystem

## SYNOPSIS
**podman cp [CONTAINER:]SRC_PATH [CONTAINER:]DEST_PATH**

## DESCRIPTION
Copies the contents of **SRC_PATH** to the **DEST_PATH**. You can copy from the containers's filesystem to the local machine or the reverse, from the local filesystem to the container.

The CONTAINER can be a running or stopped container. The **SRC_PATH** or **DEST_PATH** can be a file or directory.

The **podman cp** command assumes container paths are relative to the container's / (root) directory.

This means supplying the initial forward slash is optional;

The command sees **compassionate_darwin:/tmp/foo/myfile.txt** and **compassionate_darwin:tmp/foo/myfile.txt** as identical.

Local machine paths can be an absolute or relative value.
The command interprets a local machine's relative paths as relative to the current working directory where **podman cp** is run.

Assuming a path separator of /, a first argument of **SRC_PATH** and second argument of **DEST_PATH**, the behavior is as follows:

**SRC_PATH** specifies a file
  - **DEST_PATH** does not exist
	- the file is saved to a file created at **DEST_PATH**
  - **DEST_PATH** does not exist and ends with /
	- **DEST_PATH** is created as a directory and the file is copied into this directory using the basename from **SRC_PATH**
  - **DEST_PATH** exists and is a file
	- the destination is overwritten with the source file's contents
  - **DEST_PATH** exists and is a directory
	- the file is copied into this directory using the basename from **SRC_PATH**

**SRC_PATH** specifies a directory
  - **DEST_PATH** does not exist
	- **DEST_PATH** is created as a directory and the contents of the source directory are copied into this directory
  - **DEST_PATH** exists and is a file
	- Error condition: cannot copy a directory to a file
  - **DEST_PATH** exists and is a directory
	- **SRC_PATH** ends with /
		- the source directory is copied into this directory
	- **SRC_PATH** ends with /. (that is: slash followed by dot)
		- the content of the source directory is copied into this directory

The command requires **SRC_PATH** and **DEST_PATH** to exist according to the above rules.

If **SRC_PATH** is local and is a symbolic link, the symbolic target, is copied by default.

A colon (:) is used as a delimiter between CONTAINER and its path.

You can also use : when specifying paths to a **SRC_PATH** or **DEST_PATH** on a local machine, for example, `file:name.txt`.

If you use a : in a local machine path, you must be explicit with a relative or absolute path, for example:
	`/path/to/file:name.txt` or `./file:name.txt`


## ALTERNATIVES

Podman has much stronger capabilities than just `podman cp` to achieve copy files between host and container.

Using standard podman-mount and podman-umount takes advantage of the entire linux tool chain, rather
then just cp.

If a user wants to copy contents out of a container or into a container, they can execute a few simple commands.

You can copy from the container's file system to the local machine or the reverse, from the local filesystem to the container.

If you want to copy the /etc/foobar directory out of a container and onto /tmp on the host, you could execute the following commands:

	mnt=$(podman mount CONTAINERID)
	cp -R ${mnt}/etc/foobar /tmp
	podman umount CONTAINERID

If you want to untar a tar ball into a container, you can execute these commands:

	mnt=$(podman mount CONTAINERID)
	tar xf content.tgz -C ${mnt}
	podman umount CONTAINERID

One last example, if you want to install a package into a container that
does not have dnf installed, you could execute something like:

	mnt=$(podman mount CONTAINERID)
	dnf install --installroot=${mnt} httpd
	chroot ${mnt} rm -rf /var/log/dnf /var/cache/dnf
	podman umount CONTAINERID

This shows that using `podman mount` and `podman umount` you can use all of the
standard linux tools for moving files into and out of containers, not just
the cp command.

## EXAMPLE

podman cp /myapp/app.conf containerID:/myapp/app.conf

podman cp /home/myuser/myfiles.tar containerID:/tmp

podman cp containerID:/myapp/ /myapp/

podman cp containerID:/home/myuser/. /home/myuser/

## SEE ALSO
podman(1), podman-mount(1), podman-umount(1)
