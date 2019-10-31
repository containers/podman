#!/bin/bash -e
# Assemble remote man pages for darwin or windows from markdown files

PLATFORM=$1                         ## linux, windows or darwin
TARGET=${2}                         ## where to output files
SOURCES=${@:3}                      ## directories to find markdown files

PODMAN=${PODMAN:-bin/podman-remote} ## location overridden for testing

function usage() {
    echo >&2 "$0 PLATFORM TARGET SOURCES..."
    echo >&2 "PLATFORM: Is either linux, darwin or windows."
    echo >&2 "TARGET: Is the directory where files will be staged. eg, docs/build/remote/linux"
    echo >&2 "SOURCES: Are the directories of source files. eg, docs/markdown"
}

function fail() {
    echo >&2 -e "$@\n"
    usage
    exit 1
}

case $PLATFORM in
darwin|linux)
    PUBLISHER=man_fn
    ;;
windows)
    PUBLISHER=html_fn
    ;;
-help)
    usage
    exit 0
    ;;
*) fail '"linux", "darwin" and "windows" are the only supported platforms.' ;;
esac

if [[ -z $TARGET ]]; then
    fail 'TARGET directory is required'
fi

if [[ -z $SOURCES ]]; then
    fail 'At least one SOURCE directory is required'
fi

if [[ ! -x $PODMAN ]]; then
    fail "$PODMAN does not exist"
fi

## man_fn copies the man page or link to flattened directory
function man_fn() {
    local page=$1
    local file=$(basename $page)
    local dir=$(dirname $page)

    if [[ ! -f $page ]]; then
        page=$dir/links/${file%.*}.1
    fi
    install $page $TARGET/${file%%.*}.1
}

## html_fn converts the markdown page or link to HTML
function html_fn() {
    local markdown=$1
    local file=$(basename $markdown)
    local dir=$(dirname $markdown)

    if [[ ! -f $markdown ]]; then
        local link=$(sed -e 's?.so man1/\(.*\)?\1?' <$dir/links/${file%.md})
        markdown=$dir/$link.md
    fi
    pandoc --ascii --lua-filter=docs/links-to-html.lua -o $TARGET/${file%%.*}.html $markdown
}

## pub_pages finds and publishes the remote manual pages
function pub_pages() {
    local source=$1
    local publisher=$2
    for f in $(ls $source/podman-remote*); do
        $publisher $f
    done

    # rename podman-remote.ext to podman.ext and copy
    local remote=$(echo $TARGET/podman-remote.*)
    local ext=${remote##*.}
    cp -f $remote $TARGET/podman.$ext

    for c in "container" "image" "pod" "volume" ""; do
        local cmd=${c:+-$c}
        for s in $($PODMAN $c --help | sed -n '/^Available Commands:/,/^Flags:/p' | sed -e '1d;$d' -e '/^$/d' | awk '{print $1}'); do
            $publisher $(echo $source/podman$cmd-$s.*)
        done
    done
}

## walk the SOURCES for markdown sources
mkdir -p $TARGET
for s in $SOURCES; do
    if [[ -d $s ]]; then
        pub_pages $s $PUBLISHER
    else
        echo >&2 "Warning: $s does not exist"
    fi
done
