# Podman Releases

## Overview

Podman (and podman-remote) versioning is mostly based on [semantic-versioning
standards](https://semver.org).
Significant versions
are tagged, including *release candidates* (`rc`).
All relevant **minor** releases (`vX.Y`) have their own branches.  The **latest**
development efforts occur on the *main* branch.  Branches with a
*rhel* suffix are use for long-term support of downstream RHEL releases.

## Release workflow expectations

* You have push access to the [upstream podman repository](https://github.com/containers/podman.git)
* You understand all basic `git` operations and concepts, like creating commits,
  local vs. remote branches, rebasing, and conflict resolution.
* You have access to your public and private *GPG* keys.
* You have reliable internet access (i.e. not the public WiFi link at McDonalds)
* Other podman maintainers are online/available for assistance if needed.
* For a **major** release, you have 4-8 hours of time available, most of which will
  be dedicated to writing release notes.
* For a **minor** or **patch** release, you have 2-4 hours of time available
  (minimum depends largely on the speed/reliability of automated testing)
* You will announce the release on the proper platforms
  (i.e. Podman blog, Twitter, Mastodon Podman and Podman-Desktop mailing lists)

# Prechecks

Two days before actually cutting a release (including RCs), send an announcement to the
[podman-desktop](mailto:podman-desktop@lists.podman.io)
mailing list about the upcoming release. This will help the Podman Desktop team test and schedule
their own new release.

# Releases

## Major (***X***.y.z) release

These releases always begin from *main*, and are contained in a branch
named with the **major** and **minor** version. **Major** release branches
begin in a *release candidate* phase, with prospective release tags being
created with an `-rc` suffix.  There may be multiple *release candidate*
tags before the final/official **major** version is tagged and released.

## Significant minor (x.**Y**.z) and patch (x.y.**Z**) releases

Significant **minor** and **patch** level releases are normally
branched from *main*, but there are occasional exceptions.
Additionally, these branches may be named with `-rhel` (or another)
suffix to signify a specialized purpose.  For example, `-rhel` indicates
a release intended for downstream *RHEL* consumption.

## Unreleased Milestones

Non-release versions may occasionally appear tagged on a branch, without
the typical (major) receive media postings or artifact distribution.  For
example, as required for the (separate) RHEL release process.  Otherwise
these tags are simply milestones of reference purposes and may
generally be safely ignored.

## Process

***Note:*** This is intended as a guideline, and generalized process.
Not all steps are applicable in all situations.  Not all steps are
spelled with complete minutiae.

1. Create a new upstream release branch (if none already exist).

   1. Check if a release branch is needed. Typically, major and minor version bumps
      should be branched sometime during the release candidate phase. Patch
      releases typically already have a branch created.
      Branching ensures all changes are curated before inclusion in the
      release, and no new features land after the *release-candidate* phases
      are complete.
   1. Ensure your local clone is fully up to date with the remote upstream
      (`git remote update`).  Switch to this branch (`git checkout upstream/main`).
   1. Make a new local branch for the release based on *main*. For example,
      `git checkout -b vX.Y`.  Where `X.Y` represent the complete release
      version-name, including any suffix (if any) like `-rhel`.  ***DO NOT***
      include any `-rc` suffix in the branch name.
   1. Edit the `.cirrus.yml` file, changing the `DEST_BRANCH` value (under the
      `env` section) to the new, complete branch name (e.x. `vX.Y`).
       Commit and sign, using the description
      `Cirrus: Update operating branch`.
   1. Push the new branch otherwise unmodified (`git push upstream vX.Y`).
   1. Automation will begin executing on the branch immediately.  Because
      the repository allows out-of-sequence PR merging, it is possible that
      merge order introduced bugs/defects.  To establish a clean
      baseline, observe the initial CI run on the branch for any unexpected
      failures.  This can be done by going directly to
      `https://cirrus-ci.com/github/containers/podman/vX.Y`
   1. If there are CI test or automation boops that need fixing on the branch,
      attend to them using normal PR process (to *main* first, then backport
      changes to the new branch).  Ideally, CI should be "green" on the new
      branch before proceeding.

   1. Create a new branch-verification Cirrus-Cron entry.

      1. This is to ensure CI's VM image timestamps are refreshed.  Without this,
         the VM images ***will*** be permanently pruned after 60 days of inactivity
         and are hard/impossible to re-create accurately.
      1. Go to
         [https://cirrus-ci.com/github/containers/podman](https://cirrus-ci.com/github/containers/podman)
         and press the "gear" (Repository Settings) button on the top-right.
      1. At the bottom of the settings page is a table of cron-job names, branches,
         schedule, and recent status.  Below that is an editable new-entry line.
      1. Set the new job's `name` and `branch` to the name of new release branch.
      1. Set the `expression` using the form `X X X ? * 1-6` where 'X' is a number
         between 0-23 and not already taken by another job in the table.  The 1-hour
         interval is used because it takes about that long for the job to run.
      1. Add the new job by pressing the `+` button on the right-side of the
         new-entry line.


1. Create a new local working-branch to develop the release PR
   1. Ensure your local clone is fully up to
      date with the remote upstream (`git remote update`).
   1. Create a local working branch based on `upstream/main` or the correct upstream branch.
      Example: `git checkout -b bump_vX.Y.Z --no-track upstream/vX.Y`

1. Compile release notes.

   1. Ensure any/all intended PR's are completed and merged prior to any
      processing of release notes.
   1. Find all commits since the last release. There is a script, `/hack/branch_commits.rb`
      that is helpful for finding all commits in one branch, but not in another,
      accounting for cherry-picks. Commits in base branch that are not in
      the old branch will be reported. `ruby branch_commits.rb upstream/main upstream/vX.Y`
      Keep this list open/available for reference as you edit.
   1. Edit `RELEASE_NOTES.md`

      * Add/update the version-section of with sub-sections for *Features*
        (new functionality), *Changes* (Altered podman behaviors),
        *Bugfixes* (self-explanatory), *API* (All related features,
        changes, and bugfixes), and *Misc* (include any **major**
        library bumps, e.g. `c/buildah`, `c/storage`, `c/common`, etc).
      * Use your merge-bot reference PR-listing to examine each PR in turn,
        adding an entry for it into the appropriate section.
      * Use the list of commits to find the PR that the commit came from.
        Write a release note if needed.

        * Use the release note field in the PR as a guideline.
          It may be helpful but also may need rewording for consistency.
          Some PR's with a release note field may not need one, and some PR's
          without a release note field may need one.
        * Be sure to link any issue the PR fixed.
        * Do not include any PRs that are only documentation or test/automation
          changes.
        * Do not include any PRs that fix bugs which we introduced due to
          new features/enhancements.  In other words, if it was working, broke, then
          got fixed, there's no need to mention those items.

   1. Commit the `RELEASE_NOTES.md` changes, using the description
      `Create release notes for vX.Y.Z` (where `X`, `Y`, and `Z` are the
      actual version numbers).
   1. Open a Release Notes PR, or include this commit with the version bump PR
       * If you decide to open a PR with just release notes, make sure that
         the commit has the prefix `[CI:DOCS]` to avoid triggering
         lengthy automated testing.
       * Otherwise, the release notes commit can also be included in the
         following release PR.

1. Update version numbers and push tag

   1. Edit `version/rawversion/version.go` and bump the `Version` value to the new
      release version.  If there were API changes, also bump `APIVersion` value.
      Make sure to also bump the version in the swagger.yaml `pkg/api/server/docs.go`
      For major and minor versions also add the new branch name to
      `docs/source/Reference.rst` to show the new swagger version on docs.podman.io.
   1. Commit this and sign the commit (`git commit -a -s -S`). The commit message
      should be `Bump to vX.Y.Z` (using the actual version numbers).
   1. Push this single change to your GitHub fork, and make a new PR,
      **being careful** to select the proper release branch as its base.
   1. Wait for all automated tests pass (including on an RC-branch PR).  Re-running
      and/or updating code as needed.
   1. In the PR, under the *Checks* tab, locate and clock on the Cirrus-CI
      task `Optional Release Test`.  In the right-hand window pane, click
      the `trigger` button and wait for the test to go green.  *This is a
      critical step* which confirms the commit is worthy of becoming a release.
   1. Tag the `Bump to vX.Y.Z` commit as a release by running
      `git tag -s -m 'vX.Y.Z' vX.Y.Z $HASH` where `$HASH` is specified explicitly
      and carefully, to avoid (basically) unfixable accidents (if they are pushed).
   1. Change `version/rawversion/version.go` again. This time, bump the **patch** version and
      re-add the `-dev` suffix to indicate this is a non-released version of Podman.
   1. Change `contrib/spec/podman.spec.in`, bumping **patch** number of `Version`.
   1. Commit these changes with the message `Bump to X.Y.Z-dev`.
   1. Push your local branch to your GitHub fork (and the PR) again.
   1. The PR should now have two commits that look very similar to
      https://github.com/containers/podman/pull/7787
      Note: Backports and release note commits may also be included in the release PR.
   1. Wait for at least all the "Build" and "Verify" (or similar) CI Testing
      steps to complete successfully.  No need to wait for complete integration
      4and system-testing (it was already done on substantially the same code, above).
   1. Merge the PR (or ask someone else to review and merge, to be safer).
   1. **Note:** This is the last point where any test-failures can be addressed
      by code changes. After pushing the new version-tag upstream, no further
      changes can be made to the code without lots of unpleasant efforts.  Please
      seek assistance if needed, before proceeding.

   1. Assuming the "Bump to ..." PR merged successfully, and you're **really**
      confident the correct commit has been tagged, push it with
      `git push upstream vX.Y.Z`

1. Locate, Verify release testing is proceeding

   1. When the tag was pushed, an automated build was created. Locate this
      by starting from
      `https://github.com/containers/podman/tags` and finding the recent entry
      for the pushed tag.  Under the tag name will be a timestamp and abbrieviated
      commit hash, for example `<> 5b2585f`.  Click the commit-hash link.
   1. In the upper-left most corner, just to the left of the "Bump to vX.Y"
      text, will be a small status icon (Yellow circle, Red "X", or green check).
      Click this, to open a small pop-up/overlay window listing all the status
      checks.
   1. In the small pop-up/overlay window, press the "Details" link on one of the
      Cirrus-CI status check entries (doesn't matter which one).
   1. On the following page, in the lower-right pane, will be a "View more details
      on Cirrus CI" link, click this.
   1. A Cirrus-CI task details page will open, click the button labeled
      "View All Tasks".
   1. Keep this page open to monitor its progress and for use in future steps.

1. Bump main `-dev` version

   1. If you made a release branch and bumped **major** or **minor** version
      Complete the "Update version numbers and push tag" steps above on the
      *main* branch.  Bump the **minor** version and set the **patch**
      version to 0.  For example, after pushing the v2.2.0 release, *main*
      should be set to v2.3.0-dev.
   1. Create a "Bump to vX.Y.Z-dev" commit with these changes.
   1. Update `RELEASE_NOTES.md` on main. Commit these changes.
   1. Create a PR with the above commits, and oversee it's merging.

1. Create GitHub Release entry and upload assets

   1. Return to the Cirrus-CI Build page for the new release tag, confirm
      (or wait for) it to complete, re-running any failed tasks as appropriate.
   1. Go to `https://github.com/containers/podman/releases/tag/vX.Y.Z` and
      press the "Edit Release" button.  Change the name to the form `vX.Y.Z`
   1. If this is a release candidate be certain to click the pre-release
      checkbox at the bottom of the page.
   1. If this new release will be the latest version released, be certain to
      click the latest release checkbox at the bottom of the page.
   1. Copy and paste the release notes for the release into the body of
      the release.
   1. Click the Publish button to make the release (or pre-release)
      available.
   1. For all releases, including RC's, artifacts should be published. The
      release-artifacts, upload-win-installer, and mac-pkg GitHub Actions
      should automatically take care of building, signing, and uploading artifacts.
      Check the "Actions" tab, after publishing you should see the jobs running.
      There may be more than one running due to the multiple
      event states triggered, but this can be ignored, as any duplicates
      will gracefully back-off. The job takes 5-6 minutes to complete.

      Please note that the Windows action depends on the artifact action, and will be
      triggered after the artifact action succeeds.

      If any of these actions are somehow not triggered, you can manually trigger them

      ### *CLI Approach*
      1. Install the [GitHub CLI](https://github.com/cli/cli#installation)
      1. Run (replacing below version number to release version)
         ```
         gh workflow run "ACTION NAME" -F version="vX.Y.Z"
         ```
      ### *GUI Approach*
      1. Go to the "Actions" tab
      1. On the left pick the required action to be triggered.
      1. A blue box will appear above the job list with a right side drop
         -down. Click the drop-down and specify the version number in the
         dialog that appears
   1. Check that all following artifacts are now attached to the release
         * podman-remote-release-darwin_amd64.zip
         * podman-remote-release-darwin_arm64.zip
         * podman-remote-release-windows_amd64.zip
         * podman-vX.Y.Z.msi
         * podman-remote-static-linux_amd64.tar.gz
         * podman-remote-static-linux_arm64.tar.gz
         * podman-installer-macos-amd64.pkg
         * podman-installer-macos-arm64.pkg
         * podman-5.2.1-setup.exe
         * shasums

1. Update Cirrus-CI cron job list
   1. After any Major or significant minor (esp. `-rhel`) releases, it's critical to
      maintain the Cirrus-CI cron job list.  This applies to all containers-org repos,
      not just podman.
   1. Access the repo. settings WebUI by navigating to
      `https://cirrus-ci.com/github/containers/<repo name>`
      and clicking the gear-icon in the upper-right.
   1. For minor (i.e. **NOT** `-rhel`) releases, (e.x. `vX.Y`), the previous release
      should be removed from rotation (e.x. `vX.<Y-1>`) assuming it's no longer supported.
      Simply click the trash-can icon to the right of the job definition.
   1. For `-rhel` releases, these are tied to products with specific EOL dates.  They should
      *never* be disabled unless you (and a buddy) are *absolutely* certain the product is EOL
      and will *never* ever see another backport (CVE or otherwise).
   1. On the settings page, pick a "less used" time-slot based on the currently defined
      jobs.  For example, if three jobs specify `12 12 12 ? * 1-6`, choose another.  Any
      spec. `H`/`M`/`S` value between 12 and 22 is acceptable (e.x. `22 22 22 ? * 1-6`).
      The point is to not overload the clouds with CI jobs.
   1. Following the pattern of the already defined jobs, at the bottom of the settings
      page add a new entry.  The "Name" should reflect the version number, the "Branch"
      is simply the newly created release branch name (must be exact), and the "Expression"
      is the time slot you selected (copy-paste).
   1. Click the "+" button next to the new-job row you just filled out.

1. Announce the release
      1. For major and minor releases, write a blog post and publish it to blogs.podman.io
         Highlight key features and important changes or fixes. Link to the GitHub release.
         Make sure the blog post is properly tagged with the Announcement, Release, and Podman tags,
         and any other appropriate tags.
      1. For all releases, including patch releases and RC's, send an email to the [podman](mailto:podman@lists.podman.io) and [podman-desktop](mailto:podman-desktop@lists.podman.io) mailing lists. This should be automated by the release-artifacts
      action, but it's best to keep and eye on it to see if the email went through to the lists.
         Link the to release blog and GitHub release.
      1. Update [LATEST_VERSION](https://github.com/containers/podman.io/blob/main/static/data/global.ts) on the Podman.io website.
      1. Tweet the release. Make a Mastodon post about the release.
      1. RC's can also be announced if needed.
