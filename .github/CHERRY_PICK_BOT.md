# Cherry-Pick Bot

Automatically cherry-picks merged PR commits to release branches using labels.

## Usage

1. Add a label `cherry-pick-<branch>` to your PR before or after merging
2. When the PR merges to `main`, the bot cherry-picks the commit to the target branch

### Label Format

```
cherry-pick-<branch>
```

Where `<branch>` is the target release branch name.

**Examples:**
- `cherry-pick-v5.8` - cherry-picks to branch `v5.8`
- `cherry-pick-v5.7` - cherry-picks to branch `v5.7`

### Multiple Branches

Add multiple labels to cherry-pick to multiple branches:

```
cherry-pick-v5.8
cherry-pick-v5.7
```

The bot processes each label sequentially.

## When It Runs

The bot triggers when:
- A labeled PR is merged to `main`
- A `cherry-pick-*` label is added to an already-merged PR

## Success

On successful cherry-pick, the commit is pushed directly to the target branch. The original commit author is preserved.

## Failures

### Merge Conflicts

If cherry-pick fails due to conflicts, the bot comments on the PR with manual instructions:

```bash
git fetch origin
git checkout v5.8
git cherry-pick <sha> -m 1
# resolve conflicts
git push origin v5.8
```

### Missing Branch

If the target branch doesn't exist, the bot comments:

```
Cherry-pick failed: branch `v5.8` does not exist.
```

## Requirements

- The PR must be merged (labels on open PRs are ignored)
- The target branch must exist
- The cherry-pick must apply cleanly (no conflicts)
