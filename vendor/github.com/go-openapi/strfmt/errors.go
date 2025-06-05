package strfmt

type strfmtError string

// ErrFormat is an error raised by the strfmt package
const ErrFormat strfmtError = "format error"

func (e strfmtError) Error() string {
	return string(e)
}
