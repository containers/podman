package retry

import (
	"syscall"
)

func shouldRestartPlatform(e error) bool {
	return e == syscall.ERESTART
}
