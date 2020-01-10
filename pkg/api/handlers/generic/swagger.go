package generic

// Create container
type swagCtrCreateResponse struct {
	// in:body
	Body struct {
		ContainerCreateResponse
	}
}

// Wait container
// swagger:response ContainerWaitResponse
type swagCtrWaitResponse struct {
	// in:body
	Body struct {
		// container exit code
		StatusCode int
		error      message
		Error      struct {
			Message string
		}
	}
}
