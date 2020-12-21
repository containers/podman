package tunnel

import (
	"context"
)

// Image-related runtime using an ssh-tunnel to utilize Podman service
type ImageEngine struct {
	ClientCtx context.Context
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type ContainerEngine struct {
	ClientCtx context.Context
}

// Container-related runtime using an ssh-tunnel to utilize Podman service
type SystemEngine struct {
	ClientCtx context.Context
}
