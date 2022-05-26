# Kubernetes Release Notes Generator

This directory contains a tool called `release-notes` and a set of library utilities at which aim to provide a simple and extensible set of tools for fetching, contextualizing, and rendering release notes for the [Kubernetes](https://github.com/kubernetes/kubernetes) repository.

## Install

The simplest way to install the `release-notes` CLI is via `go get`:

```
GO111MODULE=on go get k8s.io/release/cmd/release-notes
```

This will install `release-notes` to `$(go env GOPATH)/bin/release-notes`.

## Usage

To generate release notes for a commit range, run:

```bash
$ export GITHUB_TOKEN=a_github_api_token
$ release-notes \
  --start-sha 02dc3d713dd7f945a8b6f7ef3e008f3d29c2d549 \
  --end-sha   23649560c060ad6cd82da8da42302f8f7e38cf1e

level=info timestamp=2019-07-30T04:02:30.9452687Z caller=main.go:139 msg="fetching all commits. this might take a while..."
level=info timestamp=2019-07-30T04:02:43.8454168Z caller=notes.go:446 msg="[1/1679 - 0.06%]"
...
level=info timestamp=2019-07-30T04:09:30.3491553Z caller=notes.go:446 msg="[1679/1679 - 100.00%]"
level=info timestamp=2019-07-30T04:11:38.8033378Z caller=main.go:159 msg="got the commits, performing rendering"
level=info timestamp=2019-07-30T04:11:38.8059129Z caller=main.go:228 msg="release notes written to file" path=/tmp/release-notes-509576676 format=markdown
```

You can also generate the raw notes data into JSON. You can then use a variety of tools (such as `jq`) to slice and dice the output:

```json
[
  {
    "text": "fixed incorrect OpenAPI schema for CustomResourceDefinition objects",
    "author": "liggitt",
    "author_url": "https://github.com/liggitt",
    "pr_url": "https://github.com/kubernetes/kubernetes/pull/65256",
    "pr_number": 65256,
    "kinds": [
      "bug"
    ],
    "sigs": [
      "api-machinery"
    ]
  }
]
```

if you would like to debug a run, use the `--debug` flag:

```bash
$ export GITHUB_TOKEN=a_github_api_token
$ release-notes \
  --start-sha 02dc3d713dd7f945a8b6f7ef3e008f3d29c2d549 \
  --end-sha   23649560c060ad6cd82da8da42302f8f7e38cf1e \
  --debug 

level=debug timestamp=2019-07-30T04:02:43.8453116Z caller=notes.go:445 msg=################################################
level=info timestamp=2019-07-30T04:02:43.8454168Z caller=notes.go:446 msg="[1/1679 - 0.06%]"
level=debug timestamp=2019-07-30T04:02:43.8454701Z caller=notes.go:447 msg="Processing commit" func=ListCommitsWithNotes sha=23649560c060ad6cd82da8da42302f8f7e38cf1e
level=debug timestamp=2019-07-30T04:02:44.3711956Z caller=notes.go:464 msg="Obtaining PR associated with commit sha '23649560c060ad6cd82da8da42302f8f7e38cf1e'." func=ListCommitsWithNotes prno=80301 prbody="**What type of PR is this?**\r\n> Uncomment only one ` /kind <>` line, hit enter to put that in a new line, and remove leading whitespaces from that line:\r\n>\r\n> /kind api-change\r\n> /kind bug\r\n\r\n/kind cleanup\r\n\r\n> /kind design\r\n> /kind documentation\r\n> /kind failing-test\r\n> /kind feature\r\n> /kind flake\r\n\r\n**What this PR does / why we need it**:\r\n\r\nBased on the feedback from https://docs.google.com/document/d/1g5Aqa0BncQGRedSJH0TJQWq3mw3VxpJ_ufO1qokJ1LE we have decided to rename the `preferred` policy of the `TopologyManager` to `best-effort`.  The reasoning for this is outlined in the document.\r\n\r\nSince this change is coming before the `TopologyManager` was ever part of a release, this does not introduce a user-facing change.\r\n\r\n**Does this PR introduce a user-facing change?**:\r\n<!--\r\nIf no, just write \"NONE\" in the release-note block below.\r\nIf yes, a release note is required:\r\nEnter your extended release note in the block below. If the PR requires additional action from users switching to the new release, include the string \"action required\".\r\n-->\r\n```release-note\r\nNONE\r\n```"
level=debug timestamp=2019-07-30T04:02:44.3716249Z caller=notes.go:497 msg="Excluding notes for PR based on the exclusion filter." func=ListCommitsWithNotes filter="(?i)```(release-note\\s*)?('|\")?(none|n/a)?('|\")?\\s*```"
...
```

## Options

| Flag                    | Env Variable    | Default Value       | Required | Description                                                                                                                       |
| ----------------------- | --------------- | ------------------- | -------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **GITHUB REPO OPTIONS** |
|                         | GITHUB_TOKEN    |                     | Yes      | A personal GitHub access token                                                                                                    |
| org                     | ORG             | kubernetes          | Yes      | Name of GitHub organization                                                                                                       |
| repo                    | REPO            | kubernetes          | Yes      | Name of GitHub repository                                                                                                         |
| required-author         | REQUIRED_AUTHOR | k8s-ci-robot        | Yes      | Only commits from this GitHub user are considered. Set to empty string to include all users                                       |
| branch                  | BRANCH          | master              | Yes      | The GitHub repository branch to scrape                                                                                            |
| start-sha               | START_SHA       |                     | Yes      | The commit hash to start processing from (inclusive)                                                                              |
| end-sha                 | END_SHA         |                     | Yes      | The commit hash to end processing at (inclusive)                                                                                  |
| github-base-url         | GITHUB_BASE_URL |                     | No       | The base URL of Github              |
| github-upload-url       | GITHUB_UPLOAD_URL |                   | No       | The upload URL of enterprise Github |
| repo-path               | REPO_PATH       | /tmp/k8s-repo       | No       | Path to a local Kubernetes repository, used only for tag discovery                                                                |
| start-rev               | START_REV       |                     | No       | The git revision to start at. Can be used as alternative to start-sha                                                             |
| env-rev                 | END_REV         |                     | No       | The git revision to end at. Can be used as alternative to end-sha                                                                 |
| discover                | DISCOVER        | none                | No       | The revision discovery mode for automatic revision retrieval (options: none, mergebase-to-latest, patch-to-patch, patch-to-latest, minor-to-minor) |
| release-bucket          | RELEASE_BUCKET  | kubernetes-release  | No       | Specify gs bucket to point to in generated notes (default "kubernetes-release")                                                   |
| release-tars            | RELEASE_TARS    |                     | No       | Directory of tars to sha512 sum for display                                                                                       |
| **OUTPUT OPTIONS**      |
| output                  | OUTPUT          |                     | No       | The path where the release notes will be written                                                                                  |
| format                  | FORMAT          | markdown            | No       | The format for notes output (options: json, markdown)                                                                             |
| markdown-links          | MARKDOWN_LINKS  | false               | No       | Add links for PRs and authors in the markdown format. This is useful when the release notes are outputted to a file. When using the GitHub release page to publish release notes, this option should be set to false to take advantage of Github's autolinked references (options: true, false)                                                                               |
| go-template             | GO_TEMPLATE     | go-template:default | No       | The go template if `--format=markdown` (options: go-template:default, go-template:inline:<template-string> go-template:<file.template>) |
| dependencies            |                 | true                | No       | Add dependency report                                                                                                             |
| **LOG OPTIONS**         |
| debug                   | DEBUG           | false               | No       | Enable debug logging (options: true, false)                                                                                       |

## Building From Source

To build the `release-notes` tool, check out this repo to your `$GOPATH`:

```
git clone git@github.com:kubernetes/release.git $(go env GOPATH)/src/k8s.io/release
```

Run the following from the root of the repository to build the `release-notes` binary:

```
go install ./cmd/release-notes
```

## FAQ

### What do generated notes look like?

Check out the rendering of 1.11's release notes [here](https://gist.github.com/marpaia/acfdb889f362195bb683e9e09ce196bc).

### What formats are supported?

Right now the tool can output release notes in Markdown and JSON. The tool
also supports arbitrary formats using go-templates. The template has access
to fields in the `Document` struct. For an example, see the default markdown
template (`pkg/notes/internal/template.go`) used to render the stock format.

