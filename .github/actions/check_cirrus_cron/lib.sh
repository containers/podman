

# Send text to stderr
msg() {
    echo "$@" > /dev/stderr
}

# Must be called from top-level of script, not another function.
err() {
    # Ref: https://docs.github.com/en/free-pro-team@latest/actions/reference/workflow-commands-for-github-actions
    msg "::error file=${BASH_SOURCE[1]},line=${BASH_LINENO[0]}::$@"
    exit 1
}

# Using python3 here is a compromise for readability and
# properly handling quote, control and unicode character encoding.
escape_query() {
    local json_string
    # Assume it's okay to squash repeated whitespaces inside the query
    json_string=$(printf '%s' "$1" | \
                  tr --delete '\r\n' | \
                  tr --squeeze-repeats '[[:space:]]' | \
        python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))')
    # The $json_string in message is already quoted
    echo -n "$json_string"
}

# Given a GraphQL query/mutation, fire it at the API.
# and return the output on stdout.  The optional
# second parameter may contain a jq filter-string.
# When provided, if the GQL result is empty, null,
# fails to parse, or does not match the filter-string,
# non-zero will be returned.
gql() {
    local e_query query
    e_query=$(escape_query "$1")
    query="{\"query\": $e_query}"
    local filter
    filter="$2"
    local output
    local filtered
    msg "::group::Posting GraphQL Query and checking result"
    msg "query: "
    if ! jq -e . <<<"$query" > /dev/stderr; then
        msg "::error file=${BASH_SOURCE[1]},line=${BASH_LINENO[0]}::Invalid query JSON: $query"
        return 1
    fi
    if output=$(curl \
              --request POST \
              --silent \
              --show-error \
              --location \
              --header 'content-type: application/json' \
              --header "Authorization: Bearer $SECRET_CIRRUS_API_KEY" \
              --url 'https://api.cirrus-ci.com/graphql' \
              --data "$query") && [[ -n "$output" ]]; then

        if filtered=$(jq -e "$filter" <<<"$output") && [[ -n "$filtered" ]]; then
            msg "result:"
            # Make debugging easier w/ formatted output
            # to stderr for display, stdout for consumption by caller
            jq --indent 2 . <<<"$output" | tee /dev/stderr
            msg "::endgroup::"
            return 0
        fi

        msg "::error file=${BASH_SOURCE[1]},line=${BASH_LINENO[0]}::Query result did not pass filter '$2': '$output'"
        msg "::endgroup::"
        return 2
    fi

    msg "::error file=${BASH_SOURCE[1]},line=${BASH_LINENO[0]}::Query failed or result empty: '$output'"
    msg "::endgroup::"
    return 3
}
