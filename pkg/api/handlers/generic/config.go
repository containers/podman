package generic

// ContainerCreateResponse is the response struct for creating a container
type ContainerCreateResponse struct {
	// ID of the container created
	Id string `json:"Id"`
	// Warnings during container creation
	Warnings []string `json:"Warnings"`
}
