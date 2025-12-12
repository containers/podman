package swag

type swagError string

const (
	// ErrYAML is an error raised by YAML utilities
	ErrYAML swagError = "yaml error"

	// ErrLoader is an error raised by the file loader utility
	ErrLoader swagError = "loader error"
)

func (e swagError) Error() string {
	return string(e)
}
