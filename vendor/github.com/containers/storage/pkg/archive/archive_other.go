// +build !linux

package archive

func getWhiteoutConverter(format WhiteoutFormat, data interface{}) tarWhiteoutConverter {
	return nil
}

func getFileOwner(path string) (uint32, uint32, uint32, error) {
	return 0, 0, 0, nil
}
