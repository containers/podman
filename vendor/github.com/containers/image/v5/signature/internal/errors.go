package internal

// InvalidSignatureError is returned when parsing an invalid signature.
// This is publicly visible as signature.InvalidSignatureError
type InvalidSignatureError struct {
	msg string
}

func (err InvalidSignatureError) Error() string {
	return err.msg
}

func NewInvalidSignatureError(msg string) InvalidSignatureError {
	return InvalidSignatureError{msg: msg}
}
