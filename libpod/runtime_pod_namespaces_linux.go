// +build linux

package libpod

import (
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/mount"
	"github.com/docker/docker/pkg/symlink"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"k8s.io/release/pkg/command"
)

func (r *Runtime) pinNamespaces(p *Pod) error {
	logrus.Debugf("pinning namespaces")

	// TODO: make those configurable
	namespacesDir := "/var/run/libpod/ns"
	pinns := "pinns"

	pinDir := filepath.Join(namespacesDir, p.ID())
	if err := os.MkdirAll(pinDir, 0755); err != nil {
		return errors.Wrap(err, "unable to create namespaces dir")
	}

	args := []string{"--dir", pinDir}
	// Not supported by cri-o/pinns yet, but by pinns.rs
	/* if p.config.UsePodCgroup {
		args = append(args, "--cgroup")
	} */
	if p.config.UsePodIPC {
		args = append(args, "--ipc")
	}
	if p.config.UsePodNet {
		args = append(args, "--net")
	}
	// Not supported by cri-o/pinns yet, but by pinns.rs
	/* if p.config.UsePodPID {
		args = append(args, "--pid")
	} */
	if p.config.UsePodUTS {
		args = append(args, "--uts")
	}

	if err := command.New(pinns, args...).RunSilentSuccess(); err != nil {
		rmAllErr := os.RemoveAll(pinDir)
		return errors.Wrapf(
			errors.Wrap(rmAllErr, err.Error()), "unable to pin namespaces",
		)
	}

	p.state.PinnedNamespacesPath = pinDir
	return nil
}

func (r *Runtime) unpinNamespaces(pod *Pod) error {
	err := filepath.Walk(pod.state.PinnedNamespacesPath,
		func(p string, info os.FileInfo, err error) error {
			if p == pod.state.PinnedNamespacesPath {
				return nil
			}
			if err != nil {
				return err
			}

			fp, err := symlink.FollowSymlinkInScope(p, "/")
			if err != nil {
				return errors.Wrapf(err, "unable to resolve symlink to %s", p)
			}

			if mounted, err := mount.Mounted(fp); err == nil && mounted {
				if err := unix.Unmount(fp, unix.MNT_DETACH); err != nil {
					return errors.Wrapf(err, "unable to unmount %s", p)
				}
			}

			return errors.Wrapf(
				os.RemoveAll(p), "removing namespace path %s", p,
			)
		})
	if err != nil {
		return errors.Wrapf(err, "unable to remove pinned namespaces")
	}
	return os.RemoveAll(pod.state.PinnedNamespacesPath)
}
