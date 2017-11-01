package ocicni

type cniNoOp struct {
}

func (noop *cniNoOp) Name() string {
	return "CNINoOp"
}

func (noop *cniNoOp) SetUpPod(network PodNetwork) error {
	return nil
}

func (noop *cniNoOp) TearDownPod(network PodNetwork) error {
	return nil
}

func (noop *cniNoOp) GetPodNetworkStatus(network PodNetwork) (string, error) {
	return "", nil
}

func (noop *cniNoOp) Status() error {
	return nil
}
