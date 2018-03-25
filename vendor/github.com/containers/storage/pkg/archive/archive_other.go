// +build !linux

package archive

func getWhiteoutConverter(format WhiteoutFormat, data interface{}) tarWhiteoutConverter {
	return nil
}
