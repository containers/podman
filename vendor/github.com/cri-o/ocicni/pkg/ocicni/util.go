package ocicni

// newNSManager initializes a new namespace manager, which is a platform dependent struct.
func newNSManager() (*nsManager, error) {
	nsm := &nsManager{}
	err := nsm.init()
	return nsm, err
}
