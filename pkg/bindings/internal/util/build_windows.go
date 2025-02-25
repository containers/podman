package util

import (
	"os"
)

func CheckHardLink(fi os.FileInfo) (Devino, bool) {
	return Devino{}, false
}
