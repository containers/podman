package internal

// Types is internal packages are suspected to change with releases avoid using these outside of buildah

// StageMountDetails holds the Stage/Image mountpoint returned by StageExecutor
// StageExecutor has ability to mount stages/images in current context and
// automatically clean them up.
type StageMountDetails struct {
	IsStage    bool   // tells if mountpoint returned from stage executor is stage or image
	MountPoint string // mountpoint of stage/image
}
