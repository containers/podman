package quadlet

var (
	// Key: Extension
	// Value: Processing order for resource naming dependencies
	SupportedExtensions = map[string]int{
		".container": 4,
		".volume":    2,
		".kube":      4,
		".network":   2,
		".image":     1,
		".build":     3,
		".pod":       5,
	}
)
