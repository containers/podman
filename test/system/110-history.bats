#!/usr/bin/env bats

load helpers

@test "podman history - basic tests" {
    tests="
                                 | .*[0-9a-f]\\\{12\\\} .* CMD .* LABEL
--format '{{.ID}} {{.Created}}'  | .*[0-9a-f]\\\{12\\\} .* ago
--human=false                    | .*[0-9a-f]\\\{12\\\} *[0-9-]\\\+T[0-9:]\\\+Z
-qH                              | .*[0-9a-f]\\\{12\\\}
--no-trunc                       | .*[0-9a-f]\\\{64\\\}
"

    parse_table "$tests" | while read options expect; do
        if [ "$options" = "''" ]; then options=; fi

        eval set -- "$options"

        run_podman history "$@" $IMAGE
        is "$output" "$expect" "podman history $options"
    done
}

@test "podman history - custom format" {
    run_podman history --format "{{.ID}}\t{{.ID}}" $IMAGE
    od -c <<<$output
    while IFS= read -r row; do
        is "$row" ".*	.*$"
    done <<<$output
}

@test "podman history - json" {
    # Sigh. Timestamp in .created can be '...Z' or '...-06:00'
    tests="
id        | [0-9a-f]\\\{64\\\}
created   | [0-9-]\\\+T[0-9:.]\\\+[Z0-9:+-]\\\+
size      | -\\\?[0-9]\\\+
"

    run_podman history --format json $IMAGE

    parse_table "$tests" | while read field expect; do
        # HACK: we can't include '|' in the table
        if [ "$field" = "id" ]; then expect="$expect\|<missing>";fi

        # output is an array of dicts; check each one
        count=$(echo "$output" | jq '. | length')
        i=0
        while [ $i -lt $count ]; do
            actual=$(echo "$output" | jq -r ".[$i].$field")
            is "$actual" "$expect\$" "jq .[$i].$field"
            i=$(expr $i + 1)
        done
    done

}

# vim: filetype=sh
