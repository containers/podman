(FIXME: Should this file live elsewhere?)

# Libpod/Podman/Podman-Remote Automated Release Workflow

1. Open a new PR.

   * The first commit (`HEAD^2`) *must* update the release-notes for the new version, along with
     any other required documentation.  Automation *will* verify changes were made to
     `RELEASE_NOTES.md`, `README.md`, and  `TBD: FIXME`.

   * The second commit (`HEAD^1`) *must* increment the version number in all required code-files.
     Automation *will* verify the commit message summary line matches the regular expression:

     ```
     Release v\d+\.\d+
     ```

   * Automation will verify changes have been made to the following files:
     `version/version.go`, `contrib/spec/podman.spec.in`, and `TBD: FIXME`

   * The third commit (`HEAD`) *must* increment only the third, minor (least-significant)
     component of the version number (possibly including a `-suffix` string).
     Automation *will* verify that the two most-significant version components have
     **NOT** changed as compared to `HEAD^1`.  The commit summary *must* match
     the following regular expression:

     ```
     Development Version (v\d+\.\d+(\.\d+(-[\w\-\.]+)?)?)
     ```

   * Automation *shll not* examine or be sensitive to any other commits in the PR.

   * The first three commits in the PR *must* be GPG signed with an approved public
     key.  This is signified by they key's fingerprint being present in the file at
     `https://FIXME.com/FIXME/FIXME_WHITELIST.txt` on the repositories default branch.

2. Automation *must not* be sensitive to the presence/absence of any other commits
   contained in the release PR, beyond HEAD.  However, all commits *must* pass
   automated testing *and* be fast-forward merge-able to the relevant branch.

4. When deemed ready, the release PR is merged by humans or automation, and *must* (again)
   successfully pass all automated testing on the branch.

   * Upon failure and/or detectable abnormal conditions, notifications will be made to the
     #podman channel on Freenode IRC *and* by e-mail to the recipients specified in the file
     at `https://FIXME.com/FIXME/FIXME_EMAIL.txt`

   * An unlimited number of opportunities *will* be provided by automation, to re-test the
     branch.  In all cases, feedback *will* be provided as to the fitness of the automated
     release workflow for the existing content.

   * No further branch commits will be considered for the new release.  If changes are needed
     (for example to correct an undetected bug), the process must begin again with the next
     symentic version.  ***Note:*** In this special case, the `Prior Release` reference text
     of all future commits *must not* reference the errant version.

5. Assuming branch-testing passes automated checks.

   * A new tag and branch *will* be created by automation, referencing the "Release vX.Y.Z"
     commit.

   * The tag *will* be named like `vX.Y.Z[-extra]` and the branch *will* be named `vX.Y`.

   * The tag *will* be GPG signed by automation using a key accepted by the core maintainers.
     The tag annotation (message) *shall* be a string of the following format:
     ``<prior release> -> <tag name> from <originating branch>``.

   * Automation *may* create a Github-Release object, reflecting the name of the tag.  All
     available build-time artifacts from branch-testing *may* be uploaded to the object.


# Post-release Checklist

0. Go to [quay/libpod/gate "builds" page](https://quay.io/repository/libpod/gate?tab=builds).
   Verify a 'gate' image build is running or completed successfully for the new branch.

1. Go to [quay/libpod/in_podman "builds" page](https://quay.io/repository/libpod/in_podman?tab=builds),
   add a new build trigger for the new branch. Follow same settings as existing build triggers.

2. Send e-mail to <TBD> with release details

3. ...other TBD stuff...
