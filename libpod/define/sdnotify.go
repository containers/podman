package define

import "fmt"

// Strings used for --sdnotify option to podman
const (
	SdNotifyModeContainer = "container"
	SdNotifyModeConmon    = "conmon"
	SdNotifyModeIgnore    = "ignore"
)

// ValidateSdNotifyMode validates the specified mode.
func ValidateSdNotifyMode(mode string) error {
	switch mode {
	case "", SdNotifyModeContainer, SdNotifyModeConmon, SdNotifyModeIgnore:
		return nil
	default:
		return fmt.Errorf("%w: invalid sdnotify value %q: must be %s, %s or %s", ErrInvalidArg, mode, SdNotifyModeContainer, SdNotifyModeConmon, SdNotifyModeIgnore)
	}
}
