// +build !ostree !cgo

package ostree

func OstreeSupport() bool {
	return false
}

func DeleteOSTree(repoLocation, id string) error {
	return nil
}

func CreateOSTreeRepository(repoLocation string, rootUID int, rootGID int) error {
	return nil
}

func ConvertToOSTree(repoLocation, root, id string) error {
	return nil
}
