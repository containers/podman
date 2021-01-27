# Podman: Release process

The following process describes how to make a Podman release.


1. If this is a micro release on a release branch and requires backports from master,
   backport all necessary commits to the release branch.
1. Make a release notes PR.
   * Find all merged PRs since the last release (https://github.com/containers/podman/commits/master
   and switch to whatever the last branch we are working on is, then look for openshift-merge-robot which does
   all our merges).
   * Include all changes as a line in RELEASE_NOTES.md (https://github.com/containers/podman/blob/master/RELEASE_NOTES.md).
   * Be sure to link any issue the PR fixed. Do not include any PRs that are documentation or test changes only,
   and do not include any PRs that fix bugs that we introduced since the last release - if it was working in the
   previous release and still working in the current release we don’t need to mention it.
   * The file is divided into Features (new functionality), Changes (any significant changes from the way Podman used to behave),
   Bugfixes (self-explanatory), API (All features, changes, and bugfixes for the API are grouped here), and
   Misc (include any major library bumps here - Buildah, c/storage, c/common, etc).
   * If this is a release on a release branch, the change will need to go into both Master and the release branch.
1. Make a release branch
   1. If you are making a new major or minor release and need to make a release branch: once the release notes PR is merged,
   update your fork’s master to be up-to-date with upstream/master.
   1. (`git checkout master; git fetch upstream; git rebase upstream/master; git push origin master`)
   1. Make a new branch for the release based on master (`git checkout -b vX.Y`) and push this new branch, unmodified,
   to upstream (`git push upstream vX.Y`).
   1. The creation of the release branch should be done as part of a release
   candidate, to ensure that we curate all changes that land in the final release and no new features land after an RC.
   1. Cirrus-CI will likely fail on the new branch. Find it and cancel the run if you wish or ignore the failure
   (it will be fixed by update to $DEST_BRANCH below)
1. Creating a Release
   1. Fetch from upstream and check out a copy of the upstream release branch
      * (`git fetch upstream; git checkout upstream/vX.Y`)
   1. Check out a fresh branch to make the release in
      * (`git checkout -b bump_xyz`).
   1. Get the commit hash of the last release, and run `g=$HASH make changelog`. This will modify the `changelog.txt` file.
   1. Now, manually edit the file and change the first line (“Changelog for …”) to include the current release number.
      An example of the finished product: “- Changelog for v2.1.0 (2020-09-22):”
   1. Update `version/version.go`. Bump Version to whatever the new version is. If there were API changes, also
      bump APIVersion.
   1. Update `.cirrus.yml` to change `$DEST_BRANCH` to the new branch name (pushed above)
   1. Commit this and sign the commit (`git commit -a -s -S`). Your commit message should be “Bump to vX.Y.Z”.
   1. Tag this commit as a release by finding its hash and running `git tag -s -m 'vX.Y.Z' vX.Y.Z $HASH`.
   1. Again, change `version/version.go`. Bump micro version and add a `-dev` suffix to indicate this is a
      non-released version of Podman.
   1. Change `contrib/spec/podman.spec.in`, bumping micro in Version.
   1. Commit these changes (no need to sign the commit). Message should be “Bump to X.Y.Z-dev”
   1. Push your changes and make a PR against the release branch. You should have two commits that look very
      similar to https://github.com/containers/podman/pull/7787
   1. Assuming all automated tests pass, locate and press the "trigger" button on the "Optional Release Test" task.
      This will perform additional testing to confirm the code is release-worthy - should the PR be merged.
      This is the last point where automated testing failures can be addressed without requiring a new tag.
      Please seek assistance if needed, before proceeding. Do not merge without all automated testing, including
      the release test, passing.
   1. Once this PR merges, push the tag with `git push upstream vX.Y.Z`

1. Verify automated build and actual release-testing passes by going to:
   * `https://cirrus-ci.com/github/containers/podman/<vX.Y.Z>`
   * If there is a failure, be 100% sure you understand and accept the cause, given any code changes will require a new tag.

1. Bump master to new -dev version
   1. If you made a release branch and bumped major or minor version, complete steps 10 to 12 again but on the master
      branch, bumping minor version and setting micro to 0. So after release v2.2.0 master should be set to
      v2.3.0-dev. PR these changes against master.
   1. Make a PR to bump the version in README.md on master to reflect the new release.
1. Download the release artifacts (not necessary for release candidates)
   1. Given the new tag vX.Y.Z pushed (above), go to `https://cirrus-ci.com/github/containers/podman/<vX.Y.Z>`
   1. Visit each of the "Build for ..." tasks.
   1. Under the "Artifacts" section, click the "gosrc" item, find and download the release archive. For example
      "podman-release.tar.gz".
   1. You will need to rename these or otherwise track which distro/version they are for.
   1. Similarly, download the release archives for OS-X and Windows by navigating to the "OSX Cross" and
   "Windows Cross" tasks.  The files are located under the "gosrc" artifact item.
1. Sign the archives
   * In the directory where you downloaded the archives, run `sha256sum *.tar.gz *.zip *.msi > shasums` to generate SHA sums.
1. Upload the archives to the release page on github.
   1. Navigate to the release page for the new tag you pushed on Github (will be at the top of
   https://github.com/containers/podman/tags).
   1. Edit the release, changing its name to vX.Y.Z.
   1. If this is a release candidate be certain to click the pre-release checkbox at the bottom of the page.
   1. Copy and paste the release notes for the release into the body of the release.
   1. Near the bottom of the Edit Release page there is a box with the message “Add binaries by dropping them here or selecting them” -
   click that and upload the following binaries that you just built from the bin/ folder of your checked-out
   repository:
      * podman-remote-release-darwin.zip
      * podman-remote-release-windows.zip
      * podman-remote-static.tar.gz
      * podman-vX.Y.Z.msi
      * shasums
   1. Save the release.
