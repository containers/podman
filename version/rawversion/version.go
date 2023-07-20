package rawversion

// RawVersion is the raw version string.
//
// This indirection is needed to prevent semver packages from bloating
// Quadlet's binary size.
//
// NOTE: remember to bump the version at the top of the top-level README.md
// file when this is bumped.
const RawVersion = "4.6.0"
