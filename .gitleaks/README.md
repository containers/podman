## Secret Scanning

### Overview

During the course of submitting a pull-request, it's possible that a
malicious-actor may try to reference and exfiltrate sensitive CI
values in their commits. This activity can be obscured relatively
easily via multiple methods.

Secret-scanning is an automated process for examining commits for
any mention, addition, or changes to potentially sensitive values.
It's **not** a perfect security solution, but can help thwart naive
attempts and innocent accidents alike.

### Mechanism

Whenever a PR is pushed to, an automated process will scan all
of it's commits.  This happens from an execution context outside
of the influence from any changes in the PR.  When a detection is
made, the automated job will fail, and an e-mail notification will
be sent for review.  The email is necessary because these jobs
aren't otherwise monitored, and a malicious-actor may attempt
to cover their tracks.

### Notifications

Scan failure notification e-mails are necessary because the
detection jobs may not be closely monitored.  This makes it
more likely malicious-actions may go unnoticed.  However,
project maintainers may bypass notification by adding a
`BypassLeakNotification` label to a PR that would otherwise.

### Configuration

The meaning of a "sensitive value" is very repository specific.
So addition of any new sensitive values to automation must be
reflected in the GitLeaks configuration.  This change must
land on the repository's main branch before detection will be
active.

Additionally, there are sets of known sensitive values (e.g.
ssh keys) which could be added or referenced in the future.
To help account for this, GitLeaks is configured to reads an
upstream source of well-known, common patterns that are
automatically included in all scans.  These can **NOT** be
individually overridden by the repository configuration.

### Baseline

At times, there may be deliberate use/change of sensitive
values.  This is accounted for by referencing a "baseline" of
known detections.  For GitLeaks, new possible baseline items are
present within the detection report JSON (artifact) produced
with every automated execution.

Baseline items may also be produced by locally, by running the
GitLeaks container (from the repo. root) using a command similar
to:

```
$ git_log_options="-50"
$ podman run --rm \
    --security-opt=label=disable \
    --userns=keep-id:uid=1000,gid=1000 \
    -v $PWD:/subject:ro \
    -v $PWD:/default:ro \
    ghcr.io/gitleaks/gitleaks:latest \
    detect \
    --log-opts="$git_log_options" \
    --source=/subject \
    --config=/default/.gitleaks/config.toml \
    --report-path=/dev/stdout \
    --baseline-path=/default/.gitleaks/baseline.json
```

You may then copy-paste the necessary JSON list items into
the baseline file.  In order to be effective on current or
future pull-requests, changes to the baseline or configuration
**MUST** be merged onto the `main` branch.

### Important notes

* The `.gitleaks.toml` file **must** be in the root of the repo.
  due to it being used by third-party tooling.

* The scan will still run on PRs with a 'BypassLeakNotification' label.
  This is intended to help in scanning test-cases and where updates
  to the baseline are needed.

* Detection rules can be fairly complex, but they don't need to be.
  When adding new rules, please be mindful weather or not they **REALLY**
  are only valid from specific files.  Also be mindful of potentially
  generic names, for example 'debug' or 'base64' that may match
  code comments.

* Baseline items are commit-specific!  Meaning, if the commit which
  caused the alert changes, the baseline-detection item will no-longer
  match. Fixing this will likely generate additional e-mail notification
  spam, so please be mindful of your merge and change sequence, as well
  and content.

* There is an additional execution of the GitLeaks scanner during the
  "pre-build" phase of Cirrus-CI.  Results from this run are **NOT**
  to be trusted in any way.  This check is only present to catch
  potential configuration or base-line data breaking changes.

* Scans **are** configured to execute against all branches and new
  tags.  Because the configuration and baseline data is always sourced
  from `main`, this check is necessary to alert on changed, non-conforming
  commits.  Just remember to make any needed corrections on the `main`
  branch configuration or baseline to mitigate.
