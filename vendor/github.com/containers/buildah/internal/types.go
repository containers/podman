package internal

const (
	// BuildahExternalArtifactsDir is the pattern passed to os.MkdirTemp()
	// to generate a temporary directory which will be used to hold
	// external items which are downloaded for a build, typically a tarball
	// being used as an additional build context.
	BuildahExternalArtifactsDir = "buildah-external-artifacts"
)

// StageMountDetails holds the Stage/Image mountpoint returned by StageExecutor
// StageExecutor has ability to mount stages/images in current context and
// automatically clean them up.
type StageMountDetails struct {
	DidExecute bool   // tells if the stage which is being mounted was freshly executed or was part of older cache
	IsStage    bool   // true if the mountpoint is a temporary directory or a stage's rootfs, false if it's an image
	MountPoint string // mountpoint of the stage or image's root directory
}
