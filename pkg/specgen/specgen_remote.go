//go:build remote
// +build remote

package specgen

// Empty stub we do not use any libimage on the remote client,
// this drastically decreases binary size for the remote client.
type cacheLibImage struct{}
