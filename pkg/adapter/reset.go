// +build !remoteclient

package adapter

import (
	"context"
)

// Reset the container storage back to initial states.
// Removes all Pods, Containers, Images and Volumes.
func (r *LocalRuntime) Reset() error {
	return r.Runtime.Reset(context.TODO())
}
