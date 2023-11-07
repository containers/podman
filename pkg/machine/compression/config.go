package compression

import "strings"

type ImageCompression int64

const (
	Xz ImageCompression = iota
	Zip
	Gz
	Bz2
)

func KindFromFile(path string) ImageCompression {
	switch {
	case strings.HasSuffix(path, Bz2.String()):
		return Bz2
	case strings.HasSuffix(path, Gz.String()):
		return Gz
	case strings.HasSuffix(path, Zip.String()):
		return Zip
	}
	return Xz
}

func (c ImageCompression) String() string {
	switch c {
	case Gz:
		return "gz"
	case Zip:
		return "zip"
	case Bz2:
		return "bz2"
	}
	return "xz"
}
