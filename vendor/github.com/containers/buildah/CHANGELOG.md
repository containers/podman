![buildah logo](https://cdn.rawgit.com/containers/buildah/master/logos/buildah-logo_large.png)

# Changelog

## v1.14.9 (2020-05-11)
    Bump github.com/containers/common to 0.8.4

## v1.14.8 (2020-04-09)
    Run (make vendor)
    Run (make -C tests/tools vendor)
    Run (go mod tidy) before (go mod vendor) again
    Fix (make vendor)
    Bump validation
    Bump back to v1.15.0-dev

## v1.14.7 (2020-04-07)
    Bump github.com/containers/image/v5 from 5.3.1 to 5.4.3
    make vendor: run `tidy` after `vendor`
    Do not skip the directory when the ignore pattern matches
    Bump github.com/containers/common from 0.7.0 to 0.8.1
    Downgrade siruspen/logrus from 1.4.2
    Fix errorf conventions
    dockerignore tests : remove symlinks, rework
    Bump back to v1.15.0-dev

## v1.14.6 (2020-04-02)
    bud.bats - cleanup, refactoring
    vendor in latest containers/storage 1.18.0 and containers/common v0.7.0
    Bump github.com/spf13/cobra from 0.0.6 to 0.0.7
    Bump github.com/containers/storage from 1.16.5 to 1.17.0
    Bump github.com/containers/image/v5 from 5.2.1 to 5.3.1
    Fix Amazon install step
    Bump back to v1.15.0-dev
    Fix bud-build-arg-cache test
    Make image history work correctly with new args handling
    Don't add args to the RUN environment from the Builder
    Update github.com/openshift/imagebuilder to v1.1.4
    Add .swp files to .gitignore

## v1.14.5 (2020-03-26)
    revert #2246 FIPS mode change
    Bump back to v1.15.0-dev
    image with dup layers: we now have one on quay
    digest test : make more robust

## v1.14.4 (2020-03-25)
    Fix fips-mode check for RHEL8 boxes
    Fix potential CVE in tarfile w/ symlink
    Fix .dockerignore with globs and ! commands
    update install steps for Amazon Linux 2
    Bump github.com/openshift/imagebuilder from 1.1.2 to 1.1.3
    Add comment for RUN command in volume ownership test
    Run stat command directly for volume ownership test
    vendor in containers/common v0.6.1
    Cleanup go.sum
    Bump back to v1.15.0-dev

## v1.14.3 (2020-03-17)
    Update containers/storage to v1.16.5
    Bump github.com/containers/storage from 1.16.2 to 1.16.4
    Bump github.com/openshift/imagebuilder from 1.1.1 to 1.1.2
    Update github.com/openshift/imagebuilder vendoring
    Update unshare man page to fix script example
    Fix compilation errors on non linux platforms
    Bump containers/common and opencontainers/selinux versions
    Add tests for volume ownership
    Preserve volume uid and gid through subsequent commands
    Fix FORWARD_NULL errors found by Coverity
    Bump github.com/containers/storage from 1.16.1 to 1.16.2
    Fix errors found by codespell
    Bump back to v1.15.0-dev
    Add Pull Request Template

## v1.14.2 (2020-03-03)
    Add Buildah pull request template
    Bump to containers/storage v1.16.1
    run_linux: fix tight loop if file is not pollable
    Bump github.com/opencontainers/selinux from 1.3.2 to 1.3.3
    Bump github.com/containers/common from 0.4.1 to 0.4.2
    Bump back to v1.15.0-dev
    Add Containerfile to build a versioned stable image on quay.io

## v1.14.1 (2020-02-27)
    Search for local runtime per values in containers.conf
    Set correct ownership on working directory
    BATS : in teardown, umount stale mounts
    Bump github.com/spf13/cobra from 0.0.5 to 0.0.6
    Bump github.com/fsouza/go-dockerclient from 1.6.1 to 1.6.3
    Bump github.com/stretchr/testify from 1.4.0 to 1.5.1
    Replace unix with syscall to allow vendoring into libpod
    Update to containers/common v0.4.1
    Improve remote manifest retrieval
    Fix minor spelling errors in containertools README
    Clear the right variable in buildahimage
    Correct a couple of incorrect format specifiers
    Update to containers/common v0.3.0
    manifest push --format: force an image type, not a list type
    run: adjust the order in which elements are added to $PATH
    getDateAndDigestAndSize(): handle creation time not being set
    Bump github.com/containers/common from 0.2.0 to 0.2.1
    include installation steps for CentOS 8 and Stream
    include installation steps for CentOS7 and forks
    Adjust Ubuntu install info to also work on Pop!_OS
    Make the commit id clear like Docker
    Show error on copied file above context directory in build
    Bump github.com/containers/image/v5 from 5.2.0 to 5.2.1
    pull/from/commit/push: retry on most failures
    Makefile: fix install.cni.sudo
    Repair buildah so it can use containers.conf on the server side
    Bump github.com/mattn/go-shellwords from 1.0.9 to 1.0.10
    Bump github.com/fsouza/go-dockerclient from 1.6.0 to 1.6.1
    Fixing formatting & build instructions
    Add Code of Conduct
    Bors: Fix no. req. github reviews
    Cirrus+Bors: Simplify temp branch skipping
    Bors-ng: Add documentation and status-icon
    Bump github.com/onsi/ginkgo from 1.11.0 to 1.12.0
    fix XDG_RUNTIME_DIR for authfile
    Cirrus: Disable F29 testing
    Cirrus: Add jq package
    Cirrus: Fix lint + validation using wrong epoch
    Stop using fedorproject registry
    Bors: Workaround ineffective required statuses
    Bors: Enable app + Disable Travis
    Cirrus: Add standardized log-collection
    Cirrus: Improve automated lint + validation
    Allow passing options to golangci-lint
    Cirrus: Fixes from review feedback
    Cirrus: Temporarily ignore VM testing failures
    Cirrus: Migrate off papr + implement VM testing
    Cirrus: Update packages + fixes for get_ci_vm.sh
    Show validation command-line
    Skip overlay test w/ vfs driver
    use alpine, not centos, for various tests
    Flake handling: cache and prefetch images
    Bump to v1.15.0-dev

## v1.14.0 (2020-02-05)
    bump github.com/mtrmac/gpgme
    Update containers/common to v0.1.4
    manifest push: add --format option
    Bump github.com/onsi/gomega from 1.8.1 to 1.9.0
    vendor github.com/containers/image/v5@v5.2.0
    info test: deal with random key order
    Bump back to v1.14.0-dev

## v1.13.2 (2020-01-29)
    sign.bats: set GPG_TTY=/dev/null
    Fix parse_unsupported.go
    getDateAndDigestAndSize(): use manifest.Digest
    Bump github.com/opencontainers/selinux from 1.3.0 to 1.3.1
    Bump github.com/containers/common from 0.1.0 to 0.1.2
    Touch up os/arch doc
    chroot: handle slightly broken seccomp defaults
    buildahimage: specify fuse-overlayfs mount options
    Bump github.com/mattn/go-shellwords from 1.0.7 to 1.0.9
    copy.bats: make sure we detect failures due to missing source
    parse: don't complain about not being able to rename something to itself
    Makefile: use a $(GO_TEST) macro, fix a typo
    manifests: unit test fix
    Fix build for 32bit platforms
    Allow users to set OS and architecture on bud
    Fix COPY in containerfile with envvar
    Bump c/storage to v1.15.7
    add --sign-by to bud/commit/push, --remove-signatures for pull/push
    Remove cut/paste error in CHANGELOG.md
    Update vendor of containers/common to v0.1.0
    update install instructions for Debian, Raspbian and Ubuntu
    Add support for containers.conf
    Bump back to v1.14.0-dev

## v1.13.1 (2020-01-14)
    Bump github.com/containers/common from 0.0.5 to 0.0.7
    Bump github.com/onsi/ginkgo from 1.10.3 to 1.11.0
    Bump github.com/pkg/errors from 0.8.1 to 0.9.0
    Bump github.com/onsi/gomega from 1.7.1 to 1.8.1
    Add codespell support
    copyFileWithTar: close source files at the right time
    copy: don't digest files that we ignore
    Check for .dockerignore specifically
    Travis: rm go 1.12.x
    Don't setup excludes, if their is only one pattern to match
    set HOME env to /root on chroot-isolation by default
    docs: fix references to containers-*.5
    update openshift/api
    fix bug Add check .dockerignore COPY file
    buildah bud --volume: run from tmpdir, not source dir
    Fix imageNamePrefix to give consistent names in buildah-from
    cpp: use -traditional and -undef flags
    Fix image reference in tutorial 4
    discard outputs coming from onbuild command on buildah-from --quiet
    make --format columnizing consistent with buildah images
    Bump to v1.14.0-dev

## v1.13.0 (2019-12-27)
    Bump to c/storage v1.15.5
    Update container/storage to v1.15.4
    Fix option handling for volumes in build
    Rework overlay pkg for use with libpod
    Fix buildahimage builds for buildah
    Add support for FIPS-Mode backends
    Set the TMPDIR for pulling/pushing image to $TMPDIR
    WIP: safer test for pull --all-tags
    BATS major cleanup: blobcache.bats: refactor
    BATS major cleanup: part 4: manual stuff
    BATS major cleanup, step 3: yet more run_buildah
    BATS major cleanup, part 2: use more run_buildah
    BATS major cleanup, part 1: log-level
    Bump github.com/containers/image/v5 from 5.0.0 to 5.1.0
    Bump github.com/containers/common from 0.0.3 to 0.0.5
    Bump to v1.13.0-dev

## v1.12.0 (2019-12-13)
    Allow ADD to use http src
    Bump to c/storage v.1.15.3
    install.md: update golang dependency
    imgtype: reset storage opts if driver overridden
    Start using containers/common
    overlay.bats typo: fuse-overlays should be fuse-overlayfs
    chroot: Unmount with MNT_DETACH instead of UnmountMountpoints()
    bind: don't complain about missing mountpoints
    imgtype: check earlier for expected manifest type
    Vendor containers/storage fix
    Vendor containers/storage v1.15.1
    Add history names support
    PR takeover of #1966
    Tests: Add inspect test check steps
    Tests: Add container name and id check in containers test steps
    Test: Get permission in add test
    Tests: Add a test for tag by id
    Tests: Add test cases for push test
    Tests: Add image digest test
    Tests: Add some buildah from tests
    Tests: Add two commit test
    Tests: Add buildah bud with --quiet test
    Tests: Add two test for buildah add
    Bump back to v1.12.0-dev

## v1.11.6 (2019-12-03)
    Handle missing equal sign in --from and --chown flags for COPY/ADD
    bud COPY does not download URL
    Bump github.com/onsi/gomega from 1.7.0 to 1.7.1
    Fix .dockerignore exclude regression
    Ran buildah through codespell
    commit(docker): always set ContainerID and ContainerConfig
    Touch up commit man page image parameter
    Add builder identity annotations.
    info: use util.Runtime()
    Bump github.com/onsi/ginkgo from 1.10.2 to 1.10.3
    Bump back to v1.12.0-dev

## v1.11.5 (2019-11-11)
    Enhance error on unsafe symbolic link targets
    Add OCIRuntime to info
    Check nonexsit authfile
    Only output image id if running buildah bud --quiet
    Fix --pull=true||false and add --pull-never to bud and from (retry)
    cgroups v2: tweak or skip tests
    Prepwork: new 'skip' helpers for tests
    Handle configuration blobs for manifest lists
    unmarshalConvertedConfig: avoid using the updated image's ref
    Add completions for Manifest commands
    Add disableFips option to secrets pkg
    Update bud.bats test archive test
    Add test for caching based on content digest
    Builder.untarPath(): always evaluate b.ContentDigester.Hash()
    Bump github.com/onsi/ginkgo from 1.10.1 to 1.10.2
    Fix another broken test: copy-url-mtime
    yet more fixes
    Actual bug fix for 'add' test: fix the expected mode
    BATS tests - lots of mostly minor cleanup
    build: drop support for ostree
    Add support for make vendor-in-container
    imgtype: exit with error if storage fails
    remove XDG_RUNTIME_DIR from default authfile path
    fix troubleshooting redirect instructions
    Bump back to v1.12.0-dev

## v1.11.4 (2019-10-28)
    buildah: add a "manifest" command
    manifests: add the module
    pkg/supplemented: add a package for grouping images together
    pkg/manifests: add a manifest list build/manipulation API
    Update for ErrUnauthorizedForCredentials API change in containers/image
    Update for manifest-lists API changes in containers/image
    version: also note the version of containers/image
    Move to containers/image v5.0.0
    Enable --device directory as src device
    Fix git build with branch specified
    Bump github.com/openshift/imagebuilder from 1.1.0 to 1.1.1
    Bump github.com/fsouza/go-dockerclient from 1.4.4 to 1.5.0
    Add clarification to the Tutorial for new users
    Silence "using cache" to ensure -q is fully quiet
    Add OWNERS File to Buildah
    Bump github.com/containers/storage from 1.13.4 to 1.13.5
    Move runtime flag to bud from common
    Commit: check for storage.ErrImageUnknown using errors.Cause()
    Fix crash when invalid COPY --from flag is specified.
    Bump back to v1.12.0-dev

## v1.11.3 (2019-10-04)
    Update c/image to v4.0.1
    Bump github.com/spf13/pflag from 1.0.3 to 1.0.5
    Fix --build-args handling
    Bump github.com/spf13/cobra from 0.0.3 to 0.0.5
    Bump github.com/cyphar/filepath-securejoin from 0.2.1 to 0.2.2
    Bump github.com/onsi/ginkgo from 1.8.0 to 1.10.1
    Bump github.com/fsouza/go-dockerclient from 1.3.0 to 1.4.4
    Add support for retrieving context from stdin "-"
    Ensure bud remote context cleans up on error
    info: add cgroups2
    Bump github.com/seccomp/libseccomp-golang from 0.9.0 to 0.9.1
    Bump github.com/mattn/go-shellwords from 1.0.5 to 1.0.6
    Bump github.com/stretchr/testify from 1.3.0 to 1.4.0
    Bump github.com/opencontainers/selinux from 1.2.2 to 1.3.0
    Bump github.com/etcd-io/bbolt from 1.3.2 to 1.3.3
    Bump github.com/onsi/gomega from 1.5.0 to 1.7.0
    update c/storage to v1.13.4
    Print build 'STEP' line to stdout, not stderr
    Fix travis-ci on forks
    Vendor c/storage v1.13.3
    Use Containerfile by default
    Added tutorial on how to include Buildah as library
    util/util: Fix "configuraitno" -> "configuration" log typo
    Bump back to v1.12.0-dev

## v1.11.2 (2019-09-13)
    Add some cleanup code
    Move devices code to unit specific directory.
    Bump back to v1.12.0-dev

## v1.11.1 (2019-09-11)
    Add --devices flag to bud and from
    Downgrade .papr to highest atomic verion
    Add support for /run/.containerenv
    Truncate output of too long image names
    Preserve file and directory mount permissions
    Bump fedora version from 28 to 30
    makeImageRef: ignore EmptyLayer if Squash is set
    Set TMPDIR to /var/tmp by default
    replace --debug=false with --log-level=error
    Allow mounts.conf entries for equal source and destination paths
    fix label and annotation for 1-line Dockerfiles
    Enable interfacer linter and fix lints
    install.md: mention goproxy
    Makefile: use go proxy
    Bump to v1.12.0-dev

## v1.11.0 (2019-08-29)
    tests/bud.bats: add --signature-policy to some tests
    Vendor github.com/openshift/api
    pull/commit/push: pay attention to $BUILD_REGISTRY_SOURCES
    Add `--log-level` command line option and deprecate `--debug`
    add support for cgroupsV2
    Correctly detect ExitError values from Run()
    Disable empty logrus timestamps to reduce logger noise
    Remove outdated deps Makefile target
    Remove gofmt.sh in favor of golangci-lint
    Remove govet.sh in favor of golangci-lint
    Allow to override build date with SOURCE_DATE_EPOCH
    Update shebangs to take env into consideration
    Fix directory pull image names
    Add --digestfile and Re-add push statement as debug
    README: mention that Podman uses Buildah's API
    Use content digests in ADD/COPY history entries
    add: add a DryRun flag to AddAndCopyOptions
    Fix possible runtime panic on bud
    Add security-related volume options to validator
    use correct path for ginkgo
    Add bud 'without arguments' integration tests
    Update documentation about bud
    add: handle hard links when copying with .dockerignore
    add: teach copyFileWithTar() about symlinks and directories
    Allow buildah bud to be called without arguments
    imagebuilder: fix detection of referenced stage roots
    Touch up go mod instructions in install
    run_linux: fix mounting /sys in a userns
    Vendor Storage v1.13.2
    Cirrus: Update VM images
    Fix handling of /dev/null masked devices
    Update `bud`/`from` help to contain indicator for `--dns=none`
    Bump back to v1.11.0-dev

## v1.10.1 (2019-08-08)
    Bump containers/image to v3.0.2 to fix keyring issue
    Bug fix for volume minus syntax
    Bump container/storage v1.13.1 and containers/image v3.0.1
    bump github.com/containernetworking/cni to v0.7.1
    Add overlayfs to fuse-overlayfs tip
    Add automatic apparmor tag discovery
    Fix bug whereby --get-login has no effect
    Bump to v1.11.0-dev

## v1.10.0 (2019-08-02)
    vendor github.com/containers/image@v3.0.0
    Remove GO111MODULE in favor of `-mod=vendor`
    Vendor in containers/storage v1.12.16
    Add '-' minus syntax for removal of config values
    tests: enable overlay tests for rootless
    rootless, overlay: use fuse-overlayfs
    vendor github.com/containers/image@v2.0.1
    Added '-' syntax to remove volume config option
    delete `successfully pushed` message
    Add golint linter and apply fixes
    vendor github.com/containers/storage@v1.12.15
    Change wait to sleep in buildahimage readme
    Handle ReadOnly images when deleting images
    Add support for listing read/only images

## v1.9.2 (2019-07-19)
    from/import: record the base image's digest, if it has one
    Fix CNI version retrieval to not require network connection
    Add misspell linter and apply fixes
    Add goimports linter and apply fixes
    Add stylecheck linter and apply fixes
    Add unconvert linter and apply fixes
    image: make sure we don't try to use zstd compression
    run.bats: skip the "z" flag when testing --mount
    Update to runc v1.0.0-rc8
    Update to match updated runtime-tools API
    bump github.com/opencontainers/runtime-tools to v0.9.0
    Build e2e tests using the proper build tags
    Add unparam linter and apply fixes
    Run: correct a typo in the --cap-add help text
    unshare: add a --mount flag
    fix push check image name is not empty
    Bump to v1.9.2-dev

## v1.9.1 (2019-07-12)
    add: fix slow copy with no excludes
    Add errcheck linter and fix missing error check
    Improve tests/tools/Makefile parallelism and abstraction
    Fix response body not closed resource leak
    Switch to golangci-lint
    Add gomod instructions and mailing list links
    On Masked path, check if /dev/null already mounted before mounting
    Update to containers/storage v1.12.13
    Refactor code in package imagebuildah
    Add rootless podman with NFS issue in documentation
    Add --mount  for buildah run
    import method ValidateVolumeOpts from libpod
    Fix typo
    Makefile: set GO111MODULE=off
    rootless: add the built-in slirp DNS server
    Update docker/libnetwork to get rid of outdated sctp package
    Update buildah-login.md
    migrate to go modules
    install.md: mention go modules
    tests/tools: go module for test binaries
    fix --volume splits comma delimited option
    Add bud test for RUN with a priv'd command
    vendor logrus v1.4.2
    pkg/cli: panic when flags can't be hidden
    pkg/unshare: check all errors
    pull: check error during report write
    run_linux.go: ignore unchecked errors
    conformance test: catch copy error
    chroot/run_test.go: export funcs to actually be executed
    tests/imgtype: ignore error when shutting down the store
    testreport: check json error
    bind/util.go: remove unused func
    rm chroot/util.go
    imagebuildah: remove unused `dedupeStringSlice`
    StageExecutor: EnsureContainerPath: catch error from SecureJoin()
    imagebuildah/build.go: return <expr> instead of branching
    rmi: avoid redundant branching
    conformance tests: nilness: allocate map
    imagebuildah/build.go: avoid redundant `filepath.Join()`
    imagebuildah/build.go: avoid redundant `os.Stat()`
    imagebuildah: omit comparison to bool
    fix "ineffectual assignment" lint errors
    docker: ignore "repeats json tag" lint error
    pkg/unshare: use `...` instead of iterating a slice
    conformance: bud test: use raw strings for regexes
    conformance suite: remove unused func/var
    buildah test suite: remove unused vars/funcs
    testreport: fix golangci-lint errors
    util: remove redundant `return` statement
    chroot: only log clean-up errors
    images_test: ignore golangci-lint error
    blobcache: log error when draining the pipe
    imagebuildah: check errors in deferred calls
    chroot: fix error handling in deferred funcs
    cmd: check all errors
    chroot/run_test.go: check errors
    chroot/run.go: check errors in deferred calls
    imagebuildah.Executor: remove unused onbuild field
    docker/types.go: remove unused struct fields
    util: use strings.ContainsRune instead of index check
    Cirrus: Initial implementation
    Bump to v1.9.1-dev

## v1.9.0 (2019-06-15)
    buildah-run: fix-out-of-range panic (2)
    Bump back to v1.9.0-dev



## v1.8.4 (2019-06-13)
    Update containers/image to v2.0.0
    run: fix hang with run and --isolation=chroot
    run: fix hang when using run
    chroot: drop unused function call
    remove --> before imgageID on build
    Always close stdin pipe
    Write deny to setgroups when doing single user mapping
    Avoid including linux/memfd.h
    Add a test for the symlink pointing to a directory
    Add missing continue
    Fix the handling of symlinks to absolute paths
    Only set default network sysctls if not rootless
    Support --dns=none like podman
    fix bug --cpu-shares parsing typo
    Fix validate complaint
    Update vendor on containers/storage to v1.12.10
    Create directory paths for COPY thereby ensuring correct perms
    imagebuildah: use a stable sort for comparing build args
    imagebuildah: tighten up cache checking
    bud.bats: add a test verying the order of --build-args
    add -t to podman run
    imagebuildah: simplify screening by top layers
    imagebuildah: handle ID mappings for COPY --from
    imagebuildah: apply additionalTags ourselves
    bud.bats: test additional tags with cached images
    bud.bats: add a test for WORKDIR and COPY with absolute destinations
    Cleanup Overlay Mounts content

## v1.8.3 (2019-06-04)
    Add support for file secret mounts
    Add ability to skip secrets in mounts file
    allow 32bit builds
    fix tutorial instructions
    imagebuilder: pass the right contextDir to Add()
    add: use fileutils.PatternMatcher for .dockerignore
    bud.bats: add another .dockerignore test
    unshare: fallback to single usermapping
    addHelperSymlink: clear the destination on os.IsExist errors
    bud.bats: test replacing symbolic links
    imagebuildah: fix handling of destinations that end with '/'
    bud.bats: test COPY with a final "/" in the destination
    linux: add check for sysctl before using it
    unshare: set _CONTAINERS_ROOTLESS_GID
    Rework buildahimamges
    build context: support https git repos
    Add a test for ENV special chars behaviour
    Check in new Dockerfiles
    Apply custom SHELL during build time
    config: expand variables only at the command line
    SetEnv: we only need to expand v once
    Add default /root if empty on chroot iso
    Add support for Overlay volumes into the container.
    Export buildah validate volume functions so it can share code with libpod
    Bump baseline test to F30
    Fix rootless handling of /dev/shm size
    Avoid fmt.Printf() in the library
    imagebuildah: tighten cache checking back up
    Handle WORKDIR with dangling target
    Default Authfile to proper path
    Make buildah run --isolation follow BUILDAH_ISOLATION environment
    Vendor in latest containers/storage and containers/image
    getParent/getChildren: handle layerless images
    imagebuildah: recognize cache images for layerless images
    bud.bats: test scratch images with --layers caching
    Get CHANGELOG.md updates
    Add some symlinks to test our .dockerignore logic
    imagebuildah: addHelper: handle symbolic links
    commit/push: use an everything-allowed policy
    Correct manpage formatting in files section
    Remove must be root statement from buildah doc
    Change image names to stable, testing and upstream
    Bump back to v1.9.0-dev

## v1.8.2 (2019-05-02)
    Vendor Storage 1.12.6
    Create scratch file in TESTDIR
    Test bud-copy-dot with --layers picks up changed file
    Bump back to 1.9.0-dev

## v1.8.1 (2019-05-01)
    Don't create directory on container
    Replace kubernetes/pause in tests with k8s.gcr.io/pause
    imagebuildah: don't remove intermediate images if we need them
    Rework buildahimagegit to buildahimageupstream
    Fix Transient Mounts
    Handle WORKDIRs that are symlinks
    allow podman to build a client for windows
    Touch up 1.9-dev to 1.9.0-dev
    Bump to 1.9-dev

## v1.8.0 (2019-04-26)
    Resolve symlink when checking container path
    commit: commit on every instruction, but not always with layers
    CommitOptions: drop the unused OnBuild field
    makeImageRef: pass in the whole CommitOptions structure
    cmd: API cleanup: stores before images
    run: check if SELinux is enabled
    Fix buildahimages Dockerfiles to include support for additionalimages mounted from host.
    Detect changes in rootdir
    Fix typo in buildah-pull(1)
    Vendor in latest containers/storage
    Keep track of any build-args used during buildah bud --layers
    commit: always set a parent ID
    imagebuildah: rework unused-argument detection
    fix bug dest path when COPY .dockerignore
    Move Host IDMAppings code from util to unshare
    Add BUILDAH_ISOLATION rootless back
    Travis CI: fail fast, upon error in any step
    imagebuildah: only commit images for intermediate stages if we have to
    Use errors.Cause() when checking for IsNotExist errors
    auto pass http_proxy to container
    Bump back to 1.8-dev

## v1.7.3 (2019-04-16)
* Tue Apr 16, 2019 Tom Sweeney <tsweeney@redhat.com> 1.7.3
    imagebuildah: don't leak image structs
    Add Dockerfiles for buildahimages
    Bump to Replace golang 1.10 with 1.12
    add --dns* flags to buildah bud
    Add hack/build_speed.sh test speeds on building container images
    Create buildahimage Dockerfile for Quay
    rename 'is' to 'expect_output'
    squash.bats: test squashing in multi-layered builds
    bud.bats: test COPY --from in a Dockerfile while using the cache
    commit: make target image names optional
    Fix bud-args to allow comma separation
    oops, missed some tests in commit.bats
    new helper: expect_line_count
    New tests for #1467 (string slices in cmdline opts)
    Workarounds for dealing with travis; review feedback
    BATS tests - extensive but minor cleanup
    imagebuildah: defer pulling images for COPY --from
    imagebuildah: centralize COMMIT and image ID output
    Travis: do not use traviswait
    imagebuildah: only initialize imagebuilder configuration once per stage
    Make cleaner error on Dockerfile build errors
    unshare: move to pkg/
    unshare: move some code from cmd/buildah/unshare
    Fix handling of Slices versus Arrays
    imagebuildah: reorganize stage and per-stage logic
    imagebuildah: add empty layers for instructions
    Add missing step in installing into Ubuntu
    fix bug in .dockerignore support
    imagebuildah: deduplicate prepended "FROM" instructions
    Touch up intro
    commit: set created-by to the shell if it isn't set
    commit: check that we always set a "created-by"
    docs/buildah.md: add "containers-" prefixes under "SEE ALSO"
    Bump back to 1.8-dev

## v1.7.2 (2019-03-28)
    mount: do not create automatically a namespace
    buildah: correctly create the userns if euid!=0
    imagebuildah.Build: consolidate cleanup logic
    CommitOptions: drop the redundant Store field
    Move pkg/chrootuser from libpod to buildah.
    imagebuildah: record image IDs and references more often
    vendor imagebuilder v1.1.0
    imagebuildah: fix requiresStart/noRunsRemaining confusion
    imagebuildah: check for unused args across stages
    bump github.com/containernetworking/cni to v0.7.0-rc2
    imagebuildah: use "useCache" instead of "noCache"
    imagebuildah.resolveNameToImageRef(): take name as a parameter
    Export fields of the DokcerIgnore struct
    imagebuildah: drop the duplicate containerIDs list
    rootless: by default use the host network namespace
    imagebuildah: split Executor and per-stage execution
    imagebuildah: move some fields around
    golint: make golint happy
    docs: 01-intro.md: add missing . in Dockerfile examples
    fix bug using .dockerignore
    Do not create empty mounts.conf file
    images: suppress a spurious blank line with no images
    from: distinguish between ADD and COPY
    fix bug to not separate each --label value with comma
    buildah-bud.md: correct a typo, note a default
    Remove mistaken code that got merged in other PR
    add sample registries.conf to docs
    escape shell variables in README example
    slirp4netns: set mtu to 65520
    images: imageReposToMap() already adds <none>:<none>
    imagebuildah.ReposToMap: move to cmd
    Build: resolve copyFrom references earlier
    Allow rootless users to use the cache directory in homedir
    bud.bats: use the per-test temp directory
    bud.bats: log output before counting length
    Simplify checks for leftover args
    Print commitID with --layers
    fix bug images use the template to print results
    rootless: honor --net host
    onsi/gomeage add missing files
    vendor latest openshift/imagebuilder
    Remove noop from squash help
    Prepend a comment to files setup in container
    imagebuildah resolveSymlink: fix handling of relative links
    Errors should be printed to stderr
    Add recommends for slirp4netns and fuse-overlay
    Update pull and pull-always flags
    Hide from users command options that we don't want them to use.
    Update secrets fipsmode patch to work on rootless containers
    fix unshare option handling and documentation
    Vendor in latest containers/storage
    Hard-code docker.Transport use in pull --all-tags
    Use a types.ImageReference instead of (transport, name) strings in pullImage etc.
    Move the computation of srcRef before first pullAndFindImage
    Don't throw away user-specified tag for pull --all-tags
    CHANGES BEHAVIOR: Remove the string format input to localImageNameForReference
    Don't try to parse imageName as transport:image in pullImage
    Use reference.WithTag instead of manual string manipulation in Pull
    Don't pass image = transport:repo:tag, transport=transport to pullImage
    Fix confusing variable naming in Pull
    Don't try to parse image name as a transport:image
    Fix error reporting when parsing trans+image
    Remove 'transport == ""' handling from the pull path
    Clean up "pulls" of local image IDs / ID prefixes
    Simplify ExpandNames
    Document the semantics of transport+name returned by ResolveName
    UPdate gitvalidation epoch
    Bump back to 1.8-dev

## v1.7.1 (2019-02-26)
    vendor containers/image v1.5
    Move secrets code from libpod into buildah
    Update CHANGELOG.md with the past changes
    README.md: fix typo
    Fix a few issues found by tests/validate/gometalinter.sh
    Neutralize buildah/unshare on non-Linux platforms
    Explicitly specify a directory to find(1)
    README.md: rephrase Buildah description
    Stop printing default twice in cli --help
    install.md: add section about vendoring
    Bump to 1.8-dev

## v1.7 (2019-02-21)
    vendor containers/image v1.4
    Make "images --all" faster
    Remove a misleading comment
    Remove quiet option from pull options
    Make sure buildah pull --all-tags only works with docker transport
    Support oci layout format
    Fix pulling of images within buildah
    Fix tls-verify polarity
    Travis: execute make vendor and hack/tree_status.sh
    vendor.conf: remove unused dependencies
    add missing vendor/github.com/containers/libpod/vendor.conf
    vendor.conf: remove github.com/inconshreveable/mousetrap
    make vendor: always fetch the latest vndr
    add hack/tree_status.sh script
    Bump c/Storage to 1.10
    Add --all-tags test to pull
    mount: make error clearer
    Remove global flags from cli help
    Set --disable-compression to true as documented
    Help document using buildah mount in rootless mode
    healthcheck start-period: update documentation
    Vendor in latest c/storage and c/image
    dumpbolt: handle nested buckets
    Fix buildah commit compress by default
    Test on xenial, not trusty
    unshare: reexec using a memfd copy instead of the binary
    Add --target to bud command
    Fix example for setting multiple environment variables
    main: fix rootless mode
    buildah: force umask 022
    pull.bats: specify registry config when using registries
    pull.bats: use the temporary directory, not /tmp
    unshare: do not set rootless mode if euid=0
    Touch up cli help examples and a few nits
    Add an undocumented dumpbolt command
    Move tar commands into containers/storage
    Fix bud issue with 2 line Dockerfile
    Add package install descriptions
    Note configuration file requirements
    Replace urfave/cli with cobra
    cleanup vendor.conf
    Vendor in latest containers/storage
    Add Quiet to PullOptions and PushOptions
    cmd/commit: add flag omit-timestamp to allow for deterministic builds
    Add options for empty-layer history entries
    Make CLI help descriptions and usage a bit more consistent
    vndr opencontainers/selinux
    Bump baseline test Fedora to 29
    Bump to v1.7-dev-1
    Bump to v1.6-1
    Add support for ADD --chown
    imagebuildah: make EnsureContainerPath() check/create the right one
    Bump 1.7-dev
    Fix contrib/rpm/bulidah.spec changelog date

## v1.6-1 (2019-01-18)
    Add support for ADD --chown
    imagebuildah: make EnsureContainerPath() check/create the right one
    Fix contrib/rpm/bulidah.spec changelog date
    Vendor in latest containers/storage
    Revendor everything
    Revendor in latest code by release
    unshare: do not set USER=root
    run: ignore EIO when flushing at the end, avoid double log
    build-using-dockerfile,commit: disable compression by default
    Update some comments
    Make rootless work under no_pivot_root
    Add CreatedAtRaw date field for use with Format
    Properly format images JSON output
    pull: add all-tags option
    Fix support for multiple Short options
    pkg/blobcache: add synchronization
    Skip empty files in file check of conformance test
    Use NoPivot also for RUN, not only for run
    Remove no longer used isReferenceInsecure / isRegistryInsecure
    Do not set OCIInsecureSkipTLSVerify based on registries.conf
    Remove duplicate entries from images JSON output
    vendor parallel-copy from containers/image
    blobcache.bats: adjust explicit push tests
    Handle one line Dockerfile with layers
    We should only warn if user actually requests Hostname be set in image
    Fix compiler Warning about comparing different size types
    imagebuildah: don't walk if rootdir and path are equal
    Add aliases for buildah containers, so buildah list, ls and ps work
    vendor: use faster version instead compress/gzip
    vendor: update libpod
    Properly handle Hostname inside of RUN command
    docs: mention how to mount in rootless mode
    tests: use fully qualified name for centos image
    travis.yml: use the fully qualified name for alpine
    mount: allow mount only when using vfs
    Add some tests for buildah pull
    Touch up images -q processing
    Refactor: Use library shared idtools.ParseIDMap() instead of bundling it
    bump GITVALIDATE_EPOCH
    cli.BudFlags: add `--platform` nop
    Makefile: allow packagers to more easily add tags
    Makefile: soften the requirement on git
    tests: add containers json test
    Inline blobCache.putBlob into blobCacheDestination.PutBlob
    Move saveStream and putBlob near blobCacheDestination.PutBlob
    Remove BlobCache.PutBlob
    Update for API changes
    Vendor c/image after merging c/image#536
    Handle 'COPY --from' in Dockerfile
    Vendor in latest content from github.com/containers/storage
    Clarify docker.io default in push with docker-daemon
    Test blob caching
    Wire in a hidden --blob-cache option
    Use a blob cache when we're asked to use one
    Add --disable-compression to 'build-using-dockerfile'
    Add a blob cache implementation
    vendor: update containers/storage
    Update for sysregistriesv2 API changes
    Update containers/image to 63a1cbdc5e6537056695cf0d627c0a33b334df53
    clean up makefile variables
    Fix file permission
    Complete the instructions for the command
    Show warning when a build arg not used
    Assume user 0 group 0, if /etc/passwd file in container.
    Add buildah info command
    Enable -q when --filter is used for images command
    Add v1.5 Release Announcement
    Fix dangling filter for images command
    Fix completions to print Names as well as IDs
    tests: Fix file permissions
    Bump 1.6-dev

## v1.5-1 (2018-11-21)
    Bump min go to 1.10 in install.md
    vendor: update ostree-go
    Update docker build command line in conformance test
    Print command in SystemExec as debug information
    Add some skip word for inspect check in conformance test
    Update regex for multi stage base test
    Sort CLI flags
    vendor: update containers/storage
    Add note to install about non-root on RHEL/CentOS
    Update imagebuild depdency to support heading ARGs in Dockerfile
    rootless: do not specify --rootless to the OCI runtime
    Export resolvesymlink function
    Exclude --force-rm from common bud cli flags
    run: bind mount /etc/hosts and /etc/resolv.conf if not in a volume
    rootless: use slirp4netns to setup the network namespace
    Instructions for completing the pull command
    Fix travis to not run environment variable patch
    rootless: only discard network configuration names
    run: only set up /etc/hosts or /etc/resolv.conf with network
    common: getFormat: match entire string not only the prefix
    vendor: update libpod
    Change validation EPOCH
    Fixing broken link for container-registries.conf
    Restore rootless isolation test for from volume ro test
    ostree: fix tag for build constraint
    Handle directories better in bud -f
    vndr in latest containers/storage
    Fix unshare gofmt issue
    runSetupBuiltinVolumes(): break up volume setup
    common: support a per-user registries conf file
    unshare: do not override the configuration
    common: honor the rootless configuration file
    unshare: create a new mount namespace
    unshare: support libpod rootless pkg
    Use libpod GetDefaultStorage to report proper storage config
    Allow container storage to manage the SELinux labels
    Resolve image names with default transport in from command
    run: When the value of isolation is set, use the set value instead of the default value.
    Vendor in latest containers/storage and opencontainers/selinux
    Remove no longer valid todo
    Check for empty buildTime in version
    Change gofmt so it runs on all but 1.10
    Run gofmt only on Go 1.11
    Walk symlinks when checking cached images for copied/added files
    ReserveSELinuxLabels(): handle wrapped errors from OpenBuilder
    Set WorkingDir to empty, not / for conformance
    Update calls in e2e to addres 1101
    imagebuilder.BuildDockerfiles: return the image ID
    Update for changes in the containers/image API
    bump(github.com/containers/image)
    Allow setting --no-pivot default with an env var
    Add man page and bash completion, for --no-pivot
    Add the --no-pivot flag to the run command
    Improve reporting about individual pull failures
    Move the "short name but no search registries" error handling to resolveImage
    Return a "search registries were needed but empty" indication in util.ResolveName
    Simplify handling of the "tried to pull an image but found nothing" case in newBuilder
    Don't even invoke the pull loop if options.FromImage == ""
    Eliminate the long-running ref and img variables in resolveImage
    In resolveImage, return immediately on success
    Fix From As in Dockerfile
    Vendor latest containers/image
    Vendor in latest libpod
    Sort CLI flags of buildah bud
    Change from testing with golang 1.9 to 1.11.
    unshare: detect when unprivileged userns are disabled
    Optimize redundant code
    fix missing format param
    chroot: fix the args check
    imagebuildah: make ResolveSymLink public
    Update copy chown test
    buildah: use the same logic for XDG_RUNTIME_DIR as podman
    V1.4 Release Announcement
    Podman  --privileged selinux is broken
    papr: mount source at gopath
    parse: Modify the return value
    parse: modify the verification of the isolation value
    Make sure we log or return every error
    pullImage(): when completing an image name, try docker://
    Fix up Tutorial 3 to account for format
    Vendor in latest containers/storage and containers/image
    docs/tutorials/01-intro.md: enhanced installation instructions
    Enforce "blocked" for registries for the "docker" transport
    Correctly set DockerInsecureSkipTLSVerify when pulling images
    chroot: set up seccomp and capabilities after supplemental groups
    chroot: fix capabilities list setup and application
    .papr.yml: log the podman version
    namespaces.bats: fix handling of uidmap/gidmap options in pairs
    chroot: only create user namespaces when we know we need them
    Check /proc/sys/user/max_user_namespaces on unshare(NEWUSERNS)
    bash/buildah: add isolation option to the from command

## v1.4 (2018-10-02)
    from: fix isolation option
    Touchup pull manpage
    Export buildah ReserveSELinuxLables so podman can use it
    Add buildah.io to README.md and doc fixes
    Update rmi man for prune changes
    Ignore file not found removal error in bud
    bump(github.com/containers/{storage,image})
    NewImageSource(): only create one Diff() at a time
    Copy ExposedPorts from base image into the config
    tests: run conformance test suite in Travis
    Change rmi --prune to not accept an imageID
    Clear intermediate container IDs after each stage
    Request podman version for build issues
    unshare: keep the additional groups of the user
    Builtin volumes should be owned by the UID/GID of the container
    Get rid of dangling whitespace in markdown files
    Move buildah from projecatatomic/buildah to containers/buildah
    nitpick: parse.validateFlags loop in bud cli
    bash: Completion options
    Add signature policy to push tests
    vendor in latest containers/image
    Fix grammar in Container Tools Guide
    Don't build btrfs if it is not installed
    new: Return image-pulling errors from resolveImage
    pull: Return image-pulling errors from pullImage
    Add more volume mount tests
    chroot: create missing parent directories for volume mounts
    Push: Allow an empty destination
    Add Podman relationship to readme, create container tools guide
    Fix arg usage in buildah-tag
    Add flags/arguments order verification to other commands
    Handle ErrDuplicateName errors from store.CreateContainer()
    Evaluate symbolic links on Add/Copy Commands
    Vendor in latest containers/image and containers/storage
    Retain bounding set when running containers as non root
    run container-diff tests in Travis
    buildah-images.md: Fix option contents
    push: show image digest after push succeed
    Vendor in latest containers/storage,image,libpod and runc
    Change references to cri-o to point at new repository
    Exclude --layers from the common bug cli flags
    demos: Increase the executable permissions
    run: clear default seccomp filter if not enabled
    Bump maximum cyclomatic complexity to 45
    stdin: on HUP, read everything
    nitpick: use tabs in tests/helpers.bash
    Add flags/arguments order verification to one arg commands
    nitpick: decrease cognitive complexity in buildah-bud
    rename: Avoid renaming the same name as other containers
    chroot isolation: chroot() before setting up seccomp
    Small nitpick at the "if" condition in tag.go
    cmd/images: Modify json option
    cmd/images: Disallow the input of image when using the -a option
    Fix examples to include context directory
    Update containers/image to fix commit layer issue
    cmd/containers: End loop early when using the json option
    Make buildah-from error message clear when flags are after arg
    Touch up README.md for conformance tests
    Update container/storage for lock fix
    cmd/rm: restore the correct containerID display
    Remove debug lines
    Remove docker build image after each test
    Add README for conformance test
    Update the MakeOptions to accept all command options for buildah
    Update regrex to fit the docker output in test "run with JSON"
    cmd/buildah: Remove redundant variable declarations
    Warn about using Commands in Dockerfile that are not supported by OCI.
    Add buildah bud conformance test
    Fix rename to also change container name in builder
    Makefile: use $(GO) env-var everywhere
    Cleanup code to more closely match Docker Build images
    Document BUILDAH_* environment variables in buildah bud --help output
    Return error immediately if error occurs in Prepare step
    Fix --layers ADD from url issue
    Add "Sign your PRs" TOC item to contributing.md.
    Display the correct ID after deleting image
    rmi: Modify the handling of errors
    Let util.ResolveName() return parsing errors
    Explain Open Container Initiative (OCI) acronym, add link
    Update vendor for urfave/cli back to master
    Handle COPY --chown in Dockerfile
    Switch to Recommends container-selinux
    Update vendor for containernetworking, imagebuildah and podman
    Document STORAGE_DRIVER and STORAGE_OPTS environment variable
    Change references to projectatomic/libpod to containers/libpod
    Add container PATH retrieval example
    Expand variables names for --env
    imagebuildah: provide a way to provide stdin for RUN
    Remove an unused srcRef.NewImageSource in pullImage
    chroot: correct a comment
    chroot: bind mount an empty directory for masking
    Don't bother with --no-pivot for rootless isolation
    CentOS need EPEL repo
    Export a Pull() function
    Remove stream options, since docker build does not have it
    release v1.3: mention openSUSE
    Add Release Announcements directory
    Bump to v1.4-dev

## 1.3 (2018-08-4)
    Revert pull error handling from 881
    bud should not search context directory for Dockerfile
    Set BUILDAH_ISOLATION=rootless when running unprivileged
    .papr.sh: Also test with BUILDAH_ISOLATION=rootless
    Skip certain tests when we're using "rootless" isolation
    .travis.yml: run integration tests with BUILDAH_ISOLATION=chroot
    Add and implement IsolationOCIRootless
    Add a value for IsolationOCIRootless
    Fix rmi to remove intermediate images associated with an image
    Return policy error on pull
    Update containers/image to 216acb1bcd2c1abef736ee322e17147ee2b7d76c
    Switch to github.com/containers/image/pkg/sysregistriesv2
    unshare: make adjusting the OOM score optional
    Add flags validation
    chroot: handle raising process limits
    chroot: make the resource limits name map module-global
    Remove rpm.bats, we need to run this manually
    Set the default ulimits to match Docker
    buildah: no args is out of bounds
    unshare: error message missed the pid
    preprocess ".in" suffixed Dockerfiles
    Fix the the in buildah-config man page
    Only test rpmbuild on latest fedora
    Add support for multiple Short options
    Update to latest urvave/cli
    Add additional SELinux tests
    Vendor in latest github.com/containers/{image;storage}
    Stop testing with golang 1.8
    Fix volume cache issue with buildah bud --layers
    Create buildah pull command
    Increase the deadline for gometalinter during 'make validate'
    .papr.sh: Also test with BUILDAH_ISOLATION=chroot
    .travis.yml: run integration tests with BUILDAH_ISOLATION=chroot
    Add a Dockerfile
    Set BUILDAH_ISOLATION=chroot when running unprivileged
    Add and implement IsolationChroot
    Update github.com/opencontainers/runc
    maybeReexecUsingUserNamespace: add a default for root
    Allow ping command without NET_RAW Capabilities
    rmi.storageImageID: fix Wrapf format warning
    Allow Dockerfile content to come from stdin
    Vendor latest container/storage to fix overlay mountopt
    userns: assign additional IDs sequentially
    Remove default dev/pts
    Add OnBuild test to baseline test
    tests/run.bats(volumes): use :z when SELinux is enabled
    Avoid a stall in runCollectOutput()
    Use manifest from container/image
    Vendor in latest containers/image and containers/storage
    add rename command
    Completion command
    Update CHANGELOG.md
    Update vendor for runc to fix 32 bit builds
    bash completion: remove shebang
    Update vendor for runc to fix 32 bit builds

## 1.2 (2018-07-14)
    Vendor in lates containers/image
    build-using-dockerfile: let -t include transports again
    Block use of /proc/acpi and /proc/keys from inside containers
    Fix handling of --registries-conf
    Fix becoming a maintainer link
    add optional CI test fo darwin
    Don't pass a nil error to errors.Wrapf()
    image filter test: use kubernetes/pause as a "since"
    Add --cidfile option to from
    vendor: update containers/storage
    Contributors need to find the CONTRIBUTOR.md file easier
    Add a --loglevel option to build-with-dockerfile
    Create Development plan
    cmd: Code improvement
    allow buildah cross compile for a darwin target
    Add unused function param lint check
    docs: Follow man-pages(7) suggestions for SYNOPSIS
    Start using github.com/seccomp/containers-golang
    umount: add all option to umount all mounted containers
    runConfigureNetwork(): remove an unused parameter
    Update github.com/opencontainers/selinux
    Fix buildah bud --layers
    Force ownership of /etc/hosts and /etc/resolv.conf to 0:0
    main: if unprivileged, reexec in a user namespace
    Vendor in latest imagebuilder
    Reduce the complexity of the buildah.Run function
    mount: output it before replacing lastError
    Vendor in latest selinux-go code
    Implement basic recognition of the "--isolation" option
    Run(): try to resolve non-absolute paths using $PATH
    Run(): don't include any default environment variables
    build without seccomp
    vendor in latest runtime-tools
    bind/mount_unsupported.go: remove import errors
    Update github.com/opencontainers/runc
    Add Capabilities lists to BuilderInfo
    Tweaks for commit tests
    commit: recognize committing to second storage locations
    Fix ARGS parsing for run commands
    Add info on registries.conf to from manpage
    Switch from using docker to podman for testing in .papr
    buildah: set the HTTP User-Agent
    ONBUILD tutorial
    Add information about the configuration files to the install docs
    Makefile: add uninstall
    Add tilde info for push to troubleshooting
    mount: support multiple inputs
    Use the right formatting when adding entries to /etc/hosts
    Vendor in latest go-selinux bindings
    Allow --userns-uid-map/--userns-gid-map to be global options
    bind: factor out UnmountMountpoints
    Run(): simplify runCopyStdio()
    Run(): handle POLLNVAL results
    Run(): tweak terminal mode handling
    Run(): rename 'copyStdio' to 'copyPipes'
    Run(): don't set a Pdeathsig for the runtime
    Run(): add options for adding and removing capabilities
    Run(): don't use a callback when a slice will do
    setupSeccomp(): refactor
    Change RunOptions.Stdin/Stdout/Stderr to just be Reader/Writers
    Escape use of '_' in .md docs
    Break out getProcIDMappings()
    Break out SetupIntermediateMountNamespace()
    Add Multi From Demo
    Use the c/image conversion code instead of converting configs manually
    Don't throw away the manifest MIME type and guess again
    Consolidate loading manifest and config in initConfig
    Pass a types.Image to Builder.initConfig
    Require an image ID in importBuilderDataFromImage
    Use c/image/manifest.GuessMIMEType instead of a custom heuristic
    Do not ignore any parsing errors in initConfig
    Explicitly handle "from scratch" images in Builder.initConfig
    Fix parsing of OCI images
    Simplify dead but dangerous-looking error handling
    Don't ignore v2s1 history if docker_version is not set
    Add --rm and --force-rm to buildah bud
    Add --all,-a flag to buildah images
    Separate stdio buffering from writing
    Remove tty check from images --format
    Add environment variable BUILDAH_RUNTIME
    Add --layers and --no-cache to buildah bud
    Touch up images man
    version.md: fix DESCRIPTION
    tests: add containers test
    tests: add images test
    images: fix usage
    fix make clean error
    Change 'registries' to 'container registries' in man
    add commit test
    Add(): learn to record hashes of what we add
    Minor update to buildah config documentation for entrypoint
    Bump to v1.2-dev
    Add registries.conf link to a few man pages

## 1.1 (2018-06-08)
    Drop capabilities if running container processes as non root
    Print Warning message if cmd will not be used based on entrypoint
    Update 01-intro.md
    Shouldn't add insecure registries to list of search registries
    Report errors on bad transports specification when pushing images
    Move parsing code out of common for namespaces and into pkg/parse.go
    Add disable-content-trust noop flag to bud
    Change freenode chan to buildah
    runCopyStdio(): don't close stdin unless we saw POLLHUP
    Add registry errors for pull
    runCollectOutput(): just read until the pipes are closed on us
    Run(): provide redirection for stdio
    rmi, rm: add test
    add mount test
    Add parameter judgment for commands that do not require parameters
    Add context dir to bud command in baseline test
    run.bats: check that we can run with symlinks in the bundle path
    Give better messages to users when image can not be found
    use absolute path for bundlePath
    Add environment variable to buildah --format
    rm: add validation to args and all option
    Accept json array input for config entrypoint
    Run(): process RunOptions.Mounts, and its flags
    Run(): only collect error output from stdio pipes if we created some
    Add OnBuild support for Dockerfiles
    Quick fix on demo readme
    run: fix validate flags
    buildah bud should require a context directory or URL
    Touchup tutorial for run changes
    Validate common bud and from flags
    images: Error if the specified imagename does not exist
    inspect: Increase err judgments to avoid panic
    add test to inspect
    buildah bud picks up ENV from base image
    Extend the amount of time travis_wait should wait
    Add a make target for Installing CNI plugins
    Add tests for namespace control flags
    copy.bats: check ownerships in the container
    Fix SELinux test errors when SELinux is enabled
    Add example CNI configurations
    Run: set supplemental group IDs
    Run: use a temporary mount namespace
    Use CNI to configure container networks
    add/secrets/commit: Use mappings when setting permissions on added content
    Add CLI options for specifying namespace and cgroup setup
    Always set mappings when using user namespaces
    Run(): break out creation of stdio pipe descriptors
    Read UID/GID mapping information from containers and images
    Additional bud CI tests
    Run integration tests under travis_wait in Travis
    build-using-dockerfile: add --annotation
    Implement --squash for build-using-dockerfile and commit
    Vendor in latest container/storage for devicemapper support
    add test to inspect
    Vendor github.com/onsi/ginkgo and github.com/onsi/gomega
    Test with Go 1.10, too
    Add console syntax highlighting to troubleshooting page
    bud.bats: print "$output" before checking its contents
    Manage "Run" containers more closely
    Break Builder.Run()'s "run runc" bits out
    util.ResolveName(): handle completion for tagged/digested image names
    Handle /etc/hosts and /etc/resolv.conf properly in container
    Documentation fixes
    Make it easier to parse our temporary directory as an image name
    Makefile: list new pkg/ subdirectoris as dependencies for buildah
    containerImageSource: return more-correct errors
    API cleanup: PullPolicy and TerminalPolicy should be types
    Make "run --terminal" and "run -t" aliases for "run --tty"
    Vendor github.com/containernetworking/cni v0.6.0
    Update github.com/containers/storage
    Update github.com/containers/libpod
    Add support for buildah bud --label
    buildah push/from can push and pull images with no reference
    Vendor in latest containers/image
    Update gometalinter to fix install.tools error
    Update troubleshooting with new run workaround
    Added a bud demo and tidied up
    Attempt to download file from url, if fails assume Dockerfile
    Add buildah bud CI tests for ENV variables
    Re-enable rpm .spec version check and new commit test
    Update buildah scratch demo to support el7
    Added Docker compatibility demo
    Update to F28 and new run format in baseline test
    Touchup man page short options across man pages
    Added demo dir and a demo. chged distrorlease
    builder-inspect: fix format option
    Add cpu-shares short flag (-c) and cpu-shares CI tests
    Minor fixes to formatting in rpm spec changelog
    Fix rpm .spec changelog formatting
    CI tests and minor fix for cache related noop flags
    buildah-from: add effective value to mount propagation

## 1.0 (2018-05-06)
    Declare Buildah 1.0
    Add cache-from and no-cache noops, and fix doco
    Update option and documentation for --force-rm
    Adding noop for --force-rm to match --rm
    Add buildah bud ENTRYPOINT,CMD,RUN tests
    Adding buildah bud RUN test scenarios
    Extend tests for empty buildah run command
    Fix formatting error in run.go
    Update buildah run to make command required
    Expanding buildah run cmd/entrypoint tests
    Update test cases for buildah run behaviour
    Remove buildah run cmd and entrypoint execution
    Add Files section with registries.conf to pertinent man pages
    tests/config: perfect test
    tests/from: add name test
    Do not print directly to stdout in Commit()
    Touch up auth test commands
    Force "localhost" as a default registry
    Drop util.GetLocalTime()
    Vendor in latest containers/image
    Validate host and container paths passed to --volume
    test/from: add add-host test
    Add --compress, --rm, --squash flags as a noop for bud
    Add FIPS mode secret to buildah run and bud
    Add config --comment/--domainname/--history-comment/--hostname
    'buildah config': stop replacing Created-By whenever it's not specified
    Modify man pages so they compile correctly in mandb
    Add description on how to do --isolation to buildah-bud man page
    Add support for --iidfile to bud and commit
    Refactor buildah bud for vendoring
    Fail if date or git not installed
    Revert update of entrypoint behaviour to match docker
    Vendor in latest imagebuilder code to fix multiple stage builds
    Add /bin/sh -c to entrypoint in config
    image_test: Improve the test
    Fix README example of buildah config
    buildah-image: add validation to 'format'
    Simple changes to allow buildah to pass make validate
    Clarify the use of buildah config options
    containers_test: Perfect testing
    buildah images and podman images are listing different sizes
    buildah-containers: add tests and example to the man page
    buildah-containers: add validation to 'format'
    Clarify the use of buildah config options
    Minor fix for lighttpd example in README
    Add tls-verification to troubleshooting
    Modify buildah rmi to account for changes in containers/storage
    Vendor in latest containers/image and containers/storage
    addcopy: add src validation
    Remove tarball as an option from buildah push --help
    Fix secrets patch
    Update entrypoint behaviour to match docker
    Display imageId after commit
    config: add support for StopSignal
    Fix docker login issue in travis.yml
    Allow referencing stages as index and names
    Add multi-stage builds tests
    Add multi-stage builds support
    Add accessor functions for comment and stop signal
    Vendor in latest imagebuilder, to get mixed case AS support
    Allow umount to have multi-containers
    Update buildah push doc
    buildah bud walks symlinks
    Imagename is required for commit atm, update manpage

## 0.16.0 (2018-04-08)
    Bump to v0.16.0
    Remove requires for ostree-lib in rpm spec file
    Add support for shell
    buildah.spec should require ostree-libs
    Vendor in latest containers/image
    bash: prefer options
    Change image time to locale, add troubleshooting.md, add logo to other mds
    buildah-run.md: fix error SYNOPSIS
    docs: fix error example
    Allow --cmd parameter to have commands as values
    Touchup README to re-enable logo
    Clean up README.md
    Make default-mounts-file a hidden option
    Document the mounts.conf file
    Fix man pages to format correctly
    Add various transport support to buildah from
    Add unit tests to run.go
    If the user overrides the storage driver, the options should be dropped
    Show Config/Manifest as JSON string in inspect when format is not set
    Switch which for that in README.md
    Remove COPR
    Fix wrong order of parameters
    Vendor in latest containers/image
    Remove shallowCopy(), which shouldn't be saving us time any more
    shallowCopy: avoid a second read of the container's layer

## 0.5 - 2017-11-07
    Add secrets patch to buildah
    Add proper SELinux labeling to buildah run
    Add tls-verify to bud command
    Make filtering by date use the image's date
    images: don't list unnamed images twice
    Fix timeout issue
    Add further tty verbiage to buildah run
    Make inspect try an image on failure if type not specified
    Add support for `buildah run --hostname`
    Tons of bug fixes and code cleanup

## 0.4 - 2017-09-22
### Added
    Update buildah spec file to match new version
    Bump to version 0.4
    Add default transport to push if not provided
    Add authentication to commit and push
    Remove --transport flag
    Run: don't complain about missing volume locations
    Add credentials to buildah from
    Remove export command
    Bump containers/storage and containers/image

## 0.3 - 2017-07-20
## 0.2 - 2017-07-18
### Added
    Vendor in latest containers/image and containers/storage
    Update image-spec and runtime-spec to v1.0.0
    Add support for -- ending options parsing to buildah run
    Add/Copy need to support glob syntax
    Add flag to remove containers on commit
    Add buildah export support
    update 'buildah images' and 'buildah rmi' commands
    buildah containers/image: Add JSON output option
    Add 'buildah version' command
    Handle "run" without an explicit command correctly
    Ensure volume points get created, and with perms
    Add a -a/--all option to "buildah containers"

## 0.1 - 2017-06-14
### Added
    Vendor in latest container/storage container/image
    Add a "push" command
    Add an option to specify a Create date for images
    Allow building a source image from another image
    Improve buildah commit performance
    Add a --volume flag to "buildah run"
    Fix inspect/tag-by-truncated-image-ID
    Include image-spec and runtime-spec versions
    buildah mount command should list mounts when no arguments are given.
    Make the output image format selectable
    commit images in multiple formats
    Also import configurations from V2S1 images
    Add a "tag" command
    Add an "inspect" command
    Update reference comments for docker types origins
    Improve configuration preservation in imagebuildah
    Report pull/commit progress by default
    Contribute buildah.spec
    Remove --mount from buildah-from
    Add a build-using-dockerfile command (alias: bud)
    Create manpages for the buildah project
    Add installation for buildah and bash completions
    Rename "list"/"delete" to "containers"/"rm"
    Switch `buildah list quiet` option to only list container id's
    buildah delete should be able to delete multiple containers
    Correctly set tags on the names of pulled images
    Don't mix "config" in with "run" and "commit"
    Add a "list" command, for listing active builders
    Add "add" and "copy" commands
    Add a "run" command, using runc
    Massive refactoring
    Make a note to distinguish compression of layers

## 0.0 - 2017-01-26
### Added
    Initial version, needs work
