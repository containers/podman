#!/usr/bin/env bash
if test $(${GO:-go} env GOOS) != "linux" ; then
	exit 0
fi
tmpdir="$PWD/tmp.$RANDOM"
mkdir -p "$tmpdir"
trap 'rm -fr "$tmpdir"' EXIT
cc -o "$tmpdir"/libsubid_tag -l subid -x c - > /dev/null 2> /dev/null << EOF
#include <shadow/subid.h>
int main() {
	struct subid_range *ranges = NULL;
	get_subuid_ranges("root", &ranges);
	free(ranges);
	return 0;
}
EOF
if test $? -eq 0 ; then
	echo libsubid
fi
