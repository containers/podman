package images

import (
	"os"
)

func checkHardLink(fi os.FileInfo) (devino, bool) {
	return devino{}, false
}
