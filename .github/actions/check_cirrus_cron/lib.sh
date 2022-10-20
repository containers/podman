
# Must be called from top-level of script, not another function.
err() {
    # Ref: https://docs.github.com/en/free-pro-team@latest/actions/reference/workflow-commands-for-github-actions
    echo "::error file=${BASH_SOURCE[1]},line=${BASH_LINENO[1]}::${1:-No error message given}"
    exit 1
}
