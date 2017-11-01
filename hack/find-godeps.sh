#!/bin/bash
#
# $1 - base path of the source tree
# $2 - subpath under $1 to find *.go dependencies for
# $3 - package name (eg, github.com/organization/project)

set -o errexit
set -o nounset
set -o pipefail

# might be called from makefile before basepath is set up; just return
# empty deps.  The make target will then ensure that GOPATH is set up
# correctly, and go build will build everything the first time around
# anyway.  Next time we get here everything will be fine.
if [ ! -d "$1/$2" ]; then
	exit 0
fi

function find-deps() {
	local basepath=$1
	local srcdir=$2
	local pkgname=$3
	local deps=

	# gather imports from cri-o
	pkgs=$(cd ${basepath}/${srcdir} && go list -f "{{.Imports}}" . | tr ' ' '\n' | tr -d '[]' | grep -v "/vendor/" | grep ${pkgname} | sed -e "s|${pkgname}/||g")

	# add each Go import's sources to the deps list,
	# and recursively get that imports's imports too
	for dep in ${pkgs}; do
		deps+="$(ls ${basepath}/${dep}/*.go | sed -e "s|${basepath}/||g") "
		# add deps of this package too
		deps+="$(find-deps ${basepath} ${dep} ${pkgname}) "
	done

	echo "${deps}" | sort | uniq
}

# add Go sources from the current package at the end
echo "$(find-deps "$1" "$2" "$3" | xargs) $(cd $1 && ls $2/*.go | xargs)"

