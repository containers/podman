# Using podman-machine on MacOS (Apple silicon and x86_64)

## Setup

You must obtain a compressed tarball that contains the following:
* a qcow image
* a podman binary
* a gvproxy binary

You must also have installed brew prior to following this process.  See https://brew.sh/ for
installation instructions.

Note: If your user has admin rights, you can ignore the use of `sudo` in these instructions.


1. Install qemu from brew to obtain the required runtime dependencies.

   ```
   brew install qemu
   ```

2. If you are running MacOS on the Intel architecture, you can skip to step 8.
3. Uninstall the brew package

   ```
   brew uninstall qemu
   ```

4. Get upstream qemu source code.

   ```
   git clone https://github.com/qemu/qemu
   ```

5. Apply patches that have not been merged into upstream qemu.

   ```
   cd qemu
   git config user.name "YOUR_NAME"
   git config user.email johndoe@example.com
   git checkout v5.2.0
   curl https://patchwork.kernel.org/series/418581/mbox/ | git am --exclude=MAINTAINERS
   curl -L https://gist.github.com/citruz/9896cd6fb63288ac95f81716756cb9aa/raw/2d613e9a003b28dfe688f33055706d3873025a40/xcode-12-4.patch | git apply -
   ```

6. Install qemu build dependencies

   ```
   brew install libffi gettext pkg-config autoconf automake pixman ninja make
   ```

7. Configure, compile, and install qemu
   ```
   mkdir build
   cd build
   ../configure --target-list=aarch64-softmmu --disable-gnutls
   gmake -j8
   sudo gmake install
   ```


8. Uncompress and place provided binaries into filesystem

   **Note**: In the following instructions, you need to know the name of the compressed file
that you were given.  It will be used in two of the steps below.

   ```
   cd ~
   tar xvf `compressed_file_ending_in_xz`
   sudo cp -v `unpacked_directory`/{gvproxy,podman} /usr/local/bin
   ```

9. Sign all binaries

   If you have a Mac with Apple Silicon, issue the following command:
   ```
   sudo codesign --entitlements ~/qemu/accel/hvf/entitlements.plist --force -s - /usr/local/bin/qemu-* /usr/local/bin/gvproxy /usr/local/bin/podman
   ```

   If you have a Mac with an Intel processor, issue the following command:

   ```
   echo '<?xml version="1.0" encoding="utf-8"?>
    <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
    <plist version="1.0"> <dict> <key>com.apple.security.hypervisor</key> <true/> </dict> </plist>
   '  > ~/entitlements.plist
   sudo codesign --entitlements ~/entitlements.plist --force -s - /usr/local/bin/qemu-* /usr/local/bin/gvproxy /usr/local/bin/podman
   ```


## Test podman

1. podman machine init --image-path /path/to/image --cpus 2
2. podman machine start
3. podman images
4. git clone http://github.com/baude/alpine_nginx && cd alpine_nginx
5. podman build -t alpine_nginx .
4. podman run -dt -p 9999:80 alpine_nginx
5. curl http://localhost:9999
