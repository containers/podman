% podman-cp(1)

## NAME
podman\-cp - Copy files/folders between a container and the local filesystem

## SYNOPSIS
**podman cp** [*options*] [*container*:]*src_path* [*container*:]*dest_path*

**podman container cp** [*options*] [*container*:]*src_path* [*container*:]*dest_path*

## DESCRIPTION
Copies the contents of **src_path** to the **dest_path**. You can copy from the container's filesystem to the local machine or the reverse, from the local filesystem to the container.
If - is specified for either the SRC_PATH or DEST_PATH, you can also stream a tar archive from STDIN or to STDOUT.

The CONTAINER can be a running or stopped container. The **src_path** or **dest_path** can be a file or directory.

The **podman cp** command assumes container paths are relative to the container's / (root) directory.

This means supplying the initial forward slash is optional;

The command sees **compassionate_darwin:/tmp/foo/myfile.txt** and **compassionate_darwin:tmp/foo/myfile.txt** as identical.

Local machine paths can be an absolute or relative value.
The command interprets a local machine's relative paths as relative to the current working directory where **podman cp** is run.

Assuming a path separator of /, a first argument of **src_path** and second argument of **dest_path**, the behavior is as follows:

**src_path** specifies a file
  - **dest_path** does not exist
	- the file is saved to a file created at **dest_path**
  - **dest_path** does not exist and ends with /
	- **dest_path** is created as a directory and the file is copied into this directory using the basename from **src_path**
  - **dest_path** exists and is a file
	- the destination is overwritten with the source file's contents
  - **dest_path** exists and is a directory
	- the file is copied into this directory using the basename from **src_path**

**src_path** specifies a directory
  - **dest_path** does not exist
	- **dest_path** is created as a directory and the contents of the source directory are copied into this directory
  - **dest_path** exists and is a file
	- Error condition: cannot copy a directory to a file
  - **dest_path** exists and is a directory
	- **src_path** ends with /
		- the source directory is copied into this directory
	- **src_path** ends with /. (that is: slash followed by dot)
		- the content of the source directory is copied into this directory

The command requires **src_path** and **dest_path** to exist according to the above rules.

If **src_path** is local and is a symbolic link, the symbolic target, is copied by default.

A colon (:) is used as a delimiter between CONTAINER and its path.

You can also use : when specifying paths to a **src_path** or **dest_path** on a local machine, for example, `file:name.txt`.

If you use a : in a local machine path, you must be explicit with a relative or absolute path, for example:
	`/path/to/file:name.txt` or `./file:name.txt`

## OPTIONS

**--extract**

Extract the tar file into the destination directory. If the destination directory is not provided, extract the tar file into the root directory.

**--pause**

Pause the container while copying into it to avoid potential security issues around symlinks. Defaults to *false*.

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

podman cp --extract /home/myuser/myfiles.tar.gz containerID:/myfiles

podman cp - containerID:/myfiles.tar.gz < myfiles.tar.gz

## SEE ALSO
podman(1), podman-mount(1), podman-umount(1)
