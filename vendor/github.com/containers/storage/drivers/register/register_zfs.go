//go:build (!exclude_graphdriver_zfs && linux) || (!exclude_graphdriver_zfs && freebsd) || solaris
// +build !exclude_graphdriver_zfs,linux !exclude_graphdriver_zfs,freebsd solaris

package register

import (
	// register the zfs driver
	_ "github.com/containers/storage/drivers/zfs"
)
