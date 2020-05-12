package tunnel

import (
	"context"
)

// Image-related runtime using an ssh-tunnel to utilize Podman service
type ImageEngine struct {
	ClientCxt context.Context
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type ContainerEngine struct {
	ClientCxt context.Context
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type SystemEngine struct {
	ClientCxt context.Context
}
