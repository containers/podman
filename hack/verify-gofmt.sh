#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

find_files() {
  find . -not \( \
      \( \
        -wholename '*/vendor/*' \
      \) -prune \
    \) -name '*.go' \
    -not \( -wholename './_output/*' \) \
    -not \( -wholename './cmd/podman/ioprojectatomicpodman/ioprojectatomicpodman.go' \)
}
FIX=0
GOFMT="gofmt -s"
bad_files=$(find_files | xargs $GOFMT -l)

while getopts "f?:" opt; do
    case "$opt" in
        f) FIX=1
            ;;
    esac
done

if [[ -n "${bad_files}" ]]; then
    if (($FIX == 1)) ; then
        echo "Correcting the following files:"
        echo "${bad_files}"
        while read -r go_file; do
            gofmt -s -w $go_file
        done <<< "${bad_files}"
    else
      echo "!!! '$GOFMT' needs to be run on the following files: "
      echo "${bad_files}"
      exit 1
  fi
fi
