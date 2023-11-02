package define

type ImageFormat int64

const (
	Qcow ImageFormat = iota
	Vhdx
	Tar
	Raw
)

func (imf ImageFormat) Kind() string {
	switch imf {
	case Vhdx:
		return "vhdx"
	case Tar:
		return "tar"
	case Raw:
		return "raw"
	}
	return "qcow2"
}

func (imf ImageFormat) KindWithCompression() string {
	switch imf {
	case Vhdx:
		return "vhdx.zip"
	case Tar:
		return "tar.xz"
	case Raw:
		return "raw.gz"
	}
	return "qcow2.xz"
}
