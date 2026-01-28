//go:build linux
// +build linux

package libpod

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

func openUnixSocket(path string) (*net.UnixConn, error) {
	fd, err := unix.Open(path, unix.O_PATH, 0)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	return net.DialUnix("unixpacket", nil, &net.UnixAddr{Name: fmt.Sprintf("/proc/self/fd/%d", fd), Net: "unixpacket"})
}

// Attach to the given container.
// NOTE: This function is defined in oci_conmon_attach_common.go - removed duplicate

// Attach to the given container's exec session
// attachFd and startFd must be open file descriptors
// attachFd must be the output side of the fd. attachFd is used for two things:
//  conmon will first send a nonce value across the pipe indicating it has set up its side of the console socket
//    this ensures attachToExec gets all of the output of the called process
//  conmon will then send the exit code of the exec process, or an error in the exec session
// startFd must be the input side of the fd.
// newSize resizes the tty to this size before the process is started, must be nil if the exec session has no tty
//   conmon will wait to start the exec session until the parent process has set up the console socket.
//   Once attachToExec successfully attaches to the console socket, the child conmon process responsible for calling runtime exec
//     will read from the output side of start fd, thus learning to start the child process.
// Thus, the order goes as follow:
// 1. conmon parent process sets up its console socket. sends on attachFd
// 2. attachToExec attaches to the console socket after reading on attachFd and resizes the tty
// 3. child waits on startFd for attachToExec to attach to said console socket
// 4. attachToExec sends on startFd, signalling it has attached to the socket and child is ready to go
// 5. child receives on startFd, runs the runtime exec command
