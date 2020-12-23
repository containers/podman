#!/usr/bin/env bash
tmpdir="$PWD/tmp.$RANDOM"
mkdir -p "$tmpdir"
trap 'rm -fr "$tmpdir"' EXIT
${CC:-cc} ${CFLAGS} ${CPPFLAGS} ${LDFLAGS} -o "$tmpdir"/libdm_tag -x c - -ldevmapper > /dev/null 2> /dev/null << EOF
#include <libdevmapper.h>
int main() {
	struct dm_task *task;
	dm_task_deferred_remove(task);
	return 0;
}
EOF
if test $? -ne 0 ; then
	echo libdm_no_deferred_remove
fi
