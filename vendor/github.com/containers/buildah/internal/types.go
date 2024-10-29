package internal

const (
	// Temp directory which stores external artifacts which are download for a build.
	// Example: tar files from external sources.
	BuildahExternalArtifactsDir = "buildah-external-artifacts"
)

// Types is internal packages are suspected to change with releases avoid using these outside of buildah

// StageMountDetails holds the Stage/Image mountpoint returned by StageExecutor
// StageExecutor has ability to mount stages/images in current context and
// automatically clean them up.
type StageMountDetails struct {
	DidExecute bool   // tells if the stage which is being mounted was freshly executed or was part of older cache
	IsStage    bool   // tells if mountpoint returned from stage executor is stage or image
	MountPoint string // mountpoint of stage/image
}
