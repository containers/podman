package rest

type ServiceScheme int

const (
	TCP ServiceScheme = iota
	Unix
	None
	HTTP
)
