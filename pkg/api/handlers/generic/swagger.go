package generic

// Create container
// swagger:response ContainerCreateResponse
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
		Error      struct {
			Message string
		}
	}
}
