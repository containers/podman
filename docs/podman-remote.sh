#!/bin/bash -e
# Assemble remote man pages for darwin or windows from markdown files

PLATFORM=$1                         ## windows or darwin
TARGET=$2                           ## where to output files
SOURCES=${@:3}                      ## directories to find markdown files

PODMAN=${PODMAN:-bin/podman-remote} ## location overridden for testing

function usage() {
    echo >&2 "$0 PLATFORM TARGET SOURCES..."
    echo >&2 "PLATFORM: Is either darwin or windows."
    echo >&2 "TARGET: Is the directory where files will be staged."
    echo >&2 "SOURCES: Are the directories to source markdown files."
}

function fail() {
    echo >&2 -e "$@\n"
    usage
    exit 1
}

case $PLATFORM in
'darwin')
    EXT=1
    PUBLISHER=darwin_fn
    ;;
'windows')
    EXT=1.md
    PUBLISHER=windows_fn
    ;;
'-help')
    usage
    exit 0
    ;;
*) fail '"darwin" and "windows" are currently the only supported platforms.' ;;
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

## darwin_fn copies the markdown page or link to flattened directory
function darwin_fn() {
    local markdown=$1
    local file=$(basename $markdown)
    local dir=$(dirname $markdown)
    if [[ -f $dir/../../links/$file ]]; then
        markdown=$dir/../../links/$file
    fi
    install $markdown $TARGET
}

## windows_fn converts the markdown page or link to HTML
function windows_fn() {
    local markdown=$1
    local file=$(basename $markdown)
    local dir=$(dirname $markdown)

    if [[ ! -f $markdown ]]; then
        local link=$(sed -e 's?.so man1/\(.*\)?\1?' <$dir/links/${file%.md})
        markdown=$dir/$link.md
    fi
    pandoc --ascii --lua-filter=$dir/links-to-html.lua -o $TARGET/${file%.$EXT}.html $markdown
}

## pub_pages finds and publishes the remote manual pages
function pub_pages() {
    local source=$1
    local publisher=$2
    for f in $(ls $source/podman-remote*$EXT); do
        $publisher $f
    done

    for c in "container" "image" "pod" "volume" ""; do
        local cmd=${c:+-$c}
        for s in $($PODMAN $c --help | sed -n '/^Available Commands:/,/^Flags:/p' | sed -e '1d;$d' -e '/^$/d' | awk '{print $1}'); do
            $publisher $source/podman$cmd-$s.$EXT
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
