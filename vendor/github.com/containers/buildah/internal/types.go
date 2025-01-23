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
	DidExecute               bool   // true if this is a freshly-executed stage, or an image, possibly from a non-local cache
	IsStage                  bool   // true if the mountpoint is a stage's rootfs
	IsImage                  bool   // true if the mountpoint is an image's rootfs
	IsAdditionalBuildContext bool   // true if the mountpoint is an additional build context
	MountPoint               string // mountpoint of the stage or image's root directory or path of the additional build context
}
