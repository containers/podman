//go:build !linux
// +build !linux

package specgen

// FinishThrottleDevices cannot be called on non-linux OS' due to importing unix functions
func FinishThrottleDevices(s *SpecGenerator) error {
	return nil
}

// WeightDevices cannot be called on non-linux OS' due to importing unix functions
func WeightDevices(s *SpecGenerator) error {
	return nil
}
